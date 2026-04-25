# output
Output lsmux_term.gif

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
Type "lsmux"
Enter
Sleep 1000ms

# select first pane
Type "gitla"
Sleep 1000ms
Enter
Sleep 2s
Enter
Sleep 500ms

# run interactive command
Type "htop"
Enter
Sleep 3s
Ctrl+C
Sleep 500ms

# create a new pane
Ctrl+A
Sleep 500ms
Type "s"
Sleep 500ms

# select second pane target
Type "gitla"
Sleep 700ms
Down 1
Sleep 300ms
Enter
Sleep 1500ms
Enter
Sleep 500ms

# run a simple command
Type "hostname"
Enter
Sleep 2s

# close lsmux
Ctrl+A
Sleep 500ms
Type "&"
Sleep 1200ms
Enter
