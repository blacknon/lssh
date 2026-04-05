package mview

import (
	"sync"

	"github.com/gdamore/tcell/v2"
)

// Tree navigation events.
const (
	treeNone int = iota
	treeHome
	treeEnd
	treeUp
	treeDown
	treePageUp
	treePageDown
)

// TreeNode represents one node in a tree view.
type TreeNode struct {
	// The reference object.
	reference interface{}

	// This node's child nodes.
	children []*TreeNode

	// The item's text.
	text string

	// The text color.
	color tcell.Color

	// Whether or not this node can be focused and selected.
	selectable bool

	// Whether or not this node's children should be displayed.
	expanded bool

	// The additional horizontal indent of this node's text.
	indent int

	// An optional function which is called when the user focuses this node.
	focused func()

	// An optional function which is called when the user selects this node.
	selected func()

	// Temporary member variables.
	parent    *TreeNode // The parent node (nil for the root).
	level     int       // The hierarchy level (0 for the root, 1 for its children, and so on).
	graphicsX int       // The x-coordinate of the left-most graphics rune.
	textX     int       // The x-coordinate of the first rune of the text.

	sync.RWMutex
}

// NewTreeNode returns a new tree node.
func NewTreeNode(text string) *TreeNode {
	return &TreeNode{
		text:       text,
		color:      Styles.PrimaryTextColor,
		indent:     2,
		expanded:   true,
		selectable: true,
	}
}

// Walk traverses this node's subtree in depth-first, pre-order (NLR) order and
// calls the provided callback function on each traversed node (which includes
// this node) with the traversed node and its parent node (nil for this node).
// The callback returns whether traversal should continue with the traversed
// node's child nodes (true) or not recurse any deeper (false).
func (n *TreeNode) Walk(callback func(node, parent *TreeNode) bool) {
	n.Lock()
	defer n.Unlock()

	n.walk(callback)
}

func (n *TreeNode) walk(callback func(node, parent *TreeNode) bool) {
	n.parent = nil
	nodes := []*TreeNode{n}
	for len(nodes) > 0 {
		// Pop the top node and process it.
		node := nodes[len(nodes)-1]
		nodes = nodes[:len(nodes)-1]
		if !callback(node, node.parent) {
			// Don't add any children.
			continue
		}

		// Add children in reverse order.
		for index := len(node.children) - 1; index >= 0; index-- {
			node.children[index].parent = node
			nodes = append(nodes, node.children[index])
		}
	}
}

// SetReference allows you to store a reference of any type in this node. This
// will allow you to establish a mapping between the TreeView hierarchy and your
// internal tree structure.
func (n *TreeNode) SetReference(reference interface{}) {
	n.Lock()
	defer n.Unlock()

	n.reference = reference
}

// GetReference returns this node's reference object.
func (n *TreeNode) GetReference() interface{} {
	n.RLock()
	defer n.RUnlock()

	return n.reference
}

// SetChildren sets this node's child nodes.
func (n *TreeNode) SetChildren(childNodes []*TreeNode) {
	n.Lock()
	defer n.Unlock()

	n.children = childNodes
}

// GetText returns this node's text.
func (n *TreeNode) GetText() string {
	n.RLock()
	defer n.RUnlock()

	return n.text
}

// GetChildren returns this node's children.
func (n *TreeNode) GetChildren() []*TreeNode {
	n.RLock()
	defer n.RUnlock()

	return n.children
}

// ClearChildren removes all child nodes from this node.
func (n *TreeNode) ClearChildren() {
	n.Lock()
	defer n.Unlock()

	n.children = nil
}

// AddChild adds a new child node to this node.
func (n *TreeNode) AddChild(node *TreeNode) {
	n.Lock()
	defer n.Unlock()

	n.children = append(n.children, node)
}

// SetSelectable sets a flag indicating whether this node can be focused and
// selected by the user.
func (n *TreeNode) SetSelectable(selectable bool) {
	n.Lock()
	defer n.Unlock()

	n.selectable = selectable
}

