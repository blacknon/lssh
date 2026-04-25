# output
Output lsmux_command.gif

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

# start command mode
Type "lsmux --hold hostname"
Enter
Sleep 700ms

# select multiple hosts from filter
Type "gitla"
Sleep 1000ms
Ctrl+A
Sleep 500ms
Enter
Sleep 3s

# leave results on screen briefly
Sleep 1500ms

# close lsmux
Ctrl+A
Sleep 500ms
Type "&"
Sleep 1200ms
Enter
