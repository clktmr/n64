package summercart64_test

import (
	"io/fs"
	"testing"

	"github.com/clktmr/n64/drivers/carts/summercart64"
	"github.com/diskfs/go-diskfs/filesystem/fat32"
	"github.com/diskfs/go-diskfs/partition/mbr"
)

type blockdev struct{ *summercart64.SDCard }

// diskfs wants this implemented, although fat32 doesn't use it
func (d *blockdev) Seek(offset int64, whence int) (int64, error) { panic("not supported") }

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
	disk := &blockdev{sd}
	table, err := mbr.Read(disk, blockSize, blockSize)
	if err != nil {
		t.Fatal(err)
	}
	part := table.GetPartitions()[0]
	sdfs, err := fat32.Read(disk, part.GetSize(), part.GetStart(), blockSize)
	if err != nil {
		t.Fatal(err)
	}
	files, err := sdfs.ReadDir("/")
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range files {
		t.Log(fs.FormatFileInfo(v))
	}
}
