package mview

import (
	"fmt"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
)

const (
	// The size of the event/update/redraw channels.
	queueSize = 100

	// The minimum duration between resize event callbacks.
	resizeEventThrottle = 50 * time.Millisecond
)

// Application represents the top node of an application.
//
// It is not strictly required to use this class as none of the other classes
// depend on it. However, it provides useful tools to set up an application and
// plays nicely with all widgets.
//
// The following command displays a primitive p on the screen until Ctrl-C is
// pressed:
//
//	if err := mview.NewApplication().SetRoot(p, true).Run(); err != nil {
//	    panic(err)
//	}
type Application struct {
	// The application's screen. Apart from Run(), this variable should never be
	// set directly. Always use the screenReplacement channel after calling
	// Fini(), to set a new screen (or nil to stop the application).
	screen tcell.Screen

	// The size of the application's screen.
	width, height int

	// The primitive which currently has the keyboard focus.
	focus Primitive

	// The root primitive to be seen on the screen.
	root Primitive

	// Whether or not the application resizes the root primitive.
	rootFullscreen bool

	// Whether or not to enable bracketed paste mode.
	enableBracketedPaste bool

	// Whether or not to enable mouse events.
	enableMouse bool

	// An optional capture function which receives a key event and returns the
	// event to be forwarded to the default input handler (nil if nothing should
	// be forwarded).
	inputCapture func(event *tcell.EventKey) *tcell.EventKey

	// Time a resize event was last processed.
	lastResize time.Time

	// Timer limiting how quickly resize events are processed.
	throttleResize *time.Timer

	// An optional callback function which is invoked when the application's
	// window is initialized, and when the application's window size changes.
	// After invoking this callback the screen is cleared and the application
	// is drawn.
	afterResize func(width int, height int)

	// An optional callback function which is invoked before the application's
	// focus changes.
	beforeFocus func(p Primitive) bool

	// An optional callback function which is invoked after the application's
	// focus changes.
	afterFocus func(p Primitive)

	// An optional callback function which is invoked just before the root
	// primitive is drawn.
	beforeDraw func(screen tcell.Screen) bool

	// An optional callback function which is invoked after the root primitive
	// was drawn.
	afterDraw func(screen tcell.Screen)

	// Used to send screen events from separate goroutine to main event loop
	events chan tcell.Event

	// Functions queued from goroutines, used to serialize updates to primitives.
	updates chan func()

	// An object that the screen variable will be set to after Fini() was called.
	// Use this channel to set a new screen object for the application
	// (screen.Init() and draw() will be called implicitly). A value of nil will
	// stop the application.
	screenReplacement chan tcell.Screen

	// An optional capture function which receives a mouse event and returns the
	// event to be forwarded to the default mouse handler (nil if nothing should
	// be forwarded).
	mouseCapture func(event *tcell.EventMouse, action MouseAction) (*tcell.EventMouse, MouseAction)

	// doubleClickInterval specifies the maximum time between clicks to register a
	// double click rather than a single click.
	doubleClickInterval time.Duration

	mouseCapturingPrimitive Primitive        // A Primitive returned by a MouseHandler which will capture future mouse events.
	lastMouseX, lastMouseY  int              // The last position of the mouse.
	mouseDownX, mouseDownY  int              // The position of the mouse when its button was last pressed.
	lastMouseClick          time.Time        // The time when a mouse button was last clicked.
	lastMouseButtons        tcell.ButtonMask // The last mouse button state.

	sync.RWMutex
}

// NewApplication creates and returns a new application.
func NewApplication() *Application {
	return &Application{
		enableBracketedPaste: true,
		events:               make(chan tcell.Event, queueSize),
		updates:              make(chan func(), queueSize),
		screenReplacement:    make(chan tcell.Screen, 1),
	}
}

// HandlePanic (when deferred at the start of a goroutine) handles panics
// gracefully. The terminal is returned to its original state before the panic
// message is printed.
//
// Panics may only be handled by the panicking goroutine. Because of this,
// HandlePanic must be deferred at the start of each goroutine (including main).
func (a *Application) HandlePanic() {
	p := recover()
	if p == nil {
		return
	}

	a.finalizeScreen()

	panic(p)
}

