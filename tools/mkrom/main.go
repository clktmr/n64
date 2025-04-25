// Copyright 2024 The Embedded Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"debug/elf"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime/debug"
	"strings"
	"sync"

	"github.com/clktmr/n64/drivers/cartfs"
)

const usageString = `ELF to n64 ROM converter.

Usage: %s [flags] <elffile>

`

var (
	infile string
	format = flag.String("format", "z64", "z64 | uf2")
)

func usage() {
	fmt.Fprintf(flag.CommandLine.Output(), usageString, os.Args[0])
	flag.PrintDefaults()
}

func main() {
	flag.Usage = usage
	flag.Parse()

	if flag.NArg() == 1 {
		infile = flag.Arg(0)
	} else {
		flag.Usage()
		os.Exit(1)
	}

	outfile, _ := strings.CutSuffix(infile, ".elf")
	outfile += "." + *format

	r := must(os.Open(infile))
	defer r.Close()
	elffile := must(elf.NewFile(r))
	defer elffile.Close()

	obj := NewObjcopy(elffile)

	binfo := buildinfo(obj)
	if binfo.Path == "" {
		fmt.Println("no module path in buildinfo")
		os.Exit(1)
	}

	// TODO read buildsettings from binfo

	cmd := exec.Command("go", "list", "-json", binfo.Path)
	output := must(cmd.Output())
	v := struct{ Deps []string }{}
	json.Unmarshal(output, &v)

	// scanCartfsDecls is slow (go/build.Import in particular), call it
	// concurrently for each dependency.
	var cartfsEmbeds []*cartfsEmbed
	var wg sync.WaitGroup
	var mtx sync.Mutex
	for _, dep := range v.Deps {
		wg.Add(1)
		go func() {
			defer wg.Done()
			decls := must(scanCartfsEmbed(dep))
			mtx.Lock()
			defer mtx.Unlock()
			cartfsEmbeds = append(cartfsEmbeds, decls...)
		}()
	}
	wg.Wait()

	for _, decl := range cartfsEmbeds {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command("go", "list", "-json", decl.path)
			output := must(cmd.Output())
			v := struct{ EmbedFiles []string }{}
			json.Unmarshal(output, &v)

			mtx.Lock()
			defer mtx.Unlock()

			const m = cartfs.Align - 1

			pad := (obj.data.Len()+m)&^m - obj.data.Len()
			must(obj.data.Write(padBytes(&ones, pad, 0xff)))

			ptrbuf := obj.SymbolData(decl.SymbolName())
			if ptrbuf == nil {
				return // dead symbol
			}

			addr := uint32(obj.data.Len() + len(n64Header) + len(n64IPL3))
			obj.ByteOrder().PutUint32(ptrbuf[:4], addr+0x1000_0000)

			cartfsdev := must(os.CreateTemp("", "cartfs"))
			cwd := must(os.Getwd())
			os.Chdir(decl.dir)
			must(0, cartfs.Create(cartfsdev, v.EmbedFiles, decl.patterns))
			os.Chdir(cwd)
			must(io.Copy(obj.data, cartfsdev))

			pad = (obj.data.Len()+m)&^m - obj.data.Len()
			must(obj.data.Write(padBytes(&ones, pad, 0xff)))
		}()
	}
	wg.Wait()

	n64WriteROMFile(outfile, *format, obj.data)
}

// Stolen from embed/embed.go
func trimSlash(name string) string {
	if len(name) > 0 && name[len(name)-1] == '/' {
		return name[:len(name)-1]
	}
	return name
}

// embeddedgo binaries aren't compatible with debug/buildinfo, as they store
// buildinfo in a different way. But buildinfo can be read from modinfo, like in
// debug.ReadBuildInfo().
func buildinfo(obj *Objcopy) *debug.BuildInfo {
	modinfo := string(obj.SymbolData("runtime.modinfo.str"))
	return must(debug.ParseBuildInfo(modinfo[16 : len(modinfo)-16]))
}

func must[T any](ret T, err error) T {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return ret
}
