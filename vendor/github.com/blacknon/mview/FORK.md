This document explains why [tview](https://github.com/rivo/tview) was forked to
create [cview](https://code.rocketnine.space/tslocum/cview). It also explains any
differences between the projects and tracks which tview pull requests have been
merged into cview.

# Why fork?

[rivo](https://github.com/rivo), the creator and sole maintainer of tview,
explains his reviewing and merging process in a [GitHub comment](https://github.com/rivo/tview/pull/298#issuecomment-559373851).

He states that he does not have the necessary time or interest to review,
discuss and merge pull requests:

>this project is quite low in priority. It doesn't generate any income for me
>and, unfortunately, reviewing issues and PRs is also not much "fun".

>But some other people submitted large PRs which will cost me many hours to
>review. (I had to chuckle a bit when I saw [this comment](https://github.com/rivo/tview/pull/363#issuecomment-555484734).)

>Lastly, I'm the one who ends up maintaining this code. I have to be 100%
>behind it, understand it 100%, and be able to make changes to it later if
> necessary.

cview aims to solve these issues by increasing the number of project
maintainers and allowing code changes which may be outside of tview's scope.

# Differences

## Primitive methods do not return the primitive they belong to

When chaining multiple primitive method calls together, application developers
might accidentally end the chain with a different return type than the first
method call. This could result in unexpected return types. For example, ending
a chain with `SetTitle` would result in a `Box` rather than the original primitive.

## cview is [thread-safe](https://docs.rocketnine.space/code.rocketnine.space/tslocum/cview/#hdr-Concurrency)

tview [is not thread-safe](https://godoc.org/github.com/rivo/tview#hdr-Concurrency).

## [Application.QueueUpdate](https://docs.rocketnine.space/code.rocketnine.space/tslocum/cview/#Application.QueueUpdate) and [Application.QueueUpdateDraw](https://docs.rocketnine.space/code.rocketnine.space/tslocum/cview/#Application.QueueUpdateDraw) do not block

tview [blocks until the queued function returns](https://github.com/rivo/tview/blob/fe3052019536251fd145835dbaa225b33b7d3088/application.go#L510).

## `Primitive` has two additional fields, `SetVisible` and `GetVisible`

Widgets embedding `Box` require the addition of the following at the beginning
of their `Draw` routine to handle visibility changes.

```go
func (w *Widget) Draw(screen tcell.Screen) {
	if !w.GetVisible() {
		return
	}

	// ...
}
```

## Setting a primitive's background color to `tcell.ColorDefault` does not result in transparency

Call [Box.SetBackgroundTransparent](https://docs.rocketnine.space/code.rocketnine.space/tslocum/cview/#Box.SetBackgroundTransparent)
to enable background transparency.

## Double clicks are not handled by default

All clicks are handled as single clicks until an interval is set with [Application.SetDoubleClickInterval](https://docs.rocketnine.space/code.rocketnine.space/tslocum/cview/#Application.SetDoubleClickInterval).

## Tables are sorted when a fixed row is clicked by default

Call [Table.SetSortClicked](https://docs.rocketnine.space/code.rocketnine.space/tslocum/cview/#Table.SetSortClicked)
to disable this behavior.

## Lists and Forms do not wrap around by default

Call `SetWrapAround` to wrap around when navigating.

## Tables do not wrap around when selecting a cell

Attempting to move outside of the range of the table results in the selection
remaining unchanged. 

## TextViews store their text as []byte instead of string

This greatly improves buffer efficiency. [TextView.Write](https://docs.rocketnine.space/code.rocketnine.space/tslocum/cview/#TextView.Write)
is 90% faster and [TextView.Draw](https://docs.rocketnine.space/code.rocketnine.space/tslocum/cview/#TextView.Draw)
is 50% faster.

## `Pages` has been renamed as `Panels`

This is for consistency with `TabbedPanels`, which is built on top of `Panels`.

## `Panels.AddPanel` preserves order of panels when updating existing panel

tview moves the updated panel to the end.

## `Application.Draw` may be used to draw the entire screen or a set of primitives

When calling `Application.Draw` without providing any primitives, the entire
screen is drawn. This behavior matches tview.

When using cview, you may provide one or more primitives to limit drawing.

## `Application.ForceDraw` has been removed

Because cview is [thread-safe](https://docs.rocketnine.space/code.rocketnine.space/tslocum/cview/#hdr-Concurrency),
application draws must be queued. Call `Application.QueueUpdateDraw` to queue
an update that requires the application to be drawn when completed.

# Merged pull requests

The following tview pull requests have been merged into cview:

- [#378 Throttle resize handling](https://github.com/rivo/tview/pull/378)
- [#368 Add support for displaying text next to a checkbox](https://github.com/rivo/tview/pull/368)
- [#353 Add window size change handler](https://github.com/rivo/tview/pull/353)
- [#347 Handle ANSI code 39 and 49](https://github.com/rivo/tview/pull/347)
- [#336 Don't skip regions at end of line](https://github.com/rivo/tview/pull/336)
- [#296 Fixed TextView's reset &#x5B;-&#x5D; setting the wrong color](https://github.com/rivo/tview/pull/296)
