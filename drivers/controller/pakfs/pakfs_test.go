package pakfs

// TODO add tests for file system with multiple banks

import (
	"bytes"
	"crypto/sha1"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
	"testing"
	"time"

	"math/rand"
)

const lorem = `Lorem ipsum dolor sit amet, consectetur adipisici elit, sed
eiusmod tempor incidunt ut labore et dolore magna aliqua. Ut enim ad
minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquid
ex ea commodi consequat. Quis aute iure reprehenderit in voluptate velit
esse cillum dolore eu fugiat nulla pariatur. Excepteur sint obcaecat
cupiditat non proident, sunt in culpa qui officia deserunt mollit anim
id est laborum.`

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

func TestReadDir(t *testing.T) {
	tests := map[string][]struct {
		n   int
		err error
	}{
		"Full":      {{0, nil}},
		"Single":    {{1, nil}, {1, nil}, {1, io.EOF}},
		"Multi":     {{2, nil}, {1, io.EOF}},
		"Exceed":    {{2, nil}, {2, io.EOF}},
		"Mixed1":    {{2, nil}, {0, nil}},
		"Mixed2":    {{0, nil}, {2, io.EOF}},
		"FullMulti": {{0, nil}, {0, nil}},
	}

	data, err := os.ReadFile(path.Join("testdata", "clktmr.mpk"))
	if err != nil {
		t.Fatal("missing testdata:", err)
	}
	pfs, err := Read(bytes.NewReader(data))
	if err != nil {
		t.Fatal("damaged testdata:", err)
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			fi, _ := pfs.Open(".")
			dir, _ := fi.(*rootDir)
			for i, call := range tc {
				entries, err := dir.ReadDir(call.n)
				if err != call.err {
					t.Fatalf("expected %v, got %v (i=%v)", call.err, err, i)
				}
				if err == nil && call.n > 0 {
					if len(entries) != call.n {
						t.Fatalf("expected %d entries, got %d (i=%v)", call.n, len(entries), i)
					}
				}
			}
		})
	}
}

