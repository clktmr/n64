# Go on Nintendo 64

Develop applications for the Nintendo 64 in pure Go. Builds upon embeddedgo,
which adds a minimal rtos to the runtime via GOOS=noos.

## Prerequisites

 - Go
 - Git
 - ares emulator (optional)

## Getting Started

1. Install the embeddedgo toolchain:

```sh
    go install github.com/embeddedgo/dl/go1.24.4-embedded@latest
    go1.24.4-embedded download
```

2. Install n64go:

```sh
    go install github.com/clktmr/n64/tools/n64go@latest
```

   This tool helps managing n64 file formats. It's also hooked into the go
   command via the -toolexec flag to provide generation of z64 and uf2 ROM
   files.

3. Setup your build environment. Copy `go.env` from this repository to your
   desired location and make use of it:

```sh
    export GOENV="path/to/go.env"
```

   Alternatively you can of course use your preferred way of managing
   environment variables.

You can now use `go build` and `go run` as usual! Try it with the minimal hello
world example:

```go
package main

import _ "github.com/clktmr/n64/machine"

func main() {
	println("hello world!")
}
```

## Differences from mainline Go

### machine

Your application needs to import `github.com/clktmr/n64/machine` at some point,
which provides basic system setup. Otherwise your build will fail with a linker
error.

### fmt and log

Per default `fmt.Print()` and `log.Print()` write to `os.Stdout`, which isn't
set after boot. Use `embedded/rtos.Mount()` and
`github.com/embeddedgo/fs/termfs` to place an `io.Writer` at that location.

### os and net

Having no operating system has obvious consequences for the os package. There
are neither processes nor any network stack in the kernel. While `os/exec` is
not supported, networking applications can run if an implementation of the Conn
or Listener interface is passed to them.

### embed

While embed can be used, it will load all embedded files into RAM at boot. As an
alternative `github.com/clktmr/n64/drivers/cartfs` provides a fs.FS
implementation to read embedded files from the cartridge via DMA instead.

### testing

The `go test` command does currently not work reliably for several reasons:

 - The build might fail because if missing machine import
 - The tests might fail if they try to access testdata directory

This will probably be solved in the future. In the meantime fall back to writing
a dedicated test application like in `test/main.go`.

### cgo

cgo is not supported!
