# output
Output lsmux_filetransfer.gif

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

# select pane target
Type "gitla"
Sleep 1000ms
Enter
Sleep 2s
Enter
Sleep 500ms

# open transfer wizard
Ctrl+A
Sleep 500ms
Type "f"
Sleep 1500ms

# browse a little
Down 2
Sleep 500ms
Up 1
Sleep 500ms
Ctrl+N
Sleep 700ms
Ctrl+P
Sleep 700ms

# close transfer wizard
Esc
Sleep 500ms

# close lsmux
Ctrl+A
Sleep 500ms
Type "&"
Sleep 1200ms
Enter