// SetFocusedFunc sets the function which is called when the user navigates to
// this node.
//
// This function is also called when the user selects this node.
func (n *TreeNode) SetFocusedFunc(handler func()) {
	n.Lock()
	defer n.Unlock()

	n.focused = handler
}

// SetSelectedFunc sets a function which is called when the user selects this
// node by hitting Enter when it is focused.
func (n *TreeNode) SetSelectedFunc(handler func()) {
	n.Lock()
	defer n.Unlock()

	n.selected = handler
}

// SetExpanded sets whether or not this node's child nodes should be displayed.
func (n *TreeNode) SetExpanded(expanded bool) {
	n.Lock()
	defer n.Unlock()

	n.expanded = expanded
}

// Expand makes the child nodes of this node appear.
func (n *TreeNode) Expand() {
	n.Lock()
	defer n.Unlock()

	n.expanded = true
}

// Collapse makes the child nodes of this node disappear.
func (n *TreeNode) Collapse() {
	n.Lock()
	defer n.Unlock()

	n.expanded = false
}

// ExpandAll expands this node and all descendent nodes.
func (n *TreeNode) ExpandAll() {
	n.Walk(func(node, parent *TreeNode) bool {
		node.expanded = true
		return true
	})
}

// CollapseAll collapses this node and all descendent nodes.
func (n *TreeNode) CollapseAll() {
	n.Walk(func(node, parent *TreeNode) bool {
		n.expanded = false
		return true
	})
}

// IsExpanded returns whether the child nodes of this node are visible.
func (n *TreeNode) IsExpanded() bool {
	n.RLock()
	defer n.RUnlock()

	return n.expanded
}

// SetText sets the node's text which is displayed.
func (n *TreeNode) SetText(text string) {
	n.Lock()
	defer n.Unlock()

	n.text = text
}

// GetColor returns the node's color.
func (n *TreeNode) GetColor() tcell.Color {
	n.RLock()
	defer n.RUnlock()

	return n.color
}

// SetColor sets the node's text color.
func (n *TreeNode) SetColor(color tcell.Color) {
	n.Lock()
	defer n.Unlock()

	n.color = color
}

// SetIndent sets an additional indentation for this node's text. A value of 0
// keeps the text as far left as possible with a minimum of line graphics. Any
// value greater than that moves the text to the right.
func (n *TreeNode) SetIndent(indent int) {
	n.Lock()
	defer n.Unlock()

	n.indent = indent
}

