package main

import (
	"image/color"

	"golang.org/x/image/colornames"

	"github.com/clktmr/n64/console"
	"github.com/clktmr/n64/drivers/display"
)

// Game implements the console.Gamelooper interface
type Game struct {
	// Add your game state here
	frameCount  int
	screencolor color.RGBA
}

// Update your game logic here
func (g *Game) Update() error {
	g.frameCount++

	switch (g.frameCount / 60) % 3 {
	case 0:
		g.screencolor = colornames.Red
	case 1:
		g.screencolor = colornames.Blue
	case 2:
		g.screencolor = colornames.Green
	}

	return nil
}

// Draw your game here
func (g *Game) Draw(screen *display.Screen) {
	screen.ClearBackground(g.screencolor)
}

func main() {
	// Initialize the game
	game := &Game{}

	// Run the game in the console
	if err := console.Run(game); err != nil {
		panic(err)
	}
}
