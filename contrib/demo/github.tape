# GitHub demo: import a real issue, run a (scripted) agent, publish the result.
# Generated from this template by contrib/demo/record-github.sh, which creates
# the issue and fills in @ISSUE_URL@, @ISSUE_NUMBER@, and the Env lines.

Output .demo-frames/

Require gh
Require git

Set Shell bash
Set FontSize 16
Set Width 1240
Set Height 760
Set Padding 12
Set TypingSpeed 35ms

Hide
Type "source contrib/demo/github-setup.sh" Enter
Wait+Screen@120s /(?m)^\$ *$/
Show

Sleep 800ms
Type "dibs issue import @ISSUE_URL@"
Sleep 400ms
Enter
Wait+Screen@60s /Imported/
Sleep 1.5s

Type "dibs issue run sandbox-1 --publish --require-complete -- demo-agent"
Sleep 400ms
Enter
Wait+Screen@180s /Published sandbox-1/
Sleep 2s

Type "clear; gh issue view @ISSUE_NUMBER@ -R abevz/dibs-sandbox --comments"
Sleep 400ms
Enter
Sleep 6s

Hide
Type "demo_cleanup" Enter
Wait+Screen@60s /dibs github demo cleaned up/
