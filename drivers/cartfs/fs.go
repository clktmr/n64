// Package cartfs wraps an embed.FS and stores it on the cartridge.
//
// The embed package can be used on the N64, but it will store the embedded data
// in the binary, which is loaded into RAM as a whole at boot. Instead, a
// cartfs.FS can be initialized from an embed.FS. The `mkrom` utility will copy
// the embedded files to the cartridge and initialize the filesystem. The
// embed.FS won't be included in the binary as the compiler will find it to be
// unused in it's dead code elimination pass. This also works the other way
// round: An unused cartfs.FS embedded files won't be included in the final ROM.
// One can safely import a package providing a collection of assets and be sure
// to have only the used ones take up storage.
//
// If compiled for other targets, cartfs just passes all calls to the underlying
// embed.FS.
//
// By building cartfs around the embed package, it works nicely with exisiting
// tooling. The embedded files will be checked by Go's embed first and properly
// bundled with the package, i.e. show up in 'go list' and 'go mod vendor'.
package cartfs

import (
	"embed"
	"encoding/binary"
	"errors"
	"io"
	"io/fs"
	"math"
	"os"
	"path"
	"slices"
	"strings"
)

type FS struct {
	base baseType // must be first field, known by mkrom tool

	dev   io.ReaderAt
	files []file
}

// Embed returns a cartfs.FS initialized from an embed.FS.
func Embed(f embed.FS) FS {
	return embedfs(f)
}

// Read opens a cartfs from image or block device.
func Read(dev io.ReaderAt) (fs *FS, err error) {
	r := io.NewSectionReader(dev, 0, math.MaxInt64)
	var lenEntries, lenPaths int64

	err = binary.Read(r, binary.BigEndian, &lenEntries)
	if err != nil {
		return
	}
	entries := make([]dirEntry, lenEntries)
	err = binary.Read(r, binary.BigEndian, entries)
	if err != nil {
		return
	}
	err = binary.Read(r, binary.BigEndian, &lenPaths)
	if err != nil {
		return
	}
	paths := make([]byte, lenPaths)
	err = binary.Read(r, binary.BigEndian, paths)
	if err != nil {
		return
	}

	filesBase, err := r.Seek(0, io.SeekCurrent)
	if err != nil {
		return
	}
	filesBase = (filesBase + alignMask) &^ alignMask

	files := make([]file, lenEntries)
	for idx := range files {
		files[idx].name = string(paths[entries[idx].Start:entries[idx].End])
		files[idx].size = entries[idx].Size
		files[idx].offset = entries[idx].Offset + filesBase
	}

	return &FS{dev: dev, files: files}, nil
}

// populateDirs returns a new slice with all files and directories found in
// filepaths. Names won't be added multiple times, i.e. it's safe to call
// populateDirs multiple times.
func populateDirs(files []string) (names []string) {
	for _, file := range files {
		if !slices.Contains(names, file) {
			names = append(names, file)
		}
		dir := file
		for {
			dir, _ = path.Split(trimSlash(dir))
			if dir == "" {
				break
			}
			if !slices.Contains(names, dir) {
				names = append(names, dir)
			}
		}
	}
	return
}

// resolveEmbeds mimics the pattern matching done by embed. It will however not
// do all the checks (valid names, module boundaries, etc.), as it expects files
// to be a list output by embed. The returned list is sorted as required by
// cartfs.FS.
func resolveEmbeds(files, patterns []string) ([]string, error) {
	files = populateDirs(files)

	// Find all matches
	var incfiles []string
	for _, pattern := range patterns {
		for _, file := range files {
			if match, err := path.Match(pattern, trimSlash(file)); err != nil {
				return nil, err
			} else if !match {
				continue
			}
			incfiles = append(incfiles, file)
		}
	}

	// Collect files in matched dirs
	for _, incfile := range incfiles {
		if _, _, isDir := split(incfile); isDir {
			for _, file := range files {
				if fname, found := strings.CutPrefix(trimSlash(file), incfile); found {
					if strings.HasPrefix(fname, ".") || strings.HasPrefix(fname, "_") {
						continue
					}
					incfiles = append(incfiles, file)
				}
			}
		}
	}
	incfiles = populateDirs(incfiles)

	slices.SortFunc(incfiles, compare)

	return incfiles, nil
}