// TreeView displays tree structures. A tree consists of nodes (TreeNode
// objects) where each node has zero or more child nodes and exactly one parent
// node (except for the root node which has no parent node).
//
// The SetRoot() function is used to specify the root of the tree. Other nodes
// are added locally to the root node or any of its descendents. See the
// TreeNode documentation for details on node attributes. (You can use
// SetReference() to store a reference to nodes of your own tree structure.)
//
// Nodes can be focused by calling SetCurrentNode(). The user can navigate the
// selection or the tree by using the following keys:
//
//   - j, down arrow, right arrow: Move (the selection) down by one node.
//   - k, up arrow, left arrow: Move (the selection) up by one node.
//   - g, home: Move (the selection) to the top.
//   - G, end: Move (the selection) to the bottom.
//   - Ctrl-F, page down: Move (the selection) down by one page.
//   - Ctrl-B, page up: Move (the selection) up by one page.
//
// Selected nodes can trigger the "selected" callback when the user hits Enter.
//
// The root node corresponds to level 0, its children correspond to level 1,
// their children to level 2, and so on. Per default, the first level that is
// displayed is 0, i.e. the root node. You can call SetTopLevel() to hide
// levels.
//
// If graphics are turned on (see SetGraphics()), lines indicate the tree's
// hierarchy. Alternative (or additionally), you can set different prefixes
// using SetPrefixes() for different levels, for example to display hierarchical
// bullet point lists.
type TreeView struct {
	*Box

	// The root node.
	root *TreeNode

	// The currently focused node or nil if no node is focused.
	currentNode *TreeNode

	// The movement to be performed during the call to Draw(), one of the
	// constants defined above.
	movement int

	// The top hierarchical level shown. (0 corresponds to the root level.)
	topLevel int

	// Strings drawn before the nodes, based on their level.
	prefixes [][]byte

	// Vertical scroll offset.
	offsetY int

	// If set to true, all node texts will be aligned horizontally.
	align bool

	// If set to true, the tree structure is drawn using lines.
	graphics bool

	// The text color for selected items.
	selectedTextColor *tcell.Color

	// The background color for selected items.
	selectedBackgroundColor *tcell.Color

	// The color of the lines.
	graphicsColor tcell.Color

	// Visibility of the scroll bar.
	scrollBarVisibility ScrollBarVisibility

	// The scroll bar color.
	scrollBarColor tcell.Color

	// An optional function called when the focused tree item changes.
	changed func(node *TreeNode)

	// An optional function called when a tree item is selected.
	selected func(node *TreeNode)

	// An optional function called when the user moves away from this primitive.
	done func(key tcell.Key)

	// The visible nodes, top-down, as set by process().
	nodes []*TreeNode

	sync.RWMutex
}

// NewTreeView returns a new tree view.
func NewTreeView() *TreeView {
	return &TreeView{
		Box:                 NewBox(),
		scrollBarVisibility: ScrollBarAuto,
		graphics:            true,
		graphicsColor:       Styles.GraphicsColor,
		scrollBarColor:      Styles.ScrollBarColor,
	}
}

// SetRoot sets the root node of the tree.
func (t *TreeView) SetRoot(root *TreeNode) {
	t.Lock()
	defer t.Unlock()

	t.root = root
}

// GetRoot returns the root node of the tree. If no such node was previously
// set, nil is returned.
func (t *TreeView) GetRoot() *TreeNode {
	t.RLock()
	defer t.RUnlock()

	return t.root
}

// SetCurrentNode focuses a node or, when provided with nil, clears focus.
// Selected nodes must be visible and selectable, or else the selection will be
// changed to the top-most selectable and visible node.
//
// This function does NOT trigger the "changed" callback.
func (t *TreeView) SetCurrentNode(node *TreeNode) {
	t.Lock()
	defer t.Unlock()

	t.currentNode = node
	if t.currentNode.focused != nil {
		t.Unlock()
		t.currentNode.focused()
		t.Lock()
	}
}

// GetCurrentNode returns the currently selected node or nil of no node is
// currently selected.
func (t *TreeView) GetCurrentNode() *TreeNode {
	t.RLock()
	defer t.RUnlock()

	return t.currentNode
}

// SetTopLevel sets the first tree level that is visible with 0 referring to the
// root, 1 to the root's child nodes, and so on. Nodes above the top level are
// not displayed.
func (t *TreeView) SetTopLevel(topLevel int) {
	t.Lock()
	defer t.Unlock()

	t.topLevel = topLevel
}

// SetPrefixes defines the strings drawn before the nodes' texts. This is a
// slice of strings where each element corresponds to a node's hierarchy level,
// i.e. 0 for the root, 1 for the root's children, and so on (levels will
// cycle).
//
// For example, to display a hierarchical list with bullet points:
//
//   treeView.SetGraphics(false).
//     SetPrefixes([]string{"* ", "- ", "x "})
func (t *TreeView) SetPrefixes(prefixes []string) {
	t.Lock()
	defer t.Unlock()

	t.prefixes = make([][]byte, len(prefixes))
	for i := range prefixes {
		t.prefixes[i] = []byte(prefixes[i])
	}
}

