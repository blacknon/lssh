# output
Output example_lssh_parallel.gif

# Set
Set Shell "zsh"
Set FontSize 12
Set Width 1100
Set Height 700
Set Theme "Catppuccin Frappe"
Set Padding 20
Set Margin 20
Set MarginFill "#11111b"
Set WindowBar Colorful
Set BorderRadius 12
Set Framerate 60
Set PlaybackSpeed 0.9
Set CursorBlink false
Set TypingSpeed 140ms

# parallel command
Type "lssh -p sudo tail -f /var/log/syslog"
Enter
Sleep 500ms

# select
Type "gitla"
Sleep 1000ms
Ctrl+A
Sleep 500ms
Enter
Sleep 10s
Ctrl+C
Sleep 500ms
