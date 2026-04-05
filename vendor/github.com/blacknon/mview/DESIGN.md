This document lists architectural details of cview.

# Focus-related style attributes are unset by default

This applies to all widgets except Button and TabbedPanels, which require a
style change to indicate focus. See [ColorUnset](https://docs.rocketnine.space/code.rocketnine.space/tslocum/cview#pkg-variables).

# Widgets always use `sync.RWMutex`

See [#30](https://code.rocketnine.space/tslocum/cview/issues/30).