// SetInputCapture sets a function which captures all key events before they are
// forwarded to the key event handler of the primitive which currently has
// focus. This function can then choose to forward that key event (or a
// different one) by returning it or stop the key event processing by returning
// nil.
//
// Note that this also affects the default event handling of the application
// itself: Such a handler can intercept the Ctrl-C event which closes the
// application.
func (a *Application) SetInputCapture(capture func(event *tcell.EventKey) *tcell.EventKey) {
	a.Lock()
	defer a.Unlock()

	a.inputCapture = capture

}

// GetInputCapture returns the function installed with SetInputCapture() or nil
// if no such function has been installed.
func (a *Application) GetInputCapture() func(event *tcell.EventKey) *tcell.EventKey {
	a.RLock()
	defer a.RUnlock()

	return a.inputCapture
}

// SetMouseCapture sets a function which captures mouse events (consisting of
// the original tcell mouse event and the semantic mouse action) before they are
// forwarded to the appropriate mouse event handler. This function can then
// choose to forward that event (or a different one) by returning it or stop
// the event processing by returning a nil mouse event.
func (a *Application) SetMouseCapture(capture func(event *tcell.EventMouse, action MouseAction) (*tcell.EventMouse, MouseAction)) {
	a.mouseCapture = capture

}

// GetMouseCapture returns the function installed with SetMouseCapture() or nil
// if no such function has been installed.
func (a *Application) GetMouseCapture() func(event *tcell.EventMouse, action MouseAction) (*tcell.EventMouse, MouseAction) {
	return a.mouseCapture
}

// SetDoubleClickInterval sets the maximum time between clicks to register a
// double click rather than a single click. A standard duration is provided as
// StandardDoubleClick. No interval is set by default, disabling double clicks.
func (a *Application) SetDoubleClickInterval(interval time.Duration) {
	a.doubleClickInterval = interval
}

// SetScreen allows you to provide your own tcell.Screen object. For most
// applications, this is not needed and you should be familiar with
// tcell.Screen when using this function.
//
// This function is typically called before the first call to Run(). Init() need
// not be called on the screen.
func (a *Application) SetScreen(screen tcell.Screen) {
	if screen == nil {
		return // Invalid input. Do nothing.
	}

	a.Lock()
	if a.screen == nil {
		// Run() has not been called yet.
		a.screen = screen
		a.Unlock()
		return
	}

	// Run() is already in progress. Exchange screen.
	oldScreen := a.screen
	a.Unlock()
	oldScreen.Fini()
	a.screenReplacement <- screen
}

// GetScreen returns the current tcell.Screen of the application. Lock the
// application when manipulating the screen to prevent race conditions. This
// value is only available after calling Init or Run.
func (a *Application) GetScreen() tcell.Screen {
	a.RLock()
	defer a.RUnlock()
	return a.screen
}

// GetScreenSize returns the size of the application's screen. These values are
// only available after calling Init or Run.
func (a *Application) GetScreenSize() (width, height int) {
	a.RLock()
	defer a.RUnlock()
	return a.width, a.height
}

// Init initializes the application screen. Calling Init before running is not
// required. Its primary use is to populate screen dimensions before running an
// application.
func (a *Application) Init() error {
	a.Lock()
	defer a.Unlock()
	return a.init()
}

func (a *Application) init() error {
	if a.screen != nil {
		return nil
	}

	var err error
	a.screen, err = tcell.NewScreen()
	if err != nil {
		return err
	}
	if err = a.screen.Init(); err != nil {
		return err
	}
	a.width, a.height = a.screen.Size()
	if a.enableBracketedPaste {
		a.screen.EnablePaste()
	}
	if a.enableMouse {
		a.screen.EnableMouse()
	}
	return nil
}

// EnableBracketedPaste enables bracketed paste mode, which is enabled by default.
func (a *Application) EnableBracketedPaste(enable bool) {
	a.Lock()
	defer a.Unlock()
	if enable != a.enableBracketedPaste && a.screen != nil {
		if enable {
			a.screen.EnablePaste()
		} else {
			a.screen.DisablePaste()
		}
	}
	a.enableBracketedPaste = enable
}

