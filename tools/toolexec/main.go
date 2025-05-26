// toolexec is invocated with go build's -toolexec flag. It enforces settings
// that are required for n64 build.
package toolexec

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/clktmr/n64/drivers/cartfs"
)

// TODO share with mkrom
const (
	entryAddr = 0x400
	ipl3Size  = 0x1000
	romBase   = 0x1000_0000 + ipl3Size - entryAddr
)

func Main(args []string) {
	if len(args) < 2 {
		log.Fatalln("no command")
	}
	cmdname := args[1]
	cmdargs := args[2:]

	tool := filepath.Base(cmdname)
	switch tool {
	case "link":
		cmdargs = preLink(cmdargs)
	case "compile":
		cmdargs = preCompile(cmdargs)
	}

	cmd := exec.Command(cmdname, cmdargs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err, ok := err.(*exec.ExitError); ok {
		os.Exit(err.ExitCode())
	}
	if err != nil {
		log.Fatalln(err)
	}

	switch tool {
	case "link":
		postLink()
	case "compile":
		postCompile()
	}
}

// boolFlag gets parsed like a bool flag, i.e. the optional parameter must be
// set via the "=" flag syntax, but allows other values than true and false.
type boolFlag struct{ val string }

func (c *boolFlag) Set(s string) error { c.val = s; return nil }
func (c *boolFlag) String() string     { return c.val }
func (c *boolFlag) IsBoolFlag() bool   { return true }

var linkArgs = flag.NewFlagSet("link", flag.ContinueOnError)
var (
	linkPrintVersion  = linkArgs.String("V", "", "")
	linkOutfilePath   = linkArgs.String("o", "", "")
	linkImportcfgPath = linkArgs.String("importcfg", "", "")
	linkFormatType    = linkArgs.String("H", "", "")
)

var linkIgnoredBoolFlags = []string{
	"8", "a", "asan", "aslr", "bindnow", "c", "checklinkname",
	"compressdwarf", "d", "debugnosplit", "dumpdep", "f", "g", "h",
	"linkshared", "msan", "n", "pruneweakmap", "race", "s", "v", "w",
}

var linkIgnoredFlags = []string{
	"B", "E", "F", "I", "L", "M", "R", "T", "X", "benchmark",
	"benchmarkprofile", "buildid", "buildmode", "capturehostobjs",
	"cpuprofile", "debugtextsize", "debugtramp", "extar", "extld",
	"extldflags", "fipso", "installsuffix", "k", "libgcc", "linkmode",
	"memprofile", "memprofilerate", "pluginpath", "r", "randlayout",
	"strictdups", "stripfn", "tmpdir",
}

func init() {
	for _, name := range linkIgnoredBoolFlags {
		linkArgs.Var(&boolFlag{}, name, "")
	}
	for _, name := range linkIgnoredFlags {
		linkArgs.String(name, "", "")
	}
}

func preLink(args []string) []string {
	linkArgs.SetOutput(io.Discard)
	err := linkArgs.Parse(args)
	if err != nil {
		log.Fatalln("ldflags:", err)
	}

	if *linkPrintVersion != "" {
		// TODO modify version based on this binaries buildid to
		// invalidate caches if the tool has changed
		return args
	}

	// Remove output format and forward it to mkrom
	filteredArgs := make([]string, 0)
	linkArgs.Visit(func(f *flag.Flag) {
		// Enforce symbols cause they are currently needed by mkrom
		// TODO Check if we can use ldflags -X instead
		if f.Name == "s" {
			return
		}
		filteredArgs = append(filteredArgs, "-"+f.Name+"="+f.Value.String())
	})
	filteredArgs = append(filteredArgs, "-M=0x00000000:8M")
	filteredArgs = append(filteredArgs, "-F=0x00000400:8M")
	filteredArgs = append(filteredArgs, linkArgs.Args()...)

	return filteredArgs
}

// postLink collects all generated cartfs images from the dependencies and
// writes them into the output elf into a new section ".cartfs". It then updates
// the cartfs'es symbol values to point to the correct addresses.
//
// If a cartfs got removed during dead code elimination, it's cartfs images
// won't be included.
func postLink() {
	if *linkPrintVersion != "" {
		return
	}

	// Open output elf file for modifying
	elfFile, err := os.OpenFile(*linkOutfilePath, os.O_RDWR, 0666)
	defer elfFile.Close()
	if err != nil {
		log.Fatalln("open elf:", err)
	}
	elfFile64, err := readElf64(elfFile)
	if err != nil {
		log.Fatalln("read elf:", err)
	}

	// Go through all dependencies in importcfg and collect cartfs images
	importcfgFile, err := os.Open(*linkImportcfgPath)
	defer importcfgFile.Close()
	if err != nil {
		log.Fatalln("open importcfg:", err)
	}

	cartfses := bytes.NewBuffer(nil)
	offsets := make(map[string]uint32)
	scanner := bufio.NewScanner(importcfgFile)
	for scanner.Scan() { // for each dependency
		line := scanner.Text()
		kvpair := strings.TrimPrefix(line, "packagefile ")
		if kvpair == line {
			continue
		}
		_, pkgfilePath, found := strings.Cut(kvpair, "=")
		if !found {
			log.Fatalln("parsing importcfg:", line)
		}

		// Open package archive for reading
		pkgfile, err := os.Open(pkgfilePath)
		if err != nil {
			log.Fatalln(err)
		}
		ar, err := parseArchive(pkgfile)
		if err != nil {
			log.Fatalln(err)
		}

		// Parse cartfscfg from package archive
		cartfscfgEntry := ar.OpenEntry("cartfscfg")
		if cartfscfgEntry == nil {
			continue
		}
		cartfscfgJSON, err := io.ReadAll(cartfscfgEntry)
		symbolNames := make(map[string]string)
		err = json.Unmarshal(cartfscfgJSON, &symbolNames)
		if err != nil {
			log.Fatalln("parse cartfscfg:", err)
		}
		for cartfsName, symbol := range symbolNames {
			_, err = elfFile64.Symbol(symbol)
			if err == errNoSymbol {
				continue // dead symbol
			} else if err != nil {
				log.Fatalln(err)
			}

			pad := alignUp(uint64(cartfses.Len()), cartfs.Align) - uint64(cartfses.Len())
			_, err := cartfses.Write(make([]byte, pad))
			if err != nil {
				log.Fatalln(err)
			}

			offsets[symbol] = uint32(cartfses.Len())

			cartfsdev := ar.OpenEntry(cartfsName)
			if cartfsdev == nil {
				log.Fatalln(err)
			}
			_, err = io.Copy(cartfses, cartfsdev)
			if err != nil {
				log.Fatalln(err)
			}

			pad = alignUp(uint64(cartfses.Len()), cartfs.Align) - uint64(cartfses.Len())
			_, err = cartfses.Write(make([]byte, pad))
			if err != nil {
				log.Fatalln(err)
			}
		}
	}

	sectionAddr := elfFile64.AddProgSection(".cartfs", cartfs.Align, cartfses.Bytes())
	cartfsBase := romBase + uint32(sectionAddr)

	for symbol, cartfsOffset := range offsets {
		err = elfFile64.SetSymbol(symbol, cartfsBase+cartfsOffset)
		if err != nil {
			log.Fatalln(err)
		}
	}

	err = elfFile.Truncate(0)
	if err != nil {
		log.Fatalln(err)
	}
	err = elfFile64.Write(elfFile)
	if err != nil {
		log.Fatalln(err)
	}
}

var compileArgs = flag.NewFlagSet("compile", flag.ContinueOnError)
var (
	compilePrintVersion = compileArgs.String("V", "", "")
	compileOutfilePath  = compileArgs.String("o", "", "")
	compileImportPath   = compileArgs.String("p", "", "")
	compileEmbedcfgPath = compileArgs.String("embedcfg", "", "")
)

var compileIgnoredBoolFlags = []string{
	"%", "+", "B", "C", "E", "K", "L", "N", "S", "W", "asan", "clobberdead",
	"clobberdeadreg", "complete", "dwarf", "dwarfbasentries",
	"dwarflocationlists", "dynlink", "e", "errorurl", "h", "j", "l",
	"linkshared", "live", "m", "msan", "nolocalimports", "pack", "r",
	"race", "shared", "smallframes", "std", "t", "v", "w", "wb",
}

var compileIgnoredFlags = []string{
	"D", "I", "asmhdr", "bench", "blockprofile", "buildid", "c",
	"coveragecfg", "cpuprofile", "d", "env", "gendwarfinl", "goversion",
	"importcfg", "installsuffix", "json", "lang", "linkobj", "memprofile",
	"memprofilerate", "mutexprofile", "pgoprofile", "spectre", "symabis",
	"traceprofile", "trimpath",
}

func init() {
	for _, name := range compileIgnoredBoolFlags {
		compileArgs.Var(&boolFlag{}, name, "")
	}
	for _, name := range compileIgnoredFlags {
		compileArgs.String(name, "", "")
	}
}

func preCompile(args []string) []string {
	compileArgs.SetOutput(io.Discard)
	err := compileArgs.Parse(args)
	if err != nil {
		log.Fatalln("gcflags:", err)
	}

	if *compilePrintVersion != "" {
		// TODO modify version based on this binaries buildid to
		// invalidate caches if the tool has changed
		return args
	}

	return args
}

// postCompile scans the package for calls to cartfs.Embed() and generates the
// cartfs images. These images are appended to the archive generated by the
// compiler and will be correctly cached by the go tool.
func postCompile() {
	if *compilePrintVersion != "" {
		return
	}

	if *compileEmbedcfgPath == "" {
		return
	}

	// Read and parse embedcfg
	embedcfgJSON, err := os.ReadFile(*compileEmbedcfgPath)
	if err != nil {
		log.Fatalln("read embedcfg:", err)
	}
	var embedcfg embedConfig
	err = json.Unmarshal(embedcfgJSON, &embedcfg)
	if err != nil {
		log.Fatalln("parse embedcfg:", err)
	}

	// Scan package for cartfs declarations
	cartfsDecls, err := scanCartfsEmbed(*compileImportPath)
	if err != nil {
		log.Fatalln("scan declarations:", err)
	}

	if len(cartfsDecls) == 0 {
		return
	}

	// Open output file
	file, err := os.OpenFile(*compileOutfilePath, os.O_RDWR, 0666)
	if err != nil {
		log.Fatalln("open archive:", err)
	}
	defer file.Close()

	ar, err := parseArchive(file)
	if err != nil {
		log.Fatalln("parse archive:", err)
	}

	// Generate the cartfs filesystems
	symbolNames := make(map[string]string)
	for i, decl := range cartfsDecls {
		cartfsFile, err := os.CreateTemp("", "cartfs")
		if err != nil {
			log.Fatalln("create tempfile:", err)
		}

		err = cartfsCreate(cartfsFile, embedcfg, decl.Patterns)
		if err != nil {
			log.Fatalln("create cartfs:", err)
		}

		cartfsName := "cartfs" + strconv.Itoa(i)
		ar.AddEntry(cartfsName, cartfsFile)
		symbolNames[cartfsName] = decl.SymbolName()

		cartfsFile.Close()
	}

	// Write a cartfscfg for the linker
	cartfscfgJSON, err := json.Marshal(symbolNames)
	if err != nil {
		log.Fatalln("serialize cartfscfg:", err)
	}
	ar.AddEntry("cartfscfg", bytes.NewReader(cartfscfgJSON))
}

type embedConfig struct {
	Patterns map[string][]string
	Files    map[string]string
}

func cartfsCreate(dev io.WriterAt, embedcfg embedConfig, patterns []string) error {
	files := make(map[string]string)
	for _, pattern := range patterns {
		for _, file := range embedcfg.Patterns[pattern] {
			files[file] = embedcfg.Files[file]
		}
	}
	return cartfs.Create(dev, files)
}
