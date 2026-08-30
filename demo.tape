# VHS tape script for recording tunnelsnoop terminal demo
# Run: vhs demo.tape

Output assets/demo.gif

Set Shell "bash"
Set FontSize 14
Set Width 960
Set Height 540
Set Padding 20
Set Theme "Catppuccin Mocha"
Set WindowBar "Colorful"

Type "tunnelsnoop -once"
Sleep 500ms
Enter
Sleep 2.5s

Type "tunnelsnoop -interval 2s"
Sleep 500ms
Enter
Sleep 4s
Ctrl+C
Sleep 1s

Type "tunnelsnoop -kill-idle 15m"
Sleep 500ms
Enter
Sleep 3s
Ctrl+C
Sleep 1s
