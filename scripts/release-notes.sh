#!/usr/bin/env bash
# Writes the release notes for a tag to stdout: the tag's CHANGELOG section under "What's Changed",
# each trailing "(#N, ...)" turned into "by @author in #N, ..." with every author of those pull
# requests, then GitHub's New Contributors and Full Changelog. A reference that is not a pull request
# stops the script, so a release never ships with broken notes. Needs gh with a token that can read
# the repository.
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

# lookup fills authors[N] in this shell, so failures stop the script and results are cached.
declare -A authors
lookup() {
	local n="$1" login
	if [ -n "${authors[$n]:-}" ]; then
		return 0
	fi
	if ! login="$(gh api "repos/$repo/pulls/$n" --jq .user.login 2>/dev/null)" || [ -z "$login" ]; then
		echo "release-notes: #$n in CHANGELOG.md is not a pull request in $repo" >&2
		exit 1
	fi
	authors[$n]="$login"
}

out="## What's Changed"$'\n\n'
ref_re='^(.*)\((#[0-9]+(, #[0-9]+)*)\)[[:space:]]*$'
while IFS= read -r line; do
	if [[ "$line" =~ $ref_re ]]; then
		head="${BASH_REMATCH[1]}"
		refs="${BASH_REMATCH[2]}"
		credited=""
		for ref in ${refs//,/ }; do
			n="${ref#\#}"
			lookup "$n"
			case " $credited " in
			*" @${authors[$n]} "*) ;;
			*) credited="$credited @${authors[$n]}" ;;
			esac
		done
		by="$(printf '%s' "${credited# }" | sed 's/ / and /g')"
		out+="$(printf '%sby %s in %s' "$head" "$by" "$refs")"$'\n'
	else
		out+="$line"$'\n'
	fi
done <<<"$section"
printf '%s' "$out"

# The previous release is the highest tag below this one, not the newest tag overall.
previous="$( { git tag --list 'v*'; printf '%s\n' "$tag"; } | sort -uV | awk -v t="$tag" '$0 == t { print prev; exit } { prev = $0 }')"
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