// EnableMouse enables mouse events.
func (a *Application) EnableMouse(enable bool) {
	a.Lock()
	defer a.Unlock()
	if enable != a.enableMouse && a.screen != nil {
		if enable {
			a.screen.EnableMouse()
		} else {
			a.screen.DisableMouse()
		}
	}
	a.enableMouse = enable
}

// Run starts the application and thus the event loop. This function returns
// when Stop() was called.
func (a *Application) Run() error {
	a.Lock()

	// Initialize screen
	err := a.init()
	if err != nil {
		a.Unlock()
		return err
	}

	defer a.HandlePanic()

	// Draw the screen for the first time.
	a.Unlock()
	a.draw()

	// Separate loop to wait for screen replacement events.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer a.HandlePanic()

		defer wg.Done()
		for {
			a.RLock()
			screen := a.screen
			a.RUnlock()
			if screen == nil {
				// We have no screen. Let's stop.
				a.QueueEvent(nil)
				break
			}

			// A screen was finalized (event is nil). Wait for a new screen.
			screen = <-a.screenReplacement
			if screen == nil {
				// No new screen. We're done.
				a.QueueEvent(nil)
				return
			}

			// We have a new screen. Keep going.
			a.Lock()
			a.screen = screen
			a.Unlock()

			// Initialize and draw this screen.
			if err := screen.Init(); err != nil {
				panic(err)
			}
			if a.enableBracketedPaste {
				screen.EnablePaste()
			}
			if a.enableMouse {
				screen.EnableMouse()
			}

			a.draw()
		}
	}()

	handle := func(event interface{}) {
		a.RLock()
		p := a.focus
		inputCapture := a.inputCapture
		screen := a.screen
		a.RUnlock()

		switch event := event.(type) {
		case *tcell.EventKey:
			// Intercept keys.
			if inputCapture != nil {
				event = inputCapture(event)
				if event == nil {
					a.draw()
					return // Don't forward event.
				}
			}

			// Ctrl-C closes the application.
			if event.Key() == tcell.KeyCtrlC {
				a.Stop()
				return
			}

			// Pass other key events to the currently focused primitive.
			if p != nil {
				if handler := p.InputHandler(); handler != nil {
					handler(event, func(p Primitive) {
						a.SetFocus(p)
					})
					a.draw()
				}
			}
		case *tcell.EventResize:
			// Throttle resize events.
			if time.Since(a.lastResize) < resizeEventThrottle {
				// Stop timer
				if a.throttleResize != nil && !a.throttleResize.Stop() {
					select {
					case <-a.throttleResize.C:
					default:
					}
				}

				event := event // Capture

				// Start timer
				a.throttleResize = time.AfterFunc(resizeEventThrottle, func() {
					a.events <- event
				})

				return
			}

			a.lastResize = time.Now()

			if screen == nil {
				return
			}

			screen.Clear()
			a.width, a.height = event.Size()

			// Call afterResize handler if there is one.
			if a.afterResize != nil {
				a.afterResize(a.width, a.height)
			}

			a.draw()
		case *tcell.EventMouse:
			consumed, isMouseDownAction := a.fireMouseActions(event)
			if consumed {
				a.draw()
			}
			a.lastMouseButtons = event.Buttons()
			if isMouseDownAction {
				a.mouseDownX, a.mouseDownY = event.Position()
			}
		}
	}

	semaphore := &sync.Mutex{}

	go func() {
		defer a.HandlePanic()

		for update := range a.updates {
			semaphore.Lock()
			update()
			semaphore.Unlock()
		}
	}()

	go func() {
		defer a.HandlePanic()

		for event := range a.events {
			semaphore.Lock()
			handle(event)
			semaphore.Unlock()
		}
	}()

	// Start screen event loop.
	for {
		a.Lock()
		screen := a.screen
		a.Unlock()

		if screen == nil {
			break
		}

		// Wait for next event.
		event := screen.PollEvent()
		if event == nil {
			break
		}

		semaphore.Lock()
		handle(event)
		semaphore.Unlock()
	}

	// Wait for the screen replacement event loop to finish.
	wg.Wait()
	a.screen = nil

	return nil
}

