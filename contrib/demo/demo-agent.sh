#!/usr/bin/env bash
# Scripted stand-in for an AI agent in the GitHub demo: it makes one change,
# opens a real pull request, and reports it to dibs with hooks complete.
set -euo pipefail
repo=${DEMO_REPO:-abevz/dibs-sandbox}
branch="demo/changelog-$(date +%s)"
say() { printf '\e[2magent:\e[0m %s\n' "$*"; }
say "editing CHANGELOG.md on $branch"
git switch -q -c "$branch"
printf -- '- %s: demo entry added by a dibs issue run\n' "$(date -u +%F)" >>CHANGELOG.md
git add CHANGELOG.md
git -c user.name="dibs demo" -c user.email=demo@example.invalid commit -q -m "Add changelog entry"
git -c credential.helper= -c credential.helper='!gh auth git-credential' \
	push -q "https://github.com/$repo.git" "$branch" 2>/dev/null
pr=$(gh pr create -R "$repo" --base main --head "$branch" \
	--title "Add changelog entry" --body "Opened by the dibs GitHub demo.")
say "opened $pr"
dibs hooks complete --pr-url "$pr" --commit-sha "$(git rev-parse --short HEAD)" \
	--branch "$branch" --note "Added a changelog entry and opened a PR."
