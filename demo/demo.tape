# ricorda demo — regenerate with:  cd demo && vhs demo.tape
# Requires vhs, ttyd and ffmpeg (https://github.com/charmbracelet/vhs).
# The recording runs against the fake histories in fixtures/, never your own.

Output demo.gif

Set Shell bash
Set FontSize 15
Set Width 1100
Set Height 640
Set Padding 24
Set BorderRadius 8
Set Theme "Catppuccin Mocha"
Set TypingSpeed 50ms

Hide
Type "source ./setup-demo-env.sh && clear"
Enter
Sleep 8s
Show

Type "# your shell history already knows what you need."
Sleep 500ms
Enter
Type "# ricorda writes it down."
Sleep 500ms
Enter
Sleep 1s
Type "ricorda scan"
Sleep 500ms
Enter
Sleep 4s
Type "ricorda ffmpeg"
Sleep 500ms
Enter
Sleep 4.5s
Type "ricorda curl   # secrets are redacted before they ever touch disk"
Sleep 500ms
Enter
Sleep 4.5s
Type "# 100% local. no cloud. your history stays yours."
Sleep 3.5s