// fireMouseActions analyzes the provided mouse event, derives mouse actions
// from it and then forwards them to the corresponding primitives.
func (a *Application) fireMouseActions(event *tcell.EventMouse) (consumed, isMouseDownAction bool) {
	// We want to relay follow-up events to the same target primitive.
	var targetPrimitive Primitive

	// Helper function to fire a mouse action.
	fire := func(action MouseAction) {
		switch action {
		case MouseLeftDown, MouseMiddleDown, MouseRightDown:
			isMouseDownAction = true
		}

		// Intercept event.
		if a.mouseCapture != nil {
			event, action = a.mouseCapture(event, action)
			if event == nil {
				consumed = true
				return // Don't forward event.
			}
		}

		// Determine the target primitive.
		var primitive, capturingPrimitive Primitive
		if a.mouseCapturingPrimitive != nil {
			primitive = a.mouseCapturingPrimitive
			targetPrimitive = a.mouseCapturingPrimitive
		} else if targetPrimitive != nil {
			primitive = targetPrimitive
		} else {
			primitive = a.root
		}
		if primitive != nil {
			if handler := primitive.MouseHandler(); handler != nil {
				var wasConsumed bool
				wasConsumed, capturingPrimitive = handler(action, event, func(p Primitive) {
					a.SetFocus(p)
				})
				if wasConsumed {
					consumed = true
				}
			}
		}
		a.mouseCapturingPrimitive = capturingPrimitive
	}

	x, y := event.Position()
	buttons := event.Buttons()
	clickMoved := x != a.mouseDownX || y != a.mouseDownY
	buttonChanges := buttons ^ a.lastMouseButtons

	if x != a.lastMouseX || y != a.lastMouseY {
		fire(MouseMove)
		a.lastMouseX = x
		a.lastMouseY = y
	}

	for _, buttonEvent := range []struct {
		button                  tcell.ButtonMask
		down, up, click, dclick MouseAction
	}{
		{tcell.ButtonPrimary, MouseLeftDown, MouseLeftUp, MouseLeftClick, MouseLeftDoubleClick},
		{tcell.ButtonMiddle, MouseMiddleDown, MouseMiddleUp, MouseMiddleClick, MouseMiddleDoubleClick},
		{tcell.ButtonSecondary, MouseRightDown, MouseRightUp, MouseRightClick, MouseRightDoubleClick},
	} {
		if buttonChanges&buttonEvent.button != 0 {
			if buttons&buttonEvent.button != 0 {
				fire(buttonEvent.down)
			} else {
				fire(buttonEvent.up)
				if !clickMoved {
					if a.doubleClickInterval == 0 || a.lastMouseClick.Add(a.doubleClickInterval).Before(time.Now()) {
						fire(buttonEvent.click)
						a.lastMouseClick = time.Now()
					} else {
						fire(buttonEvent.dclick)
						a.lastMouseClick = time.Time{} // reset
					}
				}
			}
		}
	}

	for _, wheelEvent := range []struct {
		button tcell.ButtonMask
		action MouseAction
	}{
		{tcell.WheelUp, MouseScrollUp},
		{tcell.WheelDown, MouseScrollDown},
		{tcell.WheelLeft, MouseScrollLeft},
		{tcell.WheelRight, MouseScrollRight}} {
		if buttons&wheelEvent.button != 0 {
			fire(wheelEvent.action)
		}
	}

	return consumed, isMouseDownAction
}

// Stop stops the application, causing Run() to return.
func (a *Application) Stop() {
	a.Lock()
	defer a.Unlock()

	a.finalizeScreen()
	a.screenReplacement <- nil
}

func (a *Application) finalizeScreen() {
	screen := a.screen
	if screen == nil {
		return
	}

	a.screen = nil
	screen.Fini()
}

