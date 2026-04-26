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

# start
Type "lsshell"
Enter
Sleep 1000ms

# select host
Type "gitl"
Sleep 1000ms
Ctrl+A
Sleep 500ms
Enter
Sleep 2000ms

# command
Enter
Sleep 500ms
Enter
Sleep 500ms
Enter
Sleep 500ms
Type "hostname -a"
Sleep 500ms
Enter
Sleep 2000ms

# @host:command
Enter
Sleep 500ms
Enter
Sleep 500ms
Enter
Sleep 500ms
Type "@pve:sv-pve01:vm-gitlab:hostname"
Sleep 500ms
Enter
Sleep 2000ms

# remote only
Enter
Sleep 500ms
Enter
Sleep 500ms
Enter
Sleep 500ms
Type "echo $(hostname)_$(uname -s)"
Sleep 500ms
Enter
Sleep 2000ms

# repeat command
Enter
Sleep 500ms
Enter
Sleep 500ms
Enter
Sleep 500ms
Type "%out 2"
Sleep 500ms
Enter
Sleep 2000ms

# remote and local: +command
Type "%out 2 | +sed 's/$/_'$(uname -s)'/'"
Sleep 500ms
Enter
Sleep 2000ms

Enter
Sleep 500ms
Enter
Sleep 500ms
Enter
Sleep 500ms
Type "%out 2 | +sed 's/$/_'$(uname -s)'/' | sed 's/$/_'$(uname -s)'/'"
Sleep 500ms
Enter
Sleep 2000ms

# remote and local: ++command
Enter
Sleep 500ms
Enter
Sleep 500ms
Enter
Sleep 500ms
Type "%out 2 | ++sed 's/$/_'$(uname -s)'/'"
Sleep 500ms
Enter
Sleep 2000ms

Enter
Sleep 500ms
Enter
Sleep 500ms
Enter
Sleep 500ms
Type "%out 2 | ++sed 's/$/_'$(uname -s)'/' | sed 's/$/_'$(uname -s)'/'"
Sleep 500ms
Enter
Sleep 2000ms

# vimdiff
Enter
Sleep 500ms
Enter
Sleep 500ms
Enter
Sleep 500ms
Type "+vimdiff +<(cat /etc/passwd)"
Sleep 500ms
Enter
Sleep 1000ms
Down 1
Sleep 500ms
Down 1
Sleep 500ms
Down 1
Sleep 500ms
Sleep 2000ms
