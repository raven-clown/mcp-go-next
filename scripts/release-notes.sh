#!/usr/bin/env bash
# Writes the release notes for a tag to stdout: the tag's CHANGELOG section under "What's Changed",
# each "(#N, ...)" turned into "by @author in #N, ...", then GitHub's New Contributors and Full
# Changelog. Needs gh with a token that can read the repository.
#
# usage: scripts/release-notes.sh v0.1.0 [owner/repo]
set -euo pipefail

tag="${1:?usage: release-notes.sh <tag> [owner/repo]}"
repo="${2:-${GITHUB_REPOSITORY:-raven-clown/mcp-go-next}}"
# Maintainers are left out of New Contributors.
maintainers="${MAINTAINERS:-raven-clown}"
version="${tag#v}"
changelog="$(dirname "$0")/../CHANGELOG.md"

section="$(awk -v v="$version" '
	index($0, "## [" v "]") == 1 { found = 1; next }
	found && (/^## \[/ || /^\[[^]]+\]: /) { exit }
	found && !started && /^[[:space:]]*$/ { next }
	found { started = 1; print }
' "$changelog")"
if [ -z "$(printf '%s' "$section" | tr -d '[:space:]')" ]; then
	echo "release-notes: CHANGELOG.md has no section for $version" >&2
	exit 1
fi

declare -A authors
author_of() {
	local n="$1"
	if [ -z "${authors[$n]:-}" ]; then
		authors[$n]="$(gh api "repos/$repo/pulls/$n" --jq .user.login)"
	fi
	printf '%s' "${authors[$n]}"
}

echo "## What's Changed"
echo
ref_re='^(.*)\((#[0-9]+(, #[0-9]+)*)\)[[:space:]]*$'
while IFS= read -r line; do
	if [[ "$line" =~ $ref_re ]]; then
		head="${BASH_REMATCH[1]}"
		refs="${BASH_REMATCH[2]}"
		first="${refs%%,*}"
		printf '%sby @%s in %s\n' "$head" "$(author_of "${first#\#}")" "$refs"
	else
		printf '%s\n' "$line"
	fi
done <<<"$section"

previous="$(git tag --list 'v*' --sort=-v:refname | grep -vx "$tag" | head -n 1 || true)"
args=(-f "tag_name=$tag")
if [ -n "$previous" ]; then
	args+=(-f "previous_tag_name=$previous")
fi
generated="$(gh api "repos/$repo/releases/generate-notes" "${args[@]}" --jq .body)"

newcomers="$(printf '%s\n' "$generated" | awk -v skip="$maintainers" '
	BEGIN { n = split(skip, list, ","); for (i = 1; i <= n; i++) drop["@" list[i]] = 1 }
	/^## New Contributors/ { f = 1; next }
	f && /^\*\*Full Changelog/ { exit }
	f && /^\* @/ { split($2, who, " "); if (!(who[1] in drop)) print }
')"
if [ -n "$newcomers" ]; then
	echo
	echo "## New Contributors"
	echo
	printf '%s\n' "$newcomers"
fi
full="$(printf '%s\n' "$generated" | grep '^\*\*Full Changelog' || true)"
if [ -n "$full" ]; then
	echo
	printf '%s\n' "$full"
fi