// SetAlign controls the horizontal alignment of the node texts. If set to true,
// all texts except that of top-level nodes will be placed in the same column.
// If set to false, they will indent with the hierarchy.
func (t *TreeView) SetAlign(align bool) {
	t.Lock()
	defer t.Unlock()

	t.align = align
}

// SetGraphics sets a flag which determines whether or not line graphics are
// drawn to illustrate the tree's hierarchy.
func (t *TreeView) SetGraphics(showGraphics bool) {
	t.Lock()
	defer t.Unlock()

	t.graphics = showGraphics
}

// SetSelectedTextColor sets the text color of selected items.
func (t *TreeView) SetSelectedTextColor(color tcell.Color) {
	t.Lock()
	defer t.Unlock()
	t.selectedTextColor = &color
}

// SetSelectedBackgroundColor sets the background color of selected items.
func (t *TreeView) SetSelectedBackgroundColor(color tcell.Color) {
	t.Lock()
	defer t.Unlock()
	t.selectedBackgroundColor = &color
}

// SetGraphicsColor sets the colors of the lines used to draw the tree structure.
func (t *TreeView) SetGraphicsColor(color tcell.Color) {
	t.Lock()
	defer t.Unlock()

	t.graphicsColor = color
}

// SetScrollBarVisibility specifies the display of the scroll bar.
func (t *TreeView) SetScrollBarVisibility(visibility ScrollBarVisibility) {
	t.Lock()
	defer t.Unlock()

	t.scrollBarVisibility = visibility
}

// SetScrollBarColor sets the color of the scroll bar.
func (t *TreeView) SetScrollBarColor(color tcell.Color) {
	t.Lock()
	defer t.Unlock()

	t.scrollBarColor = color
}

// SetChangedFunc sets the function which is called when the user navigates to
// a new tree node.
func (t *TreeView) SetChangedFunc(handler func(node *TreeNode)) {
	t.Lock()
	defer t.Unlock()

	t.changed = handler
}

// SetSelectedFunc sets the function which is called when the user selects a
// node by pressing Enter on the current selection.
func (t *TreeView) SetSelectedFunc(handler func(node *TreeNode)) {
	t.Lock()
	defer t.Unlock()

	t.selected = handler
}

// SetDoneFunc sets a handler which is called whenever the user presses the
// Escape, Tab, or Backtab key.
func (t *TreeView) SetDoneFunc(handler func(key tcell.Key)) {
	t.Lock()
	defer t.Unlock()

	t.done = handler
}

// GetScrollOffset returns the number of node rows that were skipped at the top
// of the tree view. Note that when the user navigates the tree view, this value
// is only updated after the tree view has been redrawn.
func (t *TreeView) GetScrollOffset() int {
	t.RLock()
	defer t.RUnlock()

	return t.offsetY
}

// GetRowCount returns the number of "visible" nodes. This includes nodes which
// fall outside the tree view's box but notably does not include the children
// of collapsed nodes. Note that this value is only up to date after the tree
// view has been drawn.
func (t *TreeView) GetRowCount() int {
	t.RLock()
	defer t.RUnlock()

	return len(t.nodes)
}

// Transform modifies the current selection.
func (t *TreeView) Transform(tr Transformation) {
	t.Lock()
	defer t.Unlock()

	switch tr {
	case TransformFirstItem:
		t.movement = treeHome
	case TransformLastItem:
		t.movement = treeEnd
	case TransformPreviousItem:
		t.movement = treeUp
	case TransformNextItem:
		t.movement = treeDown
	case TransformPreviousPage:
		t.movement = treePageUp
	case TransformNextPage:
		t.movement = treePageDown
	}

	t.process()
}

