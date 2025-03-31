package console

import (
	"github.com/clktmr/n64/drivers/display"
)

// Gamelooper represents a game instance that can be updated and drawn.
type Gamelooper interface {
	// Update is called every frame to update game logic.
	// Return an error to exit the game loop, nil to continue.
	Update() error

	// Draw is called every frame to render the game.
	// The screen is already initialized and ready for drawing.
	Draw(screen *display.Screen)
}

// Run starts the game loop with default video settings (NTSC 320x240, no interlacing).
// It will initialize the display, then repeatedly call Update() and Draw().
func Run(g Gamelooper) error {
	// Initialize display with default settings
	qualityPreset := display.LowRes // TODO: make this configurable
	screen := display.Init(qualityPreset)

	// Main game loop
	for {
		// Update game logic
		if err := g.Update(); err != nil {
			return err
		}

		// Draw game
		screen.BeginDrawing()
		g.Draw(screen)
		screen.EndDrawing()
	}
}
