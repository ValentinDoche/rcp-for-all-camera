package core

import "fmt"

// Page is the layout of one camera's page, as its module describes it:
// rows of blocks, top to bottom; the blocks of a row share its width.
type Page struct {
	Rows []Row
}

// Row is blocks side by side.
type Row struct {
	Blocks []Block
}

// Block is one panel of a page. Type says which; the other fields
// configure the types that use them.
type Block struct {
	Type    BlockType
	Title   string    // BlockControls
	Control ControlID // BlockImage: Contrast, Saturation or Hue
	Wheel   Wheel     // BlockWheel
	Items   []Item    // BlockControls
	Columns int       // BlockControls (1 or 2), BlockGrid (1 to 4)
	Blocks  []Block   // BlockGrid
}

// Item is one control of a BlockControls panel and its widget ("" = the
// default widget of the control's kind).
type Item struct {
	Control ControlID
	Widget  Widget
}

// BlockType names a kind of block.
type BlockType string

const (
	BlockIris         BlockType = "iris"
	BlockLens         BlockType = "lens"
	BlockGain         BlockType = "gain"
	BlockShutter      BlockType = "shutter"
	BlockND           BlockType = "nd"
	BlockWhiteBalance BlockType = "whiteBalance"
	BlockImage        BlockType = "image"
	BlockWheel        BlockType = "wheel"
	BlockPresets      BlockType = "presets"
	BlockControls     BlockType = "controls"
	BlockGrid         BlockType = "grid"
	BlockRemaining    BlockType = "remaining"
	BlockExposure     BlockType = "exposure"
	BlockColor        BlockType = "color"
)

// Wheel names a color wheel.
type Wheel string

const (
	WheelLift  Wheel = "lift"
	WheelGamma Wheel = "gamma"
	WheelGain  Wheel = "gain"
)

// StandardPage is the generic page: the exposure and lens cards with the
// presets, then the color wheels with every other control.
func StandardPage() Page {
	return Page{Rows: []Row{{Blocks: []Block{{Type: BlockExposure}}}, {Blocks: []Block{{Type: BlockColor}}}}}
}

// PageReporter is implemented by cameras whose module lays out its page;
// the others get StandardPage.
type PageReporter interface {
	Page() Page
}

// Limits of a page.
const (
	maxPageBlocks  = 64
	maxGridDepth   = 3
	maxPageColumns = 4
)

// Check reports whether the page is well formed for these controls: known
// block types, declared controls with widgets that fit them, bounded size.
// It is the boundary check for a page a module sent.
func (p Page) Check(specs []ControlSpec) error {
	byID := make(map[ControlID]ControlSpec, len(specs))
	for _, s := range specs {
		byID[s.ID] = s
	}
	n := 0
	var check func(b Block, depth int) error
	check = func(b Block, depth int) error {
		if n++; n > maxPageBlocks {
			return fmt.Errorf("page: more than %d blocks", maxPageBlocks)
		}
		switch b.Type {
		case BlockIris, BlockLens, BlockGain, BlockShutter, BlockND, BlockWhiteBalance,
			BlockPresets, BlockRemaining, BlockExposure, BlockColor:
		case BlockImage:
			if b.Control != Contrast && b.Control != Saturation && b.Control != Hue {
				return fmt.Errorf("page: image block for an unknown control")
			}
		case BlockWheel:
			if b.Wheel != WheelLift && b.Wheel != WheelGamma && b.Wheel != WheelGain {
				return fmt.Errorf("page: unknown wheel")
			}
		case BlockControls:
			if len(b.Title) > MaxIDLen || !printable(b.Title) || b.Columns < 0 || b.Columns > 2 {
				return fmt.Errorf("page: a controls block has a printable title of at most %d bytes and 1 or 2 columns", MaxIDLen)
			}
			for _, it := range b.Items {
				s, ok := byID[it.Control]
				if !ok {
					return fmt.Errorf("page: block shows an undeclared control: %w", ErrUnknownControl)
				}
				if it.Widget != "" && !it.Widget.Fits(s.Kind, s.ReadOnly) {
					return fmt.Errorf("page: widget %q cannot show %s, a %s control", it.Widget, s.ID, s.Kind)
				}
			}
		case BlockGrid:
			if depth >= maxGridDepth || b.Columns < 1 || b.Columns > maxPageColumns {
				return fmt.Errorf("page: a grid has 1 to %d columns and nests at most %d deep", maxPageColumns, maxGridDepth)
			}
			for _, sub := range b.Blocks {
				if err := check(sub, depth+1); err != nil {
					return err
				}
			}
		default:
			return fmt.Errorf("page: unknown block type")
		}
		return nil
	}
	for _, r := range p.Rows {
		if len(r.Blocks) == 0 || len(r.Blocks) > maxPageColumns {
			return fmt.Errorf("page: a row holds 1 to %d blocks", maxPageColumns)
		}
		for _, b := range r.Blocks {
			if err := check(b, 0); err != nil {
				return err
			}
		}
	}
	return nil
}