// Suspend temporarily suspends the application by exiting terminal UI mode and
// invoking the provided function "f". When "f" returns, terminal UI mode is
// entered again and the application resumes.
//
// A return value of true indicates that the application was suspended and "f"
// was called. If false is returned, the application was already suspended,
// terminal UI mode was not exited, and "f" was not called.
func (a *Application) Suspend(f func()) bool {
	a.Lock()
	if a.screen == nil {
		a.Unlock()
		return false // Screen has not yet been initialized.
	}
	err := a.screen.Suspend()
	a.Unlock()
	if err != nil {
		panic(err)
	}

	// Wait for "f" to return.
	f()

	a.Lock()
	err = a.screen.Resume()
	a.Unlock()
	if err != nil {
		panic(err)
	}

	return true
}

// Draw draws the provided primitives on the screen, or when no primitives are
// provided, draws the application's root primitive (i.e. the entire screen).
//
// When one or more primitives are supplied, the Draw functions of the
// primitives are called. Handlers set via BeforeDrawFunc and AfterDrawFunc are
// not called.
//
// When no primitives are provided, the Draw function of the application's root
// primitive is called. This results in drawing the entire screen. Handlers set
// via BeforeDrawFunc and AfterDrawFunc are also called.
func (a *Application) Draw(p ...Primitive) {
	a.QueueUpdate(func() {
		if len(p) == 0 {
			a.draw()
			return
		}

		a.Lock()
		if a.screen != nil {
			for _, primitive := range p {
				primitive.Draw(a.screen)
			}
			a.screen.Show()
		}
		a.Unlock()
	})
}

// draw actually does what Draw() promises to do.
func (a *Application) draw() {
	a.Lock()

	screen := a.screen
	root := a.root
	fullscreen := a.rootFullscreen
	before := a.beforeDraw
	after := a.afterDraw

	// Maybe we're not ready yet or not anymore.
	if screen == nil || root == nil {
		a.Unlock()
		return
	}

	// Resize if requested.
	if fullscreen {
		root.SetRect(0, 0, a.width, a.height)
	}

	// Call before handler if there is one.
	if before != nil {
		a.Unlock()
		if before(screen) {
			screen.Show()
			return
		}
	} else {
		a.Unlock()
	}

	// Draw all primitives.
	root.Draw(screen)

	// Call after handler if there is one.
	if after != nil {
		after(screen)
	}

	// Sync screen.
	screen.Show()
}

// SetBeforeDrawFunc installs a callback function which is invoked just before
// the root primitive is drawn during screen updates. If the function returns
// true, drawing will not continue, i.e. the root primitive will not be drawn
// (and an after-draw-handler will not be called).
//
// Note that the screen is not cleared by the application. To clear the screen,
// you may call screen.Clear().
//
// Provide nil to uninstall the callback function.
func (a *Application) SetBeforeDrawFunc(handler func(screen tcell.Screen) bool) {
	a.Lock()
	defer a.Unlock()

	a.beforeDraw = handler
}

// GetBeforeDrawFunc returns the callback function installed with
// SetBeforeDrawFunc() or nil if none has been installed.
func (a *Application) GetBeforeDrawFunc() func(screen tcell.Screen) bool {
	a.RLock()
	defer a.RUnlock()

	return a.beforeDraw
}

// SetAfterDrawFunc installs a callback function which is invoked after the root
// primitive was drawn during screen updates.
//
// Provide nil to uninstall the callback function.
func (a *Application) SetAfterDrawFunc(handler func(screen tcell.Screen)) {
	a.Lock()
	defer a.Unlock()

	a.afterDraw = handler
}

// GetAfterDrawFunc returns the callback function installed with
// SetAfterDrawFunc() or nil if none has been installed.
func (a *Application) GetAfterDrawFunc() func(screen tcell.Screen) {
	a.RLock()
	defer a.RUnlock()

	return a.afterDraw
}

// SetRoot sets the root primitive for this application. If "fullscreen" is set
// to true, the root primitive's position will be changed to fill the screen.
//
// This function must be called at least once or nothing will be displayed when
// the application starts.
//
// It also calls SetFocus() on the primitive and draws the application.
func (a *Application) SetRoot(root Primitive, fullscreen bool) {
	a.Lock()
	a.root = root
	a.rootFullscreen = fullscreen
	if a.screen != nil {
		a.screen.Clear()
	}
	a.Unlock()

	a.SetFocus(root)

	a.Draw()
}

