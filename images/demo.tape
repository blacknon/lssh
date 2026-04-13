# output
Output demo.gif

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

# lssh start
Enter
Sleep 800ms
Type "lssh"
Enter
Sleep 500ms

# select
Type "gitla"
Sleep 1000ms
Down 1
Sleep 500ms
Up 1
Sleep 500ms
Enter
Sleep 2s

# gitlab
## htop
Enter
Type "htop"
Enter
Sleep 4s
Ctrl+C
Sleep 500ms
Enter

## exit
Type "exit"
Enter
Sleep 500ms
Enter

# parallel command
Type "lssh -p hostname -a"
Enter
Sleep 500ms

# select
Type "gitla"
Sleep 1000ms
Ctrl+A
Sleep 500ms
Enter
Sleep 2s

# parallel Mux
Type "lssh -P"
Enter
Sleep 1000ms

# select
Type "gitla"
Sleep 1000ms
Enter
Sleep 2s
Enter

# gitlab
## htop
Type "htop"
Enter
Sleep 3s

# create new pane
Ctrl+A
Sleep 500ms
Type "-"

# Select
Type "gitla"
Sleep 500ms
Down 1
Sleep 200ms
Enter

#
Type "vim /etc/hosts"
Sleep 500ms
Enter
Sleep 2s

# Close lssh mux mode
Ctrl+A
Sleep 500ms
Type "&"
Sleep 2s
Enter

Type "lssh -P htop"
Enter
Sleep 500ms
Type "gitla"
Sleep 1000ms
Ctrl+A
Sleep 500ms
Enter
Sleep 2s
