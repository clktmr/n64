package display

import (
	"image"
	"image/color"
	"image/draw"

	n64draw "github.com/clktmr/n64/drivers/draw"
	"github.com/clktmr/n64/machine"
	"github.com/clktmr/n64/rcp/video"
)

// VideoPreset represents a predefined video configuration
type VideoPreset int

const (
	// LowRes is the most common setup: 320x240 without interlacing
	LowRes VideoPreset = iota
	// HighRes is 640x480 with interlacing
	HighRes
)

// Screen represents the display surface that can be drawn to.
type Screen struct {
	renderer *n64draw.Rdp
}

// BeginDrawing prepares for a new frame by swapping the framebuffer.
func (s *Screen) BeginDrawing() {
	fb := currentScreen.Display.Swap()
	s.renderer.SetFramebuffer(fb)
}

// EndDrawing finalizes the frame by flushing the renderer.
func (s *Screen) EndDrawing() {
	s.renderer.Flush()
}

// ClearBackground clears the screen with the specified color.
func (s *Screen) ClearBackground(c color.Color) {
	s.renderer.Draw(s.renderer.Bounds(), &image.Uniform{c}, image.Point{}, draw.Src)
}

// screen holds all the display-related objects and state.
type screen struct {
	Display  *Display
	Renderer *n64draw.Rdp
}

var currentScreen *screen

// getPresetConfig returns the configuration for a given preset
func getPresetConfig(preset VideoPreset) (resolution image.Point, colorDepth video.ColorDepth, mode machine.VideoType, interlaced bool) {
	switch preset {
	case LowRes:
		return image.Point{X: 320, Y: 240}, video.ColorDepth(video.BPP16), machine.VideoNTSC, false
	case HighRes:
		return image.Point{X: 640, Y: 480}, video.ColorDepth(video.BPP32), machine.VideoNTSC, true
	default:
		return image.Point{X: 320, Y: 240}, video.ColorDepth(video.BPP16), machine.VideoNTSC, false
	}
}

// Init initializes display with the specified video preset.
// It sets up the framebuffer and renderer for drawing.
func Init(preset VideoPreset) *Screen {
	resolution, colorDepth, mode, interlaced := getPresetConfig(preset)

	// Set the video mode and setup
	machine.Video = mode
	video.Setup(interlaced)

	// Create display and renderer
	disp := NewDisplay(resolution, colorDepth)
	renderer := n64draw.NewRdp()
	renderer.SetFramebuffer(disp.Swap())

	currentScreen = &screen{
		Display:  disp,
		Renderer: renderer,
	}

	return &Screen{
		renderer: renderer,
	}
}
