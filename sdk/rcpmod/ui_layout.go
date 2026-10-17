package rcpmod

import "fmt"

// The layout of a camera page. Every module describes its page in its own
// ui.go file: a Page is rows of blocks, top to bottom; the blocks of a row
// share its width equally. StandardPage is the generic page a module can
// return as it is, or start from and change.

// Page is the layout of one camera's page.
type Page struct {
	Rows []Row `json:"rows"`
}

// Row is blocks side by side, sharing the width equally.
type Row struct {
	Blocks []Block `json:"blocks"`
}

// Block is one panel of the page. Type says which; the other fields
// configure the types that use them.
type Block struct {
	Type string `json:"type"`
	// Title of a BlockControls panel.
	Title string `json:"title,omitempty"`
	// Control picks the control of a BlockImage (Contrast, Saturation or
	// Hue).
	Control string `json:"control,omitempty"`
	// Wheel picks the wheel of a BlockWheel (WheelLift, WheelGamma,
	// WheelGain).
	Wheel string `json:"wheel,omitempty"`
	// Items are the controls of a BlockControls panel, in order.
	Items []Item `json:"items,omitempty"`
	// Columns of a BlockControls panel (1 or 2) or of a BlockGrid.
	Columns int `json:"columns,omitempty"`
	// Blocks of a BlockGrid, laid out in Columns columns.
	Blocks []Block `json:"blocks,omitempty"`
}

// Item is one control of a BlockControls panel and the widget showing it.
type Item struct {
	Control string `json:"control"`
	// Widget is a WidgetXxx constant; empty = the default widget of the
	// control's kind.
	Widget string `json:"widget,omitempty"`
}

// Block types.
const (
	// The standard cards, each driving its standard controls. A card whose
	// controls the camera lacks is shown greyed.
	BlockIris         = "iris"         // Iris (f-stops), AutoIris
	BlockLens         = "lens"         // Zoom lever and Focus, AutoFocus
	BlockGain         = "gain"         // Gain (dB) or ISO
	BlockShutter      = "shutter"      // ShutterSpeed with ShutterAngle
	BlockND           = "nd"           // ND
	BlockWhiteBalance = "whiteBalance" // WhiteBalance, Tint, AutoWhiteBalance
	BlockImage        = "image"        // one of Contrast, Saturation, Hue (Block.Control)
	BlockWheel        = "wheel"        // one color wheel (Block.Wheel), with its R G B Y controls
	BlockPresets      = "presets"      // the presets stored by the application
	// BlockControls is a titled panel of the controls listed in Items,
	// each with its widget.
	BlockControls = "controls"
	// BlockGrid lays Blocks out in Columns columns (1 = stacked).
	BlockGrid = "grid"
	// BlockRemaining shows every visible control that no other block of
	// the page shows, in sections by group: nothing the camera declares is
	// ever unreachable.
	BlockRemaining = "remaining"
	// BlockExposure and BlockColor are the two halves of the standard page:
	// all the exposure and lens cards with the presets; the three wheels
	// with the remaining controls.
	BlockExposure = "exposure"
	BlockColor    = "color"
)

// Wheels of a BlockWheel.
const (
	WheelLift  = "lift"
	WheelGamma = "gamma"
	WheelGain  = "gain"
)

// Widgets of a BlockControls item, and the kinds each accepts.
const (
	WidgetSlider  = "slider"  // range: a slider with − / + and the value
	WidgetStepper = "stepper" // range: − and + around the value
	WidgetButtons = "buttons" // enum: one button per option
	WidgetSelect  = "select"  // enum: a drop-down list
	WidgetSwitch  = "switch"  // toggle: an on/off switch
	WidgetButton  = "button"  // action: one button
	WidgetReadout = "readout" // any read-only kind but action: the value, large
	WidgetText    = "text"    // text: a text field (read-only: a label)
)

// StandardPage is the generic camera page: the exposure and lens cards
// with the presets, then the color wheels with every other control. A
// module that returns it gets the same page as every standard camera.
func StandardPage() Page {
	return Page{Rows: []Row{
		{Blocks: []Block{{Type: BlockExposure}}},
		{Blocks: []Block{{Type: BlockColor}}},
	}}
}

// AddRow returns the page with a row of blocks added at the end.
func (p Page) AddRow(blocks ...Block) Page {
	rows := append([]Row(nil), p.Rows...)
	return Page{Rows: append(rows, Row{Blocks: blocks})}
}

// Controls builds a titled panel of controls in 1 or 2 columns.
func Controls(title string, columns int, items ...Item) Block {
	return Block{Type: BlockControls, Title: title, Columns: columns, Items: items}
}

