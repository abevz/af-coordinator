#!/bin/sh
set -eu

repo_root="/home/abevz/github/af-coordinator"
tool_name="${DEEPSEEK_TOOL_NAME:-}"
tool_args="${DEEPSEEK_TOOL_ARGS:-}"

deny() {
	jq -nc --arg reason "$1" '{decision:"deny", reason:$reason}'
	exit 0
}

case "$tool_name" in
	agent | agent_open) ;;
	*) exit 0 ;;
esac

if ! printf '%s' "$tool_args" | jq -e 'type == "object"' >/dev/null 2>&1; then
	deny "Cannot verify sub-agent isolation: CodeWhale did not provide valid tool arguments."
fi

if ! printf '%s' "$tool_args" | jq -e '.worktree == true' >/dev/null; then
	deny "Sub-agent spawn blocked: AGENTS.md requires worktree: true."
fi

if ! branch=$(printf '%s' "$tool_args" | jq -er '.worktree_branch | select(type == "string" and length > 0)'); then
	deny "Sub-agent spawn blocked: set an explicit non-empty worktree_branch."
fi

if ! worktree_path=$(printf '%s' "$tool_args" | jq -er '.worktree_path | select(type == "string" and length > 0)'); then
	deny "Sub-agent spawn blocked: set an explicit absolute worktree_path alongside main/."
fi

if printf '%s' "$tool_args" | jq -e '.cwd? != null' >/dev/null; then
	deny "Sub-agent spawn blocked: do not combine cwd with worktree isolation."
fi

case "$worktree_path" in
	"$repo_root"/*) ;;
	*) deny "Sub-agent spawn blocked: worktree_path must be an absolute sibling path under $repo_root/." ;;
esac

if [ "${worktree_path%/*}" != "$repo_root" ]; then
	deny "Sub-agent spawn blocked: worktree_path must be directly alongside main/, not nested."
fi

case "${worktree_path##*/}" in
	"" | main | .bare | .codewhale-worktrees)
		deny "Sub-agent spawn blocked: worktree_path must name a dedicated sibling worktree."
		;;
esac

if [ "$branch" = "main" ]; then
	deny "Sub-agent spawn blocked: worktree_branch must not be main."
fi

printf '%s\n' '{"decision":"allow"}'
