package pakfs

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"testing"
)

func prepareRead(t *testing.T, filename string, flipBytes []int) io.ReaderAt {
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal("missing testdata:", err)
	}
	for _, v := range flipBytes {
		data[v] = ^data[v]
	}
	return bytes.NewReader(data)
}

func TestRead(t *testing.T) {
	filename := path.Join("testdata", "clktmr.mpk")
	tests := map[string]struct {
		data io.ReaderAt
		err  error
	}{
		"/dev/null":       {bytes.NewReader(make([]byte, 256*10)), ErrInconsistent},
		"valid":           {prepareRead(t, filename, []int{}), nil},
		"damageId":        {prepareRead(t, filename, []int{0x20}), nil},
		"damageIdBak1":    {prepareRead(t, filename, []int{0x20, 0x60}), nil},
		"damageIdBak12":   {prepareRead(t, filename, []int{0x20, 0x60, 0x80}), nil},
		"damageIdAll":     {prepareRead(t, filename, []int{0x20, 0x60, 0x80, 0xc0}), ErrInconsistent},
		"damageInodes":    {prepareRead(t, filename, []int{0x1ff}), nil},
		"damageInodesBak": {prepareRead(t, filename, []int{0x1ff, 0x2ff}), ErrInconsistent},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Read(tc.data)
			if err != tc.err {
				t.Fatalf("expected %v, got %v", tc.err, err)
			}
		})
	}
}

func TestReadFile(t *testing.T) {
	// The following testcases were defined with the help of MPKEdit
	tests := map[string]struct {
		name string
		size int64
		sha1 string
		err  error
	}{
		"PerfectDark1": {"PERFECT ", 7168, "\x84\xc2\x88\x64\x69\xed\xab\xd5\x1b\x4d\xc0\x7d\x2b\xbe\x67\x86\xd4\x47\xc1\xd2", nil},
		"PerfectDark2": {"PERFECT DARK", 7168, "\x01\x35\x24\x57\x45\x74\xf7\xb7\xe9\x1f\xfa\xda\x2e\xfb\x44\xe5\x74\x36\x55\x73", nil},
		"Vigilante82":  {"V82, \"METIN\"", 256, "\x86\x99\x89\x88\x78\x19\x3d\x84\xb3\x2f\x8b\x49\x40\xb6\x22\x6b\x57\x28\x25\xdf", nil},
	}

	data, err := os.ReadFile(path.Join("testdata", "clktmr.mpk"))
	if err != nil {
		t.Fatal("missing testdata:", err)
	}
	fs, err := Read(bytes.NewReader(data))
	if err != nil {
		t.Fatal("damaged testdata:", err)
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			file, err := fs.Open(tc.name)
			if err != tc.err {
				t.Fatalf("expected %v, got %v", tc.err, err)
			}
			if tc.err != nil {
				return
			}
			stat, err := file.Stat()
			if err != nil {
				t.Fatal("stat:", err)
			}
			if stat.Size() != tc.size {
				t.Fatalf("expected %v, got %v", tc.size, stat.Size())
			}
			filedata, err := io.ReadAll(file)
			if err != nil {
				t.Fatal("read:", err)
			}
			hash := sha1.New()
			if _, err := io.Copy(hash, bytes.NewReader(filedata)); err != nil {
				t.Fatal("io.Copy:", err)
			}
			hashsum := hash.Sum([]byte{})
			if !bytes.Equal(hashsum, []byte(tc.sha1)) {
				t.Fatal("hash mismatch")
			}
		})
	}
}

func writeableTestdata(t *testing.T, name string) *os.File {
	data, err := os.ReadFile(path.Join("testdata", "clktmr.mpk"))
	if err != nil {
		t.Fatal("missing testdata:", err)
	}

	tempTestdata := path.Join(t.TempDir(), "clktmr.mpk")
	err = os.WriteFile(tempTestdata, data, 0666)
	if err != nil {
		t.Fatal("copying testdata:", err)
	}

	file, err := os.OpenFile(tempTestdata, os.O_RDWR, 0777)
	if err != nil {
		t.Fatal("open testdata:", err)
	}
	return file
}

func TestCreateFile(t *testing.T) {
	tests := map[string]struct {
		name string
		err  error
	}{
		"ErrExist1":          {"PERFECT ", fs.ErrExist},
		"ErrExist2":          {"PERFECT DARK", fs.ErrExist},
		"ErrExist3":          {"V82, \"METIN\"", fs.ErrExist},
		"Simple":             {"SIMPLE.TXT", nil},
		"NoExtension":        {"NOEXT", nil},
		"NoExtension2":       {"NOEXT2.", nil},
		"OnlyExtension":      {".EXT", nil},
		"DotInName":          {"DOT.IN.NAME", nil},
		"NoNullTerm":         {"NONULLTERMINATOR", nil},
		"NoNullTermExt":      {"NO.NULL", nil},
		"ErrNameTooLongName": {"VERYLONGFILENAME!", ErrNameTooLong},
		"ErrNameTooLongExt":  {"NAME.EXTEN", ErrNameTooLong},
		"ErrNotExist":        {"ISDIR/FILE", fs.ErrNotExist},
	}

	testdata := writeableTestdata(t, "clktmr.mpk")

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			pfs, err := Read(testdata)
			if err != nil {
				t.Fatal("damaged testdata:", err)
			}
			freeBefore := pfs.Free()
			numFiles := len(pfs.Root())

			f, err := pfs.Create(tc.name)
			if !errors.Is(err, tc.err) {
				t.Fatalf("expected %v, got %v", tc.err, err)
			}

			if err == nil {
				numFiles += 1
			}
			if len(pfs.Root()) != numFiles {
				t.Fatalf("expected %v files, got %v", numFiles, len(pfs.Root()))
			}
			if pfs.Free() != freeBefore {
				t.Fatalf("free disk space changed")
			}

			if err != nil {
				return
			}
			if f.Name() != tc.name {
				t.Fatalf("expected filename '%v', got '%v'", tc.name, f.Name())
			}

		})
	}
}

