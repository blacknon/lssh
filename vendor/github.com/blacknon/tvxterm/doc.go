// Package tvxterm provides a small terminal widget for tview.
//
// The package is built around three pieces:
//   - View: a tview primitive that renders terminal state
//   - Backend: a transport abstraction for PTY, SSH, or custom streams
//   - Emulator: a small VT-like state machine that turns bytes into cells
//
// View and Backend form the primary public API for embedding terminal sessions
// into applications. Emulator and Snapshot are also exported so callers can
// test, inspect, or reuse the byte-to-cell pipeline directly when needed.
package tvxterm