func TestReadFile(t *testing.T) {
	// The following testcases were defined with the help of MPKEdit
	tests := map[string]struct {
		name, gamecode, companycode string

		size int64
		sha1 string
		err  error
	}{
		"PerfectDark1": {"PERFECT ", "NPDP", "4Y", 7168, "\x84\xc2\x88\x64\x69\xed\xab\xd5\x1b\x4d\xc0\x7d\x2b\xbe\x67\x86\xd4\x47\xc1\xd2", nil},
		"PerfectDark2": {"PERFECT DARK", "NPDP", "4Y", 7168, "\x01\x35\x24\x57\x45\x74\xf7\xb7\xe9\x1f\xfa\xda\x2e\xfb\x44\xe5\x74\x36\x55\x73", nil},
		"Vigilante82":  {"V82, \"METIN\"", "NVGP", "52", 256, "\x86\x99\x89\x88\x78\x19\x3d\x84\xb3\x2f\x8b\x49\x40\xb6\x22\x6b\x57\x28\x25\xdf", nil},
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

			fi, _ := file.(*File)
			gamecode := fi.GameCode()
			if string(gamecode[:]) != tc.gamecode {
				t.Fatal("gamecode code", string(gamecode[:]))
			}
			companycode := fi.CompanyCode()
			if string(companycode[:]) != tc.companycode {
				t.Fatal("company code", string(companycode[:]))
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

func TestWriteFile(t *testing.T) {
	tests := map[string]struct {
		name   string
		data   []byte
		offset int64
		err    error
	}{
		"Noop":       {"PERFECT ", []byte(""), 0, nil},
		"NoopEOF":    {"PERFECT ", []byte(""), 9999, nil},
		"Short1":     {"PERFECT ", []byte("foo"), 0, nil},
		"Short2":     {"PERFECT ", []byte("foo"), 256, nil},
		"Short3":     {"PERFECT ", []byte("foo"), 600, nil},
		"Short4":     {"PERFECT ", []byte("foo"), 7168, nil},
		"ShortEOF":   {"PERFECT ", []byte("foo"), 7421, nil},
		"Long1":      {"PERFECT DARK", []byte(lorem), 100, nil},
		"Long2":      {"PERFECT DARK", []byte(lorem), 7068, nil},
		"LongEOF":    {"V82, \"METIN\"", []byte(lorem), 300, nil},
		"ErrNoSpace": {"V82, \"METIN\"", []byte(lorem), 1000000, ErrNoSpace},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			pfs, err := Read(writeableTestdata(t, "clktmr.mpk"))
			if err != nil {
				t.Fatal("damaged testdata:", err)
			}

			fi, err := pfs.Open(tc.name)
			if err != nil {
				t.Fatal(err)
			}
			f, _ := fi.(*File)
			oldSize := f.Size()

			n, err := f.WriteAt(tc.data, tc.offset)
			if !errors.Is(err, tc.err) {
				t.Fatalf("expected %v, got %v", tc.err, err)
			}
			if err != nil {
				return
			}
			if n != len(tc.data) {
				t.Fatalf("expected %v written, got %v", len(tc.data), n)
			}
			if len(tc.data) > 0 {
				expectedSize := max(oldSize, tc.offset+int64(len(tc.data)))
				expectedSize = (expectedSize + pageMask) &^ pageMask
				if f.Size() != expectedSize {
					t.Fatalf("filesize: exptected %v, got %v", expectedSize, f.Size())
				}
			} else {
				if oldSize != f.Size() {
					t.Fatalf("filesize changed on empty write: old %v new %v", oldSize, f.Size())
				}
			}
			buf := make([]byte, n)
			_, err = f.ReadAt(buf, tc.offset)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(buf, tc.data) {
				t.Fatalf("read unexpected data:\nexpected %q\ngot %q", tc.data, buf)
			}

			// Newly allocated bytes not written must be zeroed
			end := tc.offset + int64(len(tc.data))
			for _, gap := range [...]struct{ size, offset int64 }{
				{tc.offset - oldSize, oldSize},
				{pageSize - end&pageMask, end},
			} {
				if gap.size > 0 && gap.offset > oldSize {
					buf := make([]byte, gap.size)
					zeroes := make([]byte, gap.size)
					_, err = f.ReadAt(buf, gap.offset)
					if err != nil && err != io.EOF {
						t.Fatal(err)
					}
					if !bytes.Equal(buf, zeroes) {
						t.Errorf("gap not zeroed: %v\n%q", gap, buf)
					}
				}
			}
		})
	}
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
			numFiles := len(pfs.ReadDirRoot())

			f, err := pfs.Create(tc.name)
			if !errors.Is(err, tc.err) {
				t.Fatalf("expected %v, got %v", tc.err, err)
			}

			if err == nil {
				numFiles += 1
			}
			if len(pfs.ReadDirRoot()) != numFiles {
				t.Fatalf("expected %v files, got %v", numFiles, len(pfs.ReadDirRoot()))
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
			numFiles := len(pfs.ReadDirRoot())
			free = pfs.Free()

			err = pfs.Remove(tc.name)
			if !errors.Is(err, tc.err) {
				t.Fatalf("expected %v, got %v", tc.err, err)
			}
			if err == nil {
				numFiles -= 1
				free += tc.size
			}
			if len(pfs.ReadDirRoot()) != numFiles {
				t.Fatalf("expected %v files, got %v", numFiles, len(pfs.ReadDirRoot()))
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
		"Noop":        {"PERFECT ", 7168, nil},
		"Clear1":      {"PERFECT ", 7167, nil},
		"Clear2":      {"PERFECT ", 6913, nil},
		"Zero":        {"PERFECT ", 0, nil},
		"Grow":        {"V82, \"METIN\"", 257, nil},
		"Shrink1":     {"PERFECT DARK", 6912, nil},
		"Shrink2":     {"PERFECT DARK", 1337, nil},
		"Create":      {"NEWFILE", 1000, nil},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			testdata := writeableTestdata(t, "clktmr.mpk")
			pfs, err := Read(testdata)
			if err != nil {
				t.Fatal("damaged testdata:", err)
			}
			free := pfs.Free()

			fi, _ := pfs.Open(tc.name)
			if strings.HasPrefix(name, "Create") {
				fi, _ = pfs.Create(tc.name)
			}
			f, _ := fi.(*File)
			var oldSize int64

			if tc.err == nil {
				oldSize = f.Size()
			}

			err = pfs.Truncate(tc.name, tc.size)
			if !errors.Is(err, tc.err) {
				t.Fatalf("expected %v, got %v", tc.err, err)
			}

			pfs, err = Read(testdata)
			if err != nil {
				t.Fatal("damaged testdata:", err)
			}
			fi, _ = pfs.Open(tc.name)
			f, _ = fi.(*File)

			if tc.err == nil {
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

				// Check for zeroes after truncated size
				buf := make([]byte, f.Size()-tc.size)
				zeroes := make([]byte, len(buf))
				_, err := f.ReadAt(buf, tc.size)
				if err != nil && err != io.EOF {
					t.Fatal(err)
				}
				if !bytes.Equal(buf, zeroes) {
					t.Fatal("data after truncated size")
				}
			}
			if pfs.Free() != free {
				t.Fatalf("expected %v free bytes, got %v", free, pfs.Free())
			}
		})
	}
}

