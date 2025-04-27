package cartfs_test

import (
	"embed"
	"testing"

	"github.com/clktmr/n64/drivers/cartfs"
)

//go:embed concurrency.txt
var _embed1 embed.FS

var (
	//go:embed testdata/ken.txt
	_embed2        embed.FS
	embed1, embed2 cartfs.FS = cartfs.Embed(_embed1), cartfs.Embed(_embed2)
)

//go:embed testdata/hello.txt
var _embed3 embed.FS

var embed3 cartfs.FS = cartfs.Embed(_embed3)

var _nocomment embed.FS
var nocomment cartfs.FS = cartfs.Embed(_nocomment)

// TestMkrom checks if the different declaration styles for variables are
// correctly parsed by the mkrom tool.
func TestEmbed(t *testing.T) {
	testFiles(t, embed1, "concurrency.txt", "Concurrency is not parallelism.\n")
	testFiles(t, embed3, "testdata/hello.txt", "hello, world\n")
	testFiles(t, embed2, "testdata/ken.txt", "If a program is too slow, it must have a loop.\n")
	testDir(t, nocomment, ".")
}