// process builds the visible tree, populates the "nodes" slice, and processes
// pending selection actions.
func (t *TreeView) process() {
	_, _, _, height := t.GetInnerRect()

	// Determine visible nodes and their placement.
	var graphicsOffset, maxTextX int
	t.nodes = nil
	selectedIndex := -1
	topLevelGraphicsX := -1
	if t.graphics {
		graphicsOffset = 1
	}
	t.root.walk(func(node, parent *TreeNode) bool {
		// Set node attributes.
		node.parent = parent
		if parent == nil {
			node.level = 0
			node.graphicsX = 0
			node.textX = 0
		} else {
			node.level = parent.level + 1
			node.graphicsX = parent.textX
			node.textX = node.graphicsX + graphicsOffset + node.indent
		}
		if !t.graphics && t.align {
			// Without graphics, we align nodes on the first column.
			node.textX = 0
		}
		if node.level == t.topLevel {
			// No graphics for top level nodes.
			node.graphicsX = 0
			node.textX = 0
		}

		// Add the node to the list.
		if node.level >= t.topLevel {
			// This node will be visible.
			if node.textX > maxTextX {
				maxTextX = node.textX
			}
			if node == t.currentNode && node.selectable {
				selectedIndex = len(t.nodes)
			}

			// Maybe we want to skip this level.
			if t.topLevel == node.level && (topLevelGraphicsX < 0 || node.graphicsX < topLevelGraphicsX) {
				topLevelGraphicsX = node.graphicsX
			}

			t.nodes = append(t.nodes, node)
		}

		// Recurse if desired.
		return node.expanded
	})

	// Post-process positions.
	for _, node := range t.nodes {
		// If text must align, we correct the positions.
		if t.align && node.level > t.topLevel {
			node.textX = maxTextX
		}

		// If we skipped levels, shift to the left.
		if topLevelGraphicsX > 0 {
			node.graphicsX -= topLevelGraphicsX
			node.textX -= topLevelGraphicsX
		}
	}

	// Process selection. (Also trigger events if necessary.)
	if selectedIndex >= 0 {
		// Move the selection.
		newSelectedIndex := selectedIndex
	MovementSwitch:
		switch t.movement {
		case treeUp:
			for newSelectedIndex > 0 {
				newSelectedIndex--
				if t.nodes[newSelectedIndex].selectable {
					break MovementSwitch
				}
			}
			newSelectedIndex = selectedIndex
		case treeDown:
			for newSelectedIndex < len(t.nodes)-1 {
				newSelectedIndex++
				if t.nodes[newSelectedIndex].selectable {
					break MovementSwitch
				}
			}
			newSelectedIndex = selectedIndex
		case treeHome:
			for newSelectedIndex = 0; newSelectedIndex < len(t.nodes); newSelectedIndex++ {
				if t.nodes[newSelectedIndex].selectable {
					break MovementSwitch
				}
			}
			newSelectedIndex = selectedIndex
		case treeEnd:
			for newSelectedIndex = len(t.nodes) - 1; newSelectedIndex >= 0; newSelectedIndex-- {
				if t.nodes[newSelectedIndex].selectable {
					break MovementSwitch
				}
			}
			newSelectedIndex = selectedIndex
		case treePageDown:
			if newSelectedIndex+height < len(t.nodes) {
				newSelectedIndex += height
			} else {
				newSelectedIndex = len(t.nodes) - 1
			}
			for ; newSelectedIndex < len(t.nodes); newSelectedIndex++ {
				if t.nodes[newSelectedIndex].selectable {
					break MovementSwitch
				}
			}
			newSelectedIndex = selectedIndex
		case treePageUp:
			if newSelectedIndex >= height {
				newSelectedIndex -= height
			} else {
				newSelectedIndex = 0
			}
			for ; newSelectedIndex >= 0; newSelectedIndex-- {
				if t.nodes[newSelectedIndex].selectable {
					break MovementSwitch
				}
			}
			newSelectedIndex = selectedIndex
		}
		t.currentNode = t.nodes[newSelectedIndex]
		if newSelectedIndex != selectedIndex {
			t.movement = treeNone
			if t.changed != nil {
				t.Unlock()
				t.changed(t.currentNode)
				t.Lock()
			}
			if t.currentNode.focused != nil {
				t.Unlock()
				t.currentNode.focused()
				t.Lock()
			}
		}
		selectedIndex = newSelectedIndex

		// Move selection into viewport.
		if selectedIndex-t.offsetY >= height {
			t.offsetY = selectedIndex - height + 1
		}
		if selectedIndex < t.offsetY {
			t.offsetY = selectedIndex
		}
	} else {
		// If selection is not visible or selectable, select the first candidate.
		if t.currentNode != nil {
			for index, node := range t.nodes {
				if node.selectable {
					selectedIndex = index
					t.currentNode = node
					break
				}
			}
		}
		if selectedIndex < 0 {
			t.currentNode = nil
		}
	}
}

