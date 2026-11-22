# Security policy

## Supported versions

Fixes go into the latest minor release. Please check that a problem still happens on the newest version before reporting it.

## Reporting a vulnerability

Do not open a public issue. Report it privately through GitHub: open the [Security tab](https://github.com/raven-clown/mcp-go-next/security), choose "Report a vulnerability", and include:

- what an attacker can do, and what they need first
- steps or a small proof of concept
- the mcp-go-next version, your Go version and your OS

Never include real tokens or credentials. Redact them from any logs you attach.

You should get a first answer within 7 days. Once a fix is ready we publish an advisory and a release, and credit you unless you ask us not to.

## What counts

mcp-go-next handles requests from clients and calls on behalf of users, so these matter most:

- a request reaching a tool, handle or toolset that belongs to another principal
- a request bypassing the Origin check, CORS rules or header validation
- a crafted message, schema or header causing a crash, unbounded memory or unbounded CPU
- credentials leaking into errors, logs or responses