// Show is an item with the default widget of its control.
func Show(control string) Item { return Item{Control: control} }

// With is an item shown with a given widget.
func With(control, widget string) Item { return Item{Control: control, Widget: widget} }

// Grid lays blocks out in columns.
func Grid(columns int, blocks ...Block) Block {
	return Block{Type: BlockGrid, Columns: columns, Blocks: blocks}
}

// Card is a standard card block (BlockIris, BlockGain...).
func Card(blockType string) Block { return Block{Type: blockType} }

// Image is the card of one image control (Contrast, Saturation or Hue).
func Image(control string) Block { return Block{Type: BlockImage, Control: control} }

// Wheel is one color wheel (WheelLift, WheelGamma or WheelGain).
func Wheel(wheel string) Block { return Block{Type: BlockWheel, Wheel: wheel} }

// Remaining shows every control the page does not show elsewhere.
func Remaining() Block { return Block{Type: BlockRemaining} }

// Designer is implemented by cameras that lay out their own page; the
// others get StandardPage. A module usually implements it in ui.go.
type Designer interface {
	Page() Page
}

// Limits of a page.
const (
	maxBlocks   = 64 // blocks in a page, grids included
	maxDepth    = 3  // grids inside grids
	maxColumns  = 4
	maxTitleLen = 64
)

// WidgetFits reports whether a widget can show a control of that kind.
func WidgetFits(widget, kind string, readOnly bool) bool {
	switch widget {
	case WidgetSlider, WidgetStepper:
		return kind == KindRange
	case WidgetButtons, WidgetSelect:
		return kind == KindEnum
	case WidgetSwitch:
		return kind == KindToggle
	case WidgetButton:
		return kind == KindAction
	case WidgetReadout:
		return readOnly && kind != KindAction
	case WidgetText:
		return kind == KindText
	}
	return false
}

// Check reports whether the page is well formed for these controls: known
// block types, existing controls with widgets that fit them, bounded size.
// The application runs the same checks when it loads a module.
func (p Page) Check(controls []ControlSpec) error {
	byID := make(map[string]ControlSpec, len(controls))
	for _, c := range controls {
		byID[c.ID] = c
	}
	n := 0
	var check func(b Block, depth int) error
	check = func(b Block, depth int) error {
		n++
		if n > maxBlocks {
			return fmt.Errorf("page: more than %d blocks", maxBlocks)
		}
		switch b.Type {
		case BlockIris, BlockLens, BlockGain, BlockShutter, BlockND, BlockWhiteBalance,
			BlockPresets, BlockRemaining, BlockExposure, BlockColor:
		case BlockImage:
			if b.Control != Contrast && b.Control != Saturation && b.Control != Hue {
				return fmt.Errorf("page: image block for %q, want contrast, saturation or hue", b.Control)
			}
		case BlockWheel:
			if b.Wheel != WheelLift && b.Wheel != WheelGamma && b.Wheel != WheelGain {
				return fmt.Errorf("page: wheel %q, want lift, gamma or gain", b.Wheel)
			}
		case BlockControls:
			if len(b.Title) > maxTitleLen || b.Columns < 0 || b.Columns > 2 {
				return fmt.Errorf("page: controls block %q: title of at most %d bytes, 1 or 2 columns", b.Title, maxTitleLen)
			}
			for _, it := range b.Items {
				c, ok := byID[it.Control]
				if !ok {
					return fmt.Errorf("page: block %q shows %q, which the camera does not declare", b.Title, it.Control)
				}
				if it.Widget != "" && !WidgetFits(it.Widget, c.Kind, c.ReadOnly) {
					return fmt.Errorf("page: widget %q cannot show %s, a %s control", it.Widget, c.ID, c.Kind)
				}
			}
		case BlockGrid:
			if depth >= maxDepth || b.Columns < 1 || b.Columns > maxColumns {
				return fmt.Errorf("page: grid of 1 to %d columns, nested at most %d deep", maxColumns, maxDepth)
			}
			for _, sub := range b.Blocks {
				if err := check(sub, depth+1); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("page: unknown block type %q", b.Type)
		}
		return nil
	}
	for _, r := range p.Rows {
		if len(r.Blocks) == 0 || len(r.Blocks) > maxColumns {
			return fmt.Errorf("page: a row holds 1 to %d blocks", maxColumns)
		}
		for _, b := range r.Blocks {
			if err := check(b, 0); err != nil {
				return err
			}
		}
	}
	return nil
}
