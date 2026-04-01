/*
Package mview implements rich widgets for terminal based user interfaces.

See the demos folder and the example application provided with the
NewApplication documentation for usage examples.

Types

This package is built on top of tcell, which provides the types necessary to
create a terminal-based application (e.g. EventKey). For information on
inherited types see the tcell documentation.

tcell: https://github.com/gdamore/tcell

Base Primitive

Widgets must implement the Primitive interface. All widgets embed the base
primitive, Box, and thus inherit its functions. This isn't necessarily
required, but it makes more sense than reimplementing Box's functionality in
each widget.

Widgets

The following widgets are available:

  Button - Button which is activated when the user selects it.
  CheckBox - Selectable checkbox for boolean values.
  DropDown - Drop-down selection field.
  Flex - A Flexbox based layout manager.
  Form - Form composed of input fields, drop down selections, checkboxes, and
    buttons.
  Grid - A grid based layout manager.
  InputField - Single-line text entry field.
  List - A navigable text list with optional keyboard shortcuts.
  Modal - A centered window with a text message and one or more buttons.
  Panels - A panel based layout manager.
  ProgressBar - Indicates the progress of an operation.
  TabbedPanels - Panels widget with tabbed navigation.
  Table - A scrollable display of tabular data. Table cells, rows, or columns
    may also be highlighted.
  TextView - A scrollable window that displays multi-colored text. Text may
    also be highlighted.
  TreeView - A scrollable display for hierarchical data. Tree nodes can be
    highlighted, collapsed, expanded, and more.
  Window - A draggable and resizable container.

Widgets may be used without an application created via NewApplication, allowing
them to be integrated into any tcell-based application.

Concurrency

All functions may be called concurrently (they are thread-safe). When called
from multiple threads, functions will block until the application or widget
becomes available. Function calls may be queued with Application.QueueUpdate to
avoid blocking.

Unicode Support

This package supports unicode characters including wide characters.

Keyboard Shortcuts

Widgets use keyboard shortcuts (a.k.a. keybindings) such as arrow keys and
H/J/K/L by default. You may replace these defaults by modifying the shortcuts
listed in Keys. You may also override keyboard shortcuts globally by setting a
handler with Application.SetInputCapture.

cbind is a library which simplifies the process of adding support for custom
keyboard shortcuts to your application. It allows setting handlers for
EventKeys. It also translates between EventKeys and human-readable strings such
as "Alt+Enter". This makes it possible to store keybindings in a configuration
file.

cbind: https://code.rocketnine.space/tslocum/cbind

Bracketed Paste Mode

Bracketed paste mode is enabled by default. It may be disabled by calling
Application.EnableBracketedPaste before Application.Run. The following demo
shows how to handle paste events and process pasted text.

tcell bracketed paste demo: https://github.com/gdamore/tcell/blob/master/_demos/mouse.go

Mouse Support

Mouse support may be enabled by calling Application.EnableMouse before
Application.Run. See the example application provided with the
Application.EnableMouse documentation.

Double clicks are treated single clicks by default. Specify a maximum duration
between clicks with Application.SetDoubleClickInterval to enable double clicks.
A standard duration is provided as StandardDoubleClick.

Mouse events are passed to:

- The handler set with SetMouseCapture, which is reserved for use by application
developers to permanently intercept mouse events. Return nil to stop
propagation.

- The MouseHandler method of the topmost widget under the mouse.

Colors

Throughout this package, colors are specified using the tcell.Color type.
Functions such as tcell.GetColor(), tcell.NewHexColor(), and tcell.NewRGBColor()
can be used to create colors from W3C color names or RGB values.

Almost all strings which are displayed can contain color tags. Color tags are
W3C color names or six hexadecimal digits following a hash tag, wrapped in
square brackets. Examples:

  This is a [red]warning[white]!
  The sky is [#8080ff]blue[#ffffff].

A color tag changes the color of the characters following that color tag. This
applies to almost everything from box titles, list text, form item labels, to
table cells. In a TextView, this functionality must be explicitly enabled. See
the TextView documentation for more information.

Color tags may contain not just the foreground (text) color but also the
background color and additional flags. In fact, the full definition of a color
tag is as follows:

  [<foreground>:<background>:<flags>]

Each of the three fields can be left blank and trailing fields can be omitted.
(Empty square brackets "[]", however, are not considered color tags.) Colors
that are not specified will be left unchanged. A field with just a dash ("-")
means "reset to default".

You can specify the following flags (some flags may not be supported by your
terminal):

  l: blink
  b: bold
  d: dim
  i: italic
  r: reverse (switch foreground and background color)
  u: underline
  s: strikethrough

Examples:

  [yellow]Yellow text
  [yellow:red]Yellow text on red background
  [:red]Red background, text color unchanged
  [yellow::u]Yellow text underlined
  [::bl]Bold, blinking text
  [::-]Colors unchanged, flags reset
  [-]Reset foreground color
  [-:-:-]Reset everything
  [:]No effect
  []Not a valid color tag, will print square brackets as they are

In the rare event that you want to display a string such as "[red]" or
"[#00ff1a]" without applying its effect, you need to put an opening square
bracket before the closing square bracket. Note that the text inside the
brackets will be matched less strictly than region or colors tags. I.e. any
character that may be used in color or region tags will be recognized. Examples:

  [red[]      will be output as [red]
  ["123"[]    will be output as ["123"]
  [#6aff00[[] will be output as [#6aff00[]
  [a#"[[[]    will be output as [a#"[[]
  []          will be output as [] (see color tags above)
  [[]         will be output as [[] (not an escaped tag)

You can use the Escape() function to insert brackets automatically where needed.

Setting the background color of a primitive to tcell.ColorDefault will use the
default terminal background color. To enable transparency (allowing one or more
primitives to display behind a primitive) call SetBackgroundTransparent. The
screen is not cleared before drawing the application. Overlaying transparent
widgets directly onto the screen may result in artifacts. To resolve this, add
a blank, non-transparent Box to the bottom layer of the interface via Panels,
or set a handler via SetBeforeDrawFunc which clears the screen.

Styles

When primitives are instantiated, they are initialized with colors taken from
the global Styles variable. You may change this variable to adapt the look and
feel of the primitives to your preferred style.

Scroll Bars

Scroll bars are supported by the following widgets: List, Table, TextView and
TreeView. Each widget will display scroll bars automatically when there are
additional items offscreen. See SetScrollBarColor and SetScrollBarVisibility.

Hello World

The following is an example application which shows a box titled "Greetings"
containing the text "Hello, world!":

  package main

  import (
    "github.com/blacknon/mview"
  )

  func main() {
    tv := mview.NewTextView()
    tv.SetText("Hello, world!").
       SetBorder(true).
       SetTitle("Greetings")
    if err := mview.NewApplication().SetRoot(tv, true).Run(); err != nil {
      panic(err)
    }
  }

First, we create a TextView with a border and a title. Then we create an
application, set the TextView as its root primitive, and run the event loop.
The application exits when the application's Stop() function is called or when
Ctrl-C is pressed.

If we have a primitive which consumes key presses, we call the application's
SetFocus() function to redirect all key presses to that primitive. Most
primitives then offer ways to install handlers that allow you to react to any
actions performed on them.

Demos

The "demos" subdirectory contains a demo for each widget, as well as a
presentation which gives an overview of the widgets and how they may be used.
*/
package mview
