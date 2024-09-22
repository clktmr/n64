package pakfs

import (
	"bytes"
	"crypto/sha1"
	"io"
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

	if free := fs.Free(); free != 16896 {
		t.Fatalf("free: expected %v, got %v", 16896, free)
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