// Create generates a cartfs from a list of filenames.
func Create(dev io.WriterAt, files, patterns []string) error {
	incfiles, err := resolveEmbeds(files, patterns)
	if err != nil {
		return err
	}

	// Calculate offsets for all files
	var offset int64
	paths := make([]byte, 0)
	entries := make([]dirEntry, 0)
	for _, file := range incfiles {
		info, err := os.Stat(file)
		if err != nil {
			return err
		}
		paths = append(paths, []byte(file)...)
		entries = append(entries, dirEntry{
			int64(len(paths) - len(file)), int64(len(paths)),
			info.Size(),
			offset,
		})
		offset += info.Size()
		offset = (offset + alignMask) &^ alignMask
	}

	w := io.NewOffsetWriter(dev, 0)
	err = binary.Write(w, binary.BigEndian, int64(len(entries)))
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.BigEndian, entries)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.BigEndian, int64(len(paths)))
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.BigEndian, paths)
	if err != nil {
		return err
	}
	written, err := w.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}

	written = (written + alignMask) &^ alignMask

	for _, entry := range entries {
		path := string(paths[entry.Start:entry.End])
		if _, _, isDir := split(path); isDir {
			continue
		}
		r, err := os.Open(path)
		if err != nil {
			return err
		}
		w := io.NewOffsetWriter(dev, entry.Offset+written)
		written, err := io.Copy(w, r)
		if err != nil {
			return err
		}
		if written != entry.Size {
			return errors.New("filesize changed")
		}
	}

	return nil
}

// Open opens the named file for reading and returns it as an [fs.File].
//
// The returned file implements [io.Seeker] and [io.ReaderAt] when the file is
// not a directory.
func (f *FS) Open(name string) (fs.File, error) {
	return f.baseOpen(name)
}

// ReadDir reads and returns the entire named directory.
func (f *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	return f.baseReadDir(name)
}

// ReadFile reads and returns the content of the named file.
func (f *FS) ReadFile(name string) ([]byte, error) {
	return f.baseReadFile(name)
}

func (f *FS) cartfsOpen(name string) (fs.File, error) {
	file := f.lookup(name)
	if file == nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	if file.IsDir() {
		return &openDir{file, f.readDir(file.name), 0}, nil
	}
	r := io.NewSectionReader(f.dev, file.offset, file.size)
	return &openFile{r, file}, nil
}

func (f *FS) cartfsReadDir(name string) ([]fs.DirEntry, error) {
	file, err := f.cartfsOpen(name)
	if err != nil {
		return nil, err
	}
	dir, ok := file.(*openDir)
	if !ok {
		return nil, &fs.PathError{Op: "read", Path: name, Err: errors.New("not a directory")}
	}
	list := make([]fs.DirEntry, len(dir.files))
	for i := range list {
		list[i] = &dir.files[i]
	}
	return list, nil
}

func (f *FS) cartfsReadFile(name string) ([]byte, error) {
	file, err := f.Open(name)
	if err != nil {
		return nil, err
	}
	ofile, ok := file.(*openFile)
	if !ok {
		return nil, &fs.PathError{Op: "read", Path: name, Err: errors.New("is a directory")}
	}
	return io.ReadAll(ofile)
}

func (f *FS) readDir(name string) []file {
	name = trimSlash(name)
	i, _ := slices.BinarySearchFunc(f.files, name, func(e file, s string) int {
		idir, _, _ := split(e.name)
		if idir >= s {
			return 1
		}
		return -1
	})
	j, _ := slices.BinarySearchFunc(f.files, name, func(e file, s string) int {
		jdir, _, _ := split(e.name)
		if jdir > s {
			return 1
		}
		return -1
	})

	return f.files[i:j]
}

// lookup returns the named file, or nil if it is not present.
func (f *FS) lookup(name string) *file {
	if name == "." {
		return dotFile
	}
	if f.files == nil {
		return nil
	}

	i, found := slices.BinarySearchFunc(f.files, name, func(e file, s string) int {
		return compare(trimSlash(e.name), s)
	})
	if found {
		return &f.files[i]
	}

	return nil
}

const Align = 8
const alignMask = Align - 1

// dirEntry specifies the binary representation of a cartfs directory entry.
type dirEntry struct {
	Start, End int64
	Size       int64
	Offset     int64
}

// Stolen from embed/embed.go
func split(name string) (dir, elem string, isDir bool) {
	if name[len(name)-1] == '/' {
		isDir = true
		name = name[:len(name)-1]
	}
	i := len(name) - 1
	for i >= 0 && name[i] != '/' {
		i--
	}
	if i < 0 {
		return ".", name, isDir
	}
	return name[:i], name[i+1:], isDir
}

// Stolen from embed/embed.go
func trimSlash(name string) string {
	if len(name) > 0 && name[len(name)-1] == '/' {
		return name[:len(name)-1]
	}
	return name
}

// compare implements the sortfunc used to sort embedded files and directories.
// See embed.FS for the rationale behind it.
func compare(a, b string) int {
	adir, aelem, _ := split(a)
	bdir, belem, _ := split(b)
	if bdir == adir {
		if belem == aelem {
			return 0
		} else if belem > aelem {
			return -1
		}
	} else if bdir > adir {
		return -1
	}
	return 1
}
