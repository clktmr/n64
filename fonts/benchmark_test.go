package fonts_test

import (
	"testing"

	"github.com/clktmr/n64/fonts/gomono12"
	n64testing "github.com/clktmr/n64/testing"
)

func TestMain(m *testing.M) { n64testing.TestMain(m) }

const lorem = `Lorem ipsum dolor sit amet, consectetur adipisici elit, sed
eiusmod tempor incidunt ut labore et dolore magna aliqua. Ut enim ad
minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquid
ex ea commodi consequat. Quis aute iure reprehenderit in voluptate velit
esse cillum dolore eu fugiat nulla pariatur. Excepteur sint obcaecat
cupiditat non proident, sunt in culpa qui officia deserunt mollit anim
id est laborum.`

func BenchmarkGlyphMap(b *testing.B) {
	gomono := gomono12.NewFace()

	i := 0
	for b.Loop() {
		gomono.GlyphMap(rune(lorem[i%len(lorem)]))
		i++
	}
}
