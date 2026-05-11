package main

import (
	"image"
	"image/color"
	idraw "image/draw"
	"time"

	"github.com/clktmr/n64/drivers/controller"
	"github.com/clktmr/n64/drivers/display"
	"github.com/clktmr/n64/drivers/draw"
	"github.com/clktmr/n64/engine/scene"
	"github.com/clktmr/n64/engine/ui"
	"github.com/clktmr/n64/fonts/goregular12"
	"github.com/clktmr/n64/rcp/video"

	_ "github.com/clktmr/n64/machine"
)

const lorem = "abcdefghijklmnopqrstuvwxyz Lorem ipsum dolor sit amet, consectetur adipisici elit, sed eiusmod tempor incidunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquid ex ea commodi consequat. Quis aute iure reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur. Excepteur sint obcaecat cupiditat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum."

var goregular = goregular12.NewFace()

type Player struct {
	scene.Node
}

var canvas *ui.Canvas

func (p *Player) Update(delta time.Duration, input *[4]controller.Controller) {
	if delta == 0 {
		return
	}
	v := image.Pt(int(input[0].X()), int(-input[0].Y()))
	v = v.Div(int(4 * delta / time.Millisecond))
	t := canvas.Transform().Add(v)
	canvas.SetTransform(t)
}
func (c Player) Render(dst idraw.Image, r image.Rectangle, sp image.Point) {}

func main() {
	video.Setup(false)
	display := display.NewDisplay(image.Pt(320, 240), video.BPP16)
	white := color.NRGBA{0xff, 0xff, 0xff, 0xff}
	black := color.NRGBA{0x0, 0x0, 0x0, 0xff}
	player := scene.Zero.AddChild(&Player{})
	canvas = ui.NewCanvas(player)
	canvas.SetTransform(image.Pt(10, 10))

	newtext := func() *draw.TextImage { return draw.NewTextImage(goregular, black, white) }

	flexbox3 := ui.NewFlexBox(canvas.Node)
	flexbox3.Background = &image.Uniform{color.NRGBA{0x55, 0x55, 0x32, 0xff}}
	flexbox3.Justify = ui.JustifyStart
	flexbox3.Align = ui.AlignStretch

	flexbox5 := ui.NewFlexBox(flexbox3.Node)
	flexbox5.Direction = ui.DirectionColumn
	flexbox5.Justify = ui.JustifyStart
	flexbox5.Align = ui.AlignStart
	flexbox5.Padding = ui.Clearance{1, 1, 1, 1}
	flexbox5.Margin = ui.Clearance{1, 1, 1, 1}
	flexbox5.Background = draw.NewBorderImage(0, 0, color.NRGBA{A: 0xff}, 1, 1, 1, 1)
	loremp := ui.NewUIParagraph(flexbox5.Node, newtext(), lorem)
	loremp.Clip = true

	flexbox0 := ui.NewFlexBox(flexbox3.Node)
	flexbox0.Direction = ui.DirectionRow
	flexbox0.Justify = ui.JustifyCenter
	flexbox0.Align = ui.AlignStretch
	flexbox0.Background = &image.Uniform{color.NRGBA{0xff, 0x0, 0x32, 0xff}}

	flexbox1 := ui.NewFlexBox(flexbox0.Node)
	flexbox1.Background = draw.NewBorderImage(0, 0, color.NRGBA{A: 0xff}, 1, 1, 1, 1)
	ui.NewUIParagraph(flexbox1.Node, newtext(), "hello")
	ui.NewUIParagraph(flexbox1.Node, newtext(), "everybody")
	ui.NewUIParagraph(flexbox1.Node, newtext(), "BONZO")
	flexbox1.Padding = ui.Clearance{1, 1, 1, 1}
	flexbox1.Margin = ui.Clearance{1, 1, 1, 1}
	flexbox1.Justify = ui.JustifyStart
	flexbox1.Align = ui.AlignStart

	flexbox2 := ui.NewFlexBox(flexbox0.Node)
	flexbox2.Background = draw.NewBorderImage(0, 0, color.NRGBA{A: 0xff}, 1, 1, 1, 1)
	ui.NewUIParagraph(flexbox2.Node, newtext(), "shit")
	ui.NewUIParagraph(flexbox2.Node, newtext(), "the")
	ui.NewUIParagraph(flexbox2.Node, newtext(), "is")
	ui.NewUIParagraph(flexbox2.Node, newtext(), "this")
	flexbox2.Padding = ui.Clearance{1, 1, 1, 1}
	flexbox2.Margin = ui.Clearance{1, 1, 1, 1}
	flexbox2.Justify = ui.JustifyEnd
	flexbox2.Align = ui.AlignCenter

	flexbox4 := ui.NewFlexBox(flexbox3.Node)
	flexbox4.Direction = ui.DirectionRow
	flexbox4.Justify = ui.JustifyAround
	flexbox4.Align = ui.AlignStart
	ui.NewUIParagraph(flexbox4.Node, newtext(), "0")
	ui.NewUIParagraph(flexbox4.Node, newtext(), "1")
	ui.NewUIParagraph(flexbox4.Node, newtext(), "2")
	flexbox4.Padding = ui.Clearance{1, 1, 1, 1}
	flexbox4.Margin = ui.Clearance{1, 1, 1, 1}
	flexbox4.Background = draw.NewBorderImage(0, 0, color.NRGBA{A: 0xff}, 1, 1, 1, 1)

	canvas.Resize(image.Rect(0, 0, 300, 220))
	// canvas.Autosize(100, true)
	// flexbox0.SetTransform(image.Pt(50, 0))
	// root.Autosize(0)

	scene.ClearColor = color.RGBA{0xff, 0xff, 0xff, 0xff}
	gameloop := scene.NewGameLoop(display, canvas.Node.Node)

	gameloop.Run()
	println("exit")
}
