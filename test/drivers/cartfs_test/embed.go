// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package cartfs_test

import (
	"embed"
	"io"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/clktmr/n64/drivers/cartfs"
)

// The tests in this file were copied from the embed package's tests to ensure
// both filesystems behave the same.

//go:embed testdata/h*.txt
//go:embed c*.txt testdata/g*.txt
var _global embed.FS
var global cartfs.FS = cartfs.Embed(_global)

func testFiles(t *testing.T, f cartfs.FS, name, data string) {
	t.Helper()
	d, err := f.ReadFile(name)
	if err != nil {
		t.Error(err)
		return
	}
	if string(d) != data {
		t.Errorf("read %v = %q, want %q", name, d, data)
	}
}

func testString(t *testing.T, s, name, data string) {
	t.Helper()
	if s != data {
		t.Errorf("%v = %q, want %q", name, s, data)
	}
}

func testDir(t *testing.T, f cartfs.FS, name string, expect ...string) {
	t.Helper()
	dirs, err := f.ReadDir(name)
	if err != nil {
		t.Error(err)
		return
	}
	var names []string
	for _, d := range dirs {
		name := d.Name()
		if d.IsDir() {
			name += "/"
		}
		names = append(names, name)
	}
	if !slices.Equal(names, expect) {
		t.Errorf("readdir %v = %v, want %v", name, names, expect)
	}
}

// Tests for issue 49514.
var _ = '"'
var _ = '\''
var _ = '🦆'

func TestGlobal(t *testing.T) {
	testFiles(t, global, "concurrency.txt", "Concurrency is not parallelism.\n")
	testFiles(t, global, "testdata/hello.txt", "hello, world\n")
	testFiles(t, global, "testdata/glass.txt", "I can eat glass and it doesn't hurt me.\n")

	if err := fstest.TestFS(&global, "concurrency.txt", "testdata/hello.txt"); err != nil {
		t.Fatal(err)
	}
}

//go:embed testdata
var _testDirAll embed.FS
var testDirAll cartfs.FS = cartfs.Embed(_testDirAll)

func TestDir(t *testing.T) {
	all := testDirAll
	testFiles(t, all, "testdata/hello.txt", "hello, world\n")
	testFiles(t, all, "testdata/i/i18n.txt", "internationalization\n")
	testFiles(t, all, "testdata/i/j/k/k8s.txt", "kubernetes\n")
	testFiles(t, all, "testdata/ken.txt", "If a program is too slow, it must have a loop.\n")

	testDir(t, all, ".", "testdata/")
	testDir(t, all, "testdata/i", "i18n.txt", "j/")
	testDir(t, all, "testdata/i/j", "k/")
	testDir(t, all, "testdata/i/j/k", "k8s.txt")
}

var (
	//go:embed testdata
	_testHiddenDir embed.FS
	testHiddenDir  cartfs.FS = cartfs.Embed(_testHiddenDir)

	//go:embed testdata/*
	_testHiddenStar embed.FS
	testHiddenStar  cartfs.FS = cartfs.Embed(_testHiddenStar)
)

func TestHidden(t *testing.T) {
	dir := testHiddenDir
	star := testHiddenStar

	testDir(t, dir, "testdata",
		"-not-hidden/", "ascii.txt", "glass.txt", "hello.txt", "i/", "ken.txt")

	testDir(t, star, "testdata",
		"-not-hidden/", ".hidden/", "_hidden/", "ascii.txt", "glass.txt", "hello.txt", "i/", "ken.txt")

	testDir(t, star, "testdata/.hidden",
		"fortune.txt", "more/") // but not .more or _more
}

func TestUninitialized(t *testing.T) {
	var uninitialized cartfs.FS
	testDir(t, uninitialized, ".")
	f, err := uninitialized.Open(".")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		t.Fatal(err)
	}
	if !fi.IsDir() {
		t.Errorf("in uninitialized embed.FS, . is not a directory")
	}
}

func TestOffset(t *testing.T) {
	file, err := testDirAll.Open("testdata/hello.txt")
	if err != nil {
		t.Fatal("Open:", err)
	}

	want := "hello, world\n"

	// Read the entire file.
	got := make([]byte, len(want))
	n, err := file.Read(got)
	if err != nil {
		t.Fatal("Read:", err)
	}
	if n != len(want) {
		t.Fatal("Read:", n)
	}
	if string(got) != want {
		t.Fatalf("Read: %q", got)
	}

	// Try to read one byte; confirm we're at the EOF.
	var buf [1]byte
	n, err = file.Read(buf[:])
	if err != io.EOF {
		t.Fatal("Read:", err)
	}
	if n != 0 {
		t.Fatal("Read:", n)
	}

	// Use seek to get the offset at the EOF.
	seeker := file.(io.Seeker)
	off, err := seeker.Seek(0, io.SeekCurrent)
	if err != nil {
		t.Fatal("Seek:", err)
	}
	if off != int64(len(want)) {
		t.Fatal("Seek:", off)
	}

	// Use ReadAt to read the entire file, ignoring the offset.
	at := file.(io.ReaderAt)
	got = make([]byte, len(want))
	n, err = at.ReadAt(got, 0)
	if err != nil {
		t.Fatal("ReadAt:", err)
	}
	if n != len(want) {
		t.Fatalf("ReadAt: got %d bytes, want %d bytes", n, len(want))
	}
	if string(got) != want {
		t.Fatalf("ReadAt: got %q, want %q", got, want)
	}

	// Use ReadAt with non-zero offset.
	off = int64(7)
	want = want[off:]
	got = make([]byte, len(want))
	n, err = at.ReadAt(got, off)
	if err != nil {
		t.Fatal("ReadAt:", err)
	}
	if n != len(want) {
		t.Fatalf("ReadAt: got %d bytes, want %d bytes", n, len(want))
	}
	if string(got) != want {
		t.Fatalf("ReadAt: got %q, want %q", got, want)
	}
}
