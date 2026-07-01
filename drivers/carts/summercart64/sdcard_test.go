package summercart64_test

import (
	"io/fs"
	"testing"

	"github.com/clktmr/fat32"
	"github.com/clktmr/fat32/mbr"
)

func TestSDCard(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}
	sc64 := mustSC64(t)
	sd := sc64.SDCard()
	status, err := sd.Status()
	if err != nil {
		t.Fatal(err)
	}
	if !status.Inserted() {
		t.Skip("needs sdcard inserted")
	}

	err = sd.Init()
	if err != nil {
		t.Fatal(err)
	}

	const blockSize = 512
	table, err := mbr.Read(sd, blockSize, blockSize)
	if err != nil {
		t.Fatal(err)
	}
	part := table.GetPartitions()[0]
	sdfs, err := fat32.Read(sd, part.GetSize(), part.GetStart(), blockSize)
	if err != nil {
		t.Fatal(err)
	}
	files, err := sdfs.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range files {
		t.Log(fs.FormatDirEntry(v))
	}
}
