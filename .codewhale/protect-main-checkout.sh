#!/bin/sh
set -eu

main_workspace="/home/abevz/github/af-coordinator/main"
tool_name="${DEEPSEEK_TOOL_NAME:-}"
workspace="${DEEPSEEK_WORKSPACE:-}"

if [ "$workspace" != "$main_workspace" ]; then
	exit 0
fi

case "$tool_name" in
	write_file | edit_file | apply_patch | patch_file)
		reason="Direct file writes in main/ are blocked. Spawn an implementation agent with worktree: true, worktree_branch, and an absolute sibling worktree_path."
		;;
	exec_shell | task_shell_start)
		reason="Shell execution in main/ is blocked because arbitrary shell commands can mutate the checkout. Run the command from an isolated sibling worktree."
		;;
	*)
		exit 0
		;;
esac

jq -nc --arg reason "$reason" '{decision:"deny", reason:$reason}'
