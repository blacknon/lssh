# output
Output lscp.gif

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

# start lsmux
Type "lscp -P 3 ./ls{sh,cp,ftp} r:/tmp/lssh-bin/"
Enter
Sleep 1000ms

# Select host
Type "gitla"
Sleep 1000ms
Ctrl+A
Sleep 200ms
Enter

# Sleep
Sleep 20s