// Draw draws this primitive onto the screen.
func (t *TreeView) Draw(screen tcell.Screen) {
	if !t.GetVisible() {
		return
	}

	t.Box.Draw(screen)

	t.Lock()
	defer t.Unlock()

	if t.root == nil {
		return
	}

	t.process()

	// Scroll the tree.
	x, y, width, height := t.GetInnerRect()
	switch t.movement {
	case treeUp:
		t.offsetY--
	case treeDown:
		t.offsetY++
	case treeHome:
		t.offsetY = 0
	case treeEnd:
		t.offsetY = len(t.nodes)
	case treePageUp:
		t.offsetY -= height
	case treePageDown:
		t.offsetY += height
	}
	t.movement = treeNone

	// Fix invalid offsets.
	if t.offsetY >= len(t.nodes)-height {
		t.offsetY = len(t.nodes) - height
	}
	if t.offsetY < 0 {
		t.offsetY = 0
	}

	// Calculate scroll bar position.
	rows := len(t.nodes)
	cursor := int(float64(rows) * (float64(t.offsetY) / float64(rows-height)))

	// Draw the tree.
	posY := y
	lineStyle := tcell.StyleDefault.Background(t.backgroundColor).Foreground(t.graphicsColor)
	for index, node := range t.nodes {
		// Skip invisible parts.
		if posY >= y+height {
			break
		}
		if index < t.offsetY {
			continue
		}

		// Draw the graphics.
		if t.graphics {
			// Draw ancestor branches.
			ancestor := node.parent
			for ancestor != nil && ancestor.parent != nil && ancestor.parent.level >= t.topLevel {
				if ancestor.graphicsX >= width {
					continue
				}

				// Draw a branch if this ancestor is not a last child.
				if ancestor.parent.children[len(ancestor.parent.children)-1] != ancestor {
					if posY-1 >= y && ancestor.textX > ancestor.graphicsX {
						PrintJoinedSemigraphics(screen, x+ancestor.graphicsX, posY-1, Borders.Vertical, t.graphicsColor)
					}
					if posY < y+height {
						screen.SetContent(x+ancestor.graphicsX, posY, Borders.Vertical, nil, lineStyle)
					}
				}
				ancestor = ancestor.parent
			}

			if node.textX > node.graphicsX && node.graphicsX < width {
				// Connect to the node above.
				if posY-1 >= y && t.nodes[index-1].graphicsX <= node.graphicsX && t.nodes[index-1].textX > node.graphicsX {
					PrintJoinedSemigraphics(screen, x+node.graphicsX, posY-1, Borders.TopLeft, t.graphicsColor)
				}

				// Join this node.
				if posY < y+height {
					screen.SetContent(x+node.graphicsX, posY, Borders.BottomLeft, nil, lineStyle)
					for pos := node.graphicsX + 1; pos < node.textX && pos < width; pos++ {
						screen.SetContent(x+pos, posY, Borders.Horizontal, nil, lineStyle)
					}
				}
			}
		}

		// Draw the prefix and the text.
		if node.textX < width && posY < y+height {
			// Prefix.
			var prefixWidth int
			if len(t.prefixes) > 0 {
				_, prefixWidth = Print(screen, t.prefixes[(node.level-t.topLevel)%len(t.prefixes)], x+node.textX, posY, width-node.textX, AlignLeft, node.color)
			}

			// Text.
			if node.textX+prefixWidth < width {
				style := tcell.StyleDefault.Foreground(node.color)
				if node == t.currentNode {
					backgroundColor := node.color
					foregroundColor := t.backgroundColor
					if t.selectedTextColor != nil {
						foregroundColor = *t.selectedTextColor
					}
					if t.selectedBackgroundColor != nil {
						backgroundColor = *t.selectedBackgroundColor
					}
					style = tcell.StyleDefault.Background(backgroundColor).Foreground(foregroundColor)
				}
				PrintStyle(screen, []byte(node.text), x+node.textX+prefixWidth, posY, width-node.textX-prefixWidth, AlignLeft, style)
			}
		}

		// Draw scroll bar.
		RenderScrollBar(screen, t.scrollBarVisibility, x+(width-1), posY, height, rows, cursor, posY-y, t.hasFocus, t.scrollBarColor)

		// Advance.
		posY++
	}
}

