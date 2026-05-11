package scene

import (
	"embedded/rtos"
	"image"
	"image/color"
	"image/draw"
	"io"
	"slices"
	"time"

	"github.com/clktmr/n64/drivers/controller"
	"github.com/clktmr/n64/drivers/display"
	n64draw "github.com/clktmr/n64/drivers/draw"
	"github.com/clktmr/n64/drivers/rspq/mixer"
	"github.com/clktmr/n64/engine/tree"
	"github.com/clktmr/n64/rcp/audio"
)

type Updater interface {
	Update(delta time.Duration, input *[4]controller.Controller)
}

type Renderer interface {
	Render(dst draw.Image, r image.Rectangle, sp image.Point)
}

type GameLoop struct {
	root    Node
	display *display.Display
}

func NewGameLoop(disp *display.Display, root Node) *GameLoop {
	return &GameLoop{root, disp}
}

var ClearColor = color.RGBA{0xb9, 0xff, 0xfd, 0xff}

func (p *GameLoop) Run() {
	// input
	gamepad := make(chan [4]controller.Controller)
	last := rtos.Nanotime()
	go func() {
		inputs := [4]controller.Controller{}
		for {
			controller.Poll(&inputs)
			gamepad <- inputs
		}
	}()

	// audio
	audio.Start(48000)
	mixer.Init()
	go io.Copy(audio.Buffer, mixer.Output)

	clearImg := image.Uniform{ClearColor}
	renderers := make([]uint32, 0, 64)
	xformStack := make([]image.Point, 0, 16)
	scissorStack := make([]image.Rectangle, 0, 16)
	renderParams := make([]struct {
		r  image.Rectangle
		sp image.Point
	}, 0, 16)

	var input [4]controller.Controller
	for {
		// finish current frame
		write := p.display.Swap()

		// setup next frame
		now := rtos.Nanotime()
		now -= now % p.display.RefreshInterval()
		delta := now - last
		input = <-gamepad

		n64draw.Src.Draw(write, write.Bounds(), &clearImg, image.Point{})

		// TODO list of updaters should only be updated on Add/Remove
		for _, node := range scenetree.Preorder(tree.Root) {
			scenenode := Node(node)
			if updater, ok := scenenode.Value().(Updater); ok {
				updater.Update(delta, &input)
			}
		}
		last = now

		// Check if we need to grow renderParams array
		if len(impl) > cap(renderParams) {
			renderParams = make([]struct {
				r  image.Rectangle
				sp image.Point
			}, len(impl))
		} else {
			renderParams = renderParams[:len(impl)]
		}

		// Traverse the scene and calculate renderParams for each node
		lastDepth := -1
		xform := image.Pt(0, 0)
		scissor := write.Bounds()
		xformStack = xformStack[:0]
		scissorStack = scissorStack[:0]
		for depth, node := range Zero.Preorder() {
			if lastDepth < depth {
				xformStack = append(xformStack, xform)
				scissorStack = append(scissorStack, scissor)
				lastDepth++
			}
			for lastDepth > depth {
				xformStack = xformStack[:len(xformStack)-1]
				scissorStack = scissorStack[:len(scissorStack)-1]
				lastDepth--
			}
			parentXform := xformStack[len(xformStack)-1]
			parentScissor := scissorStack[len(scissorStack)-1]
			xform = parentXform.Add(node.Transform())
			if node.IsScissor() {
				scissor = parentScissor.Intersect(node.Bounds().Add(xform))
			} else {
				scissor = parentScissor
			}

			r := node.Bounds().Add(xform)
			renderParams[node].r = r.Intersect(scissor)
			renderParams[node].sp = renderParams[node].r.Min.Sub(r.Min)
		}

		// TODO Check if using a SortFunc is faster
		renderers = renderers[:0]
		for node, z := range z {
			renderers = append(renderers, uint32(z)<<16|uint32(node))
		}
		slices.Sort(renderers) // TODO radix sort

		for _, node := range renderers {
			// TODO Perform occlusion check
			scenenode := Node(node & 0xffff)
			r := renderParams[scenenode].r
			if r.Empty() {
				continue
			}
			sp := renderParams[scenenode].sp
			if renderer := scenenode.Value(); renderer != nil {
				renderer.Render(write, r, sp)
			}
		}

		n64draw.Flush()
	}
}