// ResizeToFullScreen resizes the given primitive such that it fills the entire
// screen.
func (a *Application) ResizeToFullScreen(p Primitive) {
	a.RLock()
	width, height := a.width, a.height
	a.RUnlock()
	p.SetRect(0, 0, width, height)
}

// SetAfterResizeFunc installs a callback function which is invoked when the
// application's window is initialized, and when the application's window size
// changes. After invoking this callback the screen is cleared and the
// application is drawn.
//
// Provide nil to uninstall the callback function.
func (a *Application) SetAfterResizeFunc(handler func(width int, height int)) {
	a.Lock()
	defer a.Unlock()

	a.afterResize = handler
}

// GetAfterResizeFunc returns the callback function installed with
// SetAfterResizeFunc() or nil if none has been installed.
func (a *Application) GetAfterResizeFunc() func(width int, height int) {
	a.RLock()
	defer a.RUnlock()

	return a.afterResize
}

// SetFocus sets the focus on a new primitive. All key events will be redirected
// to that primitive. Callers must ensure that the primitive will handle key
// events.
//
// Blur() will be called on the previously focused primitive. Focus() will be
// called on the new primitive.
func (a *Application) SetFocus(p Primitive) {
	a.Lock()

	if a.beforeFocus != nil {
		a.Unlock()
		ok := a.beforeFocus(p)
		if !ok {
			return
		}
		a.Lock()
	}

	if a.focus != nil {
		a.focus.Blur()
	}

	a.focus = p

	if a.screen != nil {
		a.screen.HideCursor()
	}

	if a.afterFocus != nil {
		a.Unlock()

		a.afterFocus(p)
	} else {
		a.Unlock()
	}

	if p != nil {
		p.Focus(func(p Primitive) {
			a.SetFocus(p)
		})
	}
}

// GetFocus returns the primitive which has the current focus. If none has it,
// nil is returned.
func (a *Application) GetFocus() Primitive {
	a.RLock()
	defer a.RUnlock()

	return a.focus
}

// SetBeforeFocusFunc installs a callback function which is invoked before the
// application's focus changes. Return false to maintain the current focus.
//
// Provide nil to uninstall the callback function.
func (a *Application) SetBeforeFocusFunc(handler func(p Primitive) bool) {
	a.Lock()
	defer a.Unlock()

	a.beforeFocus = handler
}

// SetAfterFocusFunc installs a callback function which is invoked after the
// application's focus changes.
//
// Provide nil to uninstall the callback function.
func (a *Application) SetAfterFocusFunc(handler func(p Primitive)) {
	a.Lock()
	defer a.Unlock()

	a.afterFocus = handler
}

// QueueUpdate queues a function to be executed as part of the event loop.
//
// Note that Draw() is not implicitly called after the execution of f as that
// may not be desirable. You can call Draw() from f if the screen should be
// refreshed after each update. Alternatively, use QueueUpdateDraw() to follow
// up with an immediate refresh of the screen.
func (a *Application) QueueUpdate(f func()) {
	a.updates <- f
}

// QueueUpdateDraw works like QueueUpdate() except, when one or more primitives
// are provided, the primitives are drawn after the provided function returns.
// When no primitives are provided, the entire screen is drawn after the
// provided function returns.
func (a *Application) QueueUpdateDraw(f func(), p ...Primitive) {
	a.QueueUpdate(func() {
		f()

		if len(p) == 0 {
			a.draw()
			return
		}
		a.Lock()
		if a.screen != nil {
			for _, primitive := range p {
				primitive.Draw(a.screen)
			}
			a.screen.Show()
		}
		a.Unlock()
	})
}

// QueueEvent sends an event to the Application event loop.
//
// It is not recommended for event to be nil.
func (a *Application) QueueEvent(event tcell.Event) {
	a.events <- event
}

// RingBell sends a bell code to the terminal.
func (a *Application) RingBell() {
	a.QueueUpdate(func() {
		fmt.Print(string(byte(7)))
	})
}
