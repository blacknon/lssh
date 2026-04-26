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
Type "lsftp"
Enter
Sleep 1000ms

# Select host
Type "gitla"
Sleep 1000ms
Ctrl+A
Sleep 200ms
Enter

# put lssh lscp lsftp
Type "put -P 2 ./lssh ./lscp ./lsftp /tmp/"
Sleep 200ms
Enter
Sleep 30s

# put lssh lscp lsftp
Type "ls -la /tmp/lssh /tmp/lscp /tmp/lsftp"
Sleep 200ms
Enter
Sleep 8s