func TestOpenFile(t *testing.T) {
	tests := map[string]struct {
		name string
		err  error
	}{
		"Root":         {".", nil},
		"Ok1":          {"PERFECT ", nil},
		"Ok2":          {"PERFECT DARK", nil},
		"Ok3":          {"V82, \"METIN\"", nil},
		"ErrNotExist1": {"PERFECT", os.ErrNotExist},
		"ErrNotExist2": {"PERFECT  ", os.ErrNotExist},
		"ErrNotExist3": {"perfect ", os.ErrNotExist},
		"ErrNotExist4": {"PERFECT .", os.ErrNotExist},
		"ErrInvalid1":  {"", fs.ErrInvalid},
		"ErrInvalid2":  {"./PERFECT ", fs.ErrInvalid},
		"ErrInvalid3":  {"/PERFECT ", fs.ErrInvalid},
	}

	testdata := writeableTestdata(t, "clktmr.mpk")
	pfs, err := Read(testdata)
	if err != nil {
		t.Fatal("damaged testdata:", err)
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := pfs.Open(tc.name)
			if !errors.Is(err, tc.err) {
				t.Fatalf("expected %v, got %v", tc.err, err)
			}
		})
	}
}

func TestRemoveFile(t *testing.T) {
	tests := map[string]struct {
		name string
		size int64
		err  error
	}{
		"Root":         {".", 0, ErrIsDir},
		"ErrNotExist1": {"NOTEXIST", 0, os.ErrNotExist},
		"Ok1":          {"PERFECT ", 7168, nil},
		"Ok2":          {"PERFECT DARK", 7168, nil},
		"Ok3":          {"V82, \"METIN\"", 256, nil},
	}

	testdata := writeableTestdata(t, "clktmr.mpk")

	var pfs *FS
	var free int64
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			var err error
			pfs, err = Read(testdata)
			if err != nil {
				t.Fatal("damaged testdata:", err)
			}
			numFiles := len(pfs.Root())
			free = pfs.Free()

			err = pfs.Remove(tc.name)
			if !errors.Is(err, tc.err) {
				t.Fatalf("expected %v, got %v", tc.err, err)
			}
			if err == nil {
				numFiles -= 1
				free += tc.size
			}
			if len(pfs.Root()) != numFiles {
				t.Fatalf("expected %v files, got %v", numFiles, len(pfs.Root()))
			}
			if pfs.Free() != free {
				t.Fatalf("expected %v free bytes, got %v", free, pfs.Free())
			}
		})
	}

	size := pfs.Size()
	if size != free {
		t.Fatalf("expected empty filesystem, got free=%v size=%v", free, size)
	}
}

func TestTruncateFile(t *testing.T) {
	tests := map[string]struct {
		name string
		size int64
		err  error
	}{
		"Root":        {".", 0, ErrIsDir},
		"ErrNotExist": {"NOTEXIST", 0, os.ErrNotExist},
		"ErrInvalid1": {"NOTEXIST", -1, fs.ErrInvalid},
		"ErrInvalid2": {"PERFECT ", -1, fs.ErrInvalid},
		"ErrNoSpace":  {"PERFECT ", 7168 + 16986 + 512, ErrNoSpace},
		"Noop1":       {"PERFECT ", 7168, nil},
		"Noop2":       {"PERFECT ", 7167, nil},
		"Noop3":       {"PERFECT ", 6913, nil},
		"Grow":        {"V82, \"METIN\"", 257, nil},
		"Shrink":      {"PERFECT DARK", 6913, nil},
	}

	testdata := writeableTestdata(t, "clktmr.mpk")

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			pfs, err := Read(testdata)
			if err != nil {
				t.Fatal("damaged testdata:", err)
			}
			free := pfs.Free()

			fi, _ := pfs.Open(tc.name)
			f, _ := fi.(*File)
			var oldSize int64

			if tc.err == nil {
				oldSize = f.Size()
			}

			err = pfs.Truncate(tc.name, tc.size)
			if !errors.Is(err, tc.err) {
				t.Fatalf("expected %v, got %v", tc.err, err)
			}

			if err == nil {
				expectedSize := (tc.size + pageMask) &^ pageMask
				if f.Size() != expectedSize {
					t.Fatalf("expected size %v, got %v", expectedSize, f.Size())
				}
				delta := f.Size() - oldSize
				free -= delta

				// Check if new bytes are zeroed
				if delta > 0 {
					buf := make([]byte, delta)
					zeroes := make([]byte, delta)
					_, err := f.ReadAt(buf, f.Size()-delta)
					if err != nil && err != io.EOF {
						t.Fatal(err)
					}
					if !bytes.Equal(buf, zeroes) {
						t.Fatal("new pages contain data")
					}
				}
			}
			if pfs.Free() != free {
				t.Fatalf("expected %v free bytes, got %v", free, pfs.Free())
			}
		})
	}
}
