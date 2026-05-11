package scene

import (
	"image"
	"testing"

	n64testing "github.com/clktmr/n64/testing"
)

func TestMain(m *testing.M) { n64testing.TestMain(m) }

func TestSceneNode(t *testing.T) {
	player1 := Node(0).AddChild("player1")
	player1.SetTransform(image.Pt(2, 0))
	player1.AddChild("gogo")
	gaga := player1.AddChild("gaga")
	gaga.SetTransform(image.Pt(1, 3))
	player1.AddChild("gigi")

	pipi := gaga.AddChild("pipi")
	pipi.SetTransform(image.Pt(10, 10))
	pipi.AddChild("papa")

	transforms := make([]image.Point, 0, 32)
	lastDepth := -1
	current := image.Point{}
	for depth, node := range Zero.Preorder() {
		if lastDepth < depth {
			transforms = append(transforms, current)
			lastDepth++
		}
		for lastDepth > depth {
			transforms = transforms[:len(transforms)-1]
			lastDepth--
		}
		parent := transforms[len(transforms)-1]
		current = parent.Add(node.Transform())
		transform[node] = current
	}

	for depth, child := range player1.Preorder() {
		t.Log(depth, transform[child], child.Value())
	}
}
