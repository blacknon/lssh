Go TextCol
==========

A Go package that takes a flat array of strings and prints it in a columnar format.

We all love the way the `ls` command formats its output.
It prints columns that are dynamically sized based on the size of the contents and the terminal.
This package lets you pass a simple array of strings, and will figure out the right sizes to print it in a columnar layout, just like `ls -C`.
No need to specify any lengths or the number of items per row or mess with seperators.
It will even work if there are ANSI escape codes in the text.

## Install

```sh
$ go get github.com/acarl005/textcol
```

## Usage

```
func PrintColumns(stringArray *[]string, margin int)
```

```go
// thing.go

import "github.com/acarl005/textcol"

func main() {
	items := []string{
		"📂 folder thing",
		"won't get tripped up by emojis",
		"or even \x1b[38;5;140mcolor codes\x1b[0m.",
		"here",
		"are",
		"some",
		"shorter",
		"lines",
		"running out of stuff",
		"foo bar",
	}
	// pass pointer to array of strings and a margin value. this will ensure at least 4 spaces appear to the right of each cell
	textcol.PrintColumns(&items, 4)
}
```

And this is what the output will be:

![demo-output](./img/demo.png)

