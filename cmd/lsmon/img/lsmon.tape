# output
Output lsmon.gif

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

# start lsmon and show selector
Type "lsmon"
Enter
Sleep 500ms

# filter and move through host list
Type "vm-"
Sleep 1000ms
Down 1
Sleep 500ms
Up 1
Sleep 1000ms
Ctrl+A
Sleep 200ms
Enter

# show list
Sleep 20s
Down 1
Sleep 1000ms
Down 1
Sleep 1000ms

# show top
Ctrl + X
Sleep 3000ms
Down 1
Sleep 3000ms
Up 1
Sleep 3000ms

# show terminal
Ctrl + T
Sleep 2000ms
Enter
Sleep 500ms
Type "htop"
Sleep 2000ms
Enter
Sleep 20s