func TestParallel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping in short mode")
	}

	const maxFileSize = 1024
	filenames := [...]string{
		".",
		"PERFECT ",
		"PERFECT DARK",
		"V82, \"METIN\"",
		"NEWFILE.1",
		"NEWFILE.2",
	}
	testdata := writeableTestdata(t, "clktmr.mpk")
	pfs, err := Read(testdata)
	if err != nil {
		t.Fatal("damaged testdata:", err)
	}

	for _, filename := range filenames {
		filename := filename
		t.Run(filename, func(t *testing.T) {
			var err error
			t.Parallel()
			timer := time.NewTimer(5 * time.Second)
			f, _ := pfs.Open(filename)
			switch f := f.(type) {
			case *File:
				for {
					f.fs.Truncate(filename, int64(rand.Intn(maxFileSize)))
					if err != nil {
						t.Fatal(err)
					}
					offset := int64(rand.Intn(maxFileSize))
					_, err = f.WriteAt([]byte(lorem), offset)
					if err != nil {
						t.Fatal(err)
					}
					buf := make([]byte, len(lorem))
					_, err = f.ReadAt(buf, offset)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(buf, []byte(lorem)) {
						t.Fatal("read unexpected data")
					}

					select {
					case <-timer.C:
						return
					default:
						continue
					}
				}
			case *rootDir:
				for {
					entries := pfs.ReadDirRoot()
					for _, entry := range entries {
						name := entry.Name()
						if !slices.Contains(filenames[:], name) {
							t.Fatalf("unexpected file %q", name)
						}
					}
					select {
					case <-timer.C:
						return
					default:
						continue
					}
				}
			case nil:
				for {
					f, err = pfs.Create(filename)
					if err != nil {
						t.Fatal(err)
					}
					f, _ := f.(*File)
					offset := int64(rand.Intn(maxFileSize))
					_, err = f.WriteAt([]byte(lorem), offset)
					if err != nil {
						t.Fatal(err)
					}
					buf := make([]byte, len(lorem))
					_, err = f.ReadAt(buf, offset)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(buf, []byte(lorem)) {
						t.Fatal("read unexpected data")
					}
					err = pfs.Remove(filename)
					if err != nil {
						t.Fatal(err)
					}
					select {
					case <-timer.C:
						return
					default:
						continue
					}
				}
			}
		})
	}

	pfs, err = Read(testdata)
	if err != nil {
		t.Fatal("damaged testdata:", err)
	}
}
