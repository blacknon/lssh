# output
Output lspipe.gif

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

# create session
Type "lspipe --name lssh_demo"
Enter
Sleep 1000ms

# select hosts
Type "gitla"
Sleep 800ms
Ctrl+A
Sleep 300ms
Enter
Sleep 2500ms

# inspect session indexes
Type "lspipe --info --name lssh_demo"
Enter
Sleep 3000ms

# use indexed host selection
Type "lspipe --name lssh_demo -H 2 --raw cat /etc/hostname"
Enter
Sleep 2500ms

# pipe stdin through the selected host
Type "echo 'hello from lspipe' | lspipe --name lssh_demo -H 2 'cat'"
Enter
Sleep 3000ms

# vimdiff
Type "eval vimdiff '<(lspipe -H '{1..3}' --name test "
Type '"'
Type "cat /etc/passwd"
Type '"'
Type ")'"
Sleep 200ms
Enter
Sleep 10s
