# output
Output command.gif

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

# check
Type "ls -la ~/Today/hoge"
Enter
Sleep 1000ms
Enter
Sleep 500ms

# start
Type "lssync -B -P 2 ~/Today/hoge r:/tmp/lssh-bin"
Sleep 500ms
Enter
Sleep 1500ms
Type "gitl"
Sleep 500ms
Ctrl+A
Sleep 500ms
Enter
Sleep 40s
