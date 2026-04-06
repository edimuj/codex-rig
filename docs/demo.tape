Output docs/demo.gif

Set Shell "bash"
Set Theme "Dracula"
Set FontSize 24
Set Width 1200
Set Height 700
Set TypingSpeed 25ms

Hide
Type "cd /home/edimuj/projects/oss/codex-rig" Enter
Type "go build -o ./codex-rig ./cmd/codex-rig" Enter
Type "tmp=$(mktemp -d)" Enter
Type "export CODEX_RIG_ROOT=$tmp/rig-root CODEX_HOME=$tmp/global-codex" Enter
Type "mkdir -p $CODEX_HOME/skills $CODEX_HOME/plugins" Enter
Type "clear" Enter
Show

Type "./codex-rig create build" Enter
Sleep 600ms
Type "./codex-rig use --no-marker build" Enter
Sleep 500ms
Type "./codex-rig inherit skills plugins" Enter
Sleep 500ms
Type "./codex-rig auth status" Enter
Sleep 1200ms