// InputHandler returns the handler for this primitive.
func (t *TreeView) InputHandler() func(event *tcell.EventKey, setFocus func(p Primitive)) {
	return t.WrapInputHandler(func(event *tcell.EventKey, setFocus func(p Primitive)) {
		selectNode := func() {
			t.Lock()
			currentNode := t.currentNode
			t.Unlock()
			if currentNode == nil {
				return
			}

			if t.selected != nil {
				t.selected(currentNode)
			}
			if currentNode.focused != nil {
				currentNode.focused()
			}
			if currentNode.selected != nil {
				currentNode.selected()
			}
		}

		t.Lock()
		defer t.Unlock()

		// Because the tree is flattened into a list only at drawing time, we also
		// postpone the (selection) movement to drawing time.
		if HitShortcut(event, Keys.Cancel, Keys.MovePreviousField, Keys.MoveNextField) {
			if t.done != nil {
				t.Unlock()
				t.done(event.Key())
				t.Lock()
			}
		} else if HitShortcut(event, Keys.MoveFirst, Keys.MoveFirst2) {
			t.movement = treeHome
		} else if HitShortcut(event, Keys.MoveLast, Keys.MoveLast2) {
			t.movement = treeEnd
		} else if HitShortcut(event, Keys.MoveUp, Keys.MoveUp2) {
			t.movement = treeUp
		} else if HitShortcut(event, Keys.MoveDown, Keys.MoveDown2) {
			t.movement = treeDown
		} else if HitShortcut(event, Keys.MovePreviousPage) {
			t.movement = treePageUp
		} else if HitShortcut(event, Keys.MoveNextPage) {
			t.movement = treePageDown
		} else if HitShortcut(event, Keys.Select, Keys.Select2) {
			t.Unlock()
			selectNode()
			t.Lock()
		}

		t.process()
	})
}

// MouseHandler returns the mouse handler for this primitive.
func (t *TreeView) MouseHandler() func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive) {
	return t.WrapMouseHandler(func(action MouseAction, event *tcell.EventMouse, setFocus func(p Primitive)) (consumed bool, capture Primitive) {
		x, y := event.Position()
		if !t.InRect(x, y) {
			return false, nil
		}

		switch action {
		case MouseLeftClick:
			_, rectY, _, _ := t.GetInnerRect()
			y -= rectY
			if y >= 0 && y < len(t.nodes) {
				node := t.nodes[y]
				if node.selectable {
					if t.currentNode != node && t.changed != nil {
						t.changed(node)
					}
					if t.selected != nil {
						t.selected(node)
					}
					t.currentNode = node
				}
			}
			consumed = true
			setFocus(t)
		case MouseScrollUp:
			t.movement = treeUp
			consumed = true
		case MouseScrollDown:
			t.movement = treeDown
			consumed = true
		}

		return
	})
}
