# N64 Basic Game Example

A simple example showing how to create a game using the N64 console package. The screen cycles through red, green, and blue colors every 60 frames.

## Code Structure

```go
// Game implements the console.Gamelooper interface
type Game struct {}

// Update is called every frame to update game logic
func (g *Game) Update() error {
    return nil
}

// Draw is called every frame to render the game
func (g *Game) Draw(screen *display.Screen) {}

func main() {
    game := &Game{}
    if err := console.Run(game); err != nil {
        log.Fatal(err)
    }
}
```

## How to Run

```bash
emgo build
```

## Key Points

1. The `console.Gamelooper` interface requires two methods:
   - `Update()`: Called every frame for game logic
   - `Draw(screen *display.Screen)`: Called every frame for rendering

2. The `console.Run()` function handles:
   - Display initialization using a default video preset (NTSC@240p)
   - Game loop

3. Colors are provided by standard library `golang.org/x/image/colornames`:

   ```go
   colornames.Red
   colornames.Blue
   colornames.Green
   ```
