//go:build ignore

package main

import (
	"bytes"
	"fmt"
	"go/format"
	"io"
	"log"
	"os"
	"strings"
	"text/template"
)

var fixedTemplate = `
func {{ .Name }}U(i int) {{ .Name }}     { return {{ .Name }}(i<<{{ .Frac }}) }
func {{ .Name }}F(f float32) {{ .Name }} { return {{ .Name }}(f*(1<<{{ .Frac }})) }

func (x {{ .Name }}) Floor() int             { return int(x >> {{ .Frac }}) }
func (x {{ .Name }}) Ceil() int              { return int({{ .MulType }}(x) + (1<<{{ .Frac }} - 1) >> {{ .Frac }}) }
func (x {{ .Name }}) Mul(y {{ .Name }}) {{ .Name }} { return {{ .Name }}(({{ .MulType }}(x)*{{ .MulType }}(y))>>{{ .Frac }}) }
func (x {{ .Name }}) Div(y {{ .Name }}) {{ .Name }} { return {{ .Name }}({{ .MulType }}(x)<<{{ .Frac }}/{{ .MulType }}(y)) }

func (x {{ .Name }}) String() string {
	const shift, mask = {{ .Frac }}, 1<<{{ .Frac }} - 1
	return fmt.Sprintf("%d:%0{{ .Digits }}d", {{ .MulType }}(x>>shift), {{ .MulType }}(x&mask))
}
`

type fixedType struct {
	Name, BaseType, MulType string
	Frac, Digits            uint
}

func fromDecl(name, basetype string) (f fixedType) {
	f.Name = name
	f.BaseType = basetype
	switch basetype {
	case "int32":
		f.MulType = "int64"
	case "uint32":
		f.MulType = "uint64"
	case "int16":
		f.MulType = "int32"
	case "uint16":
		f.MulType = "uint32"
	case "int8":
		f.MulType = "int16"
	case "uint8":
		f.MulType = "uint16"
	default:
		log.Fatalln("unsupported basetype:", basetype)
	}

	var signed, found bool
	if name, found = strings.CutPrefix(name, "Int"); found {
		signed = true
	} else if name, found = strings.CutPrefix(name, "UInt"); found {
		signed = false
	} else {
		log.Fatalln("invalid name:", f.Name)
	}

	var intbits, width uint
	_, err := fmt.Sscanf(name, "%d_%d", &intbits, &f.Frac)
	if err != nil && err != io.EOF {
		log.Fatalln(err)
	}
	if signed {
		_, err = fmt.Sscanf(basetype, "int%d", &width)
	} else {
		_, err = fmt.Sscanf(basetype, "uint%d", &width)
	}
	if err != nil && err != io.EOF {
		log.Fatalln(err)
	}
	if f.Frac+intbits != width {
		log.Fatalln("must use all bits")
	}
	f.Digits = digits(f.Frac)
	return
}

func digits(bits uint) uint {
	return uint(len(fmt.Sprint((1 << bits) - 1)))
}

func usage() {
	fmt.Printf("Usage: %v <typename> <basetype>\n", os.Args[0])
}

func main() {
	log.Default().SetFlags(log.Lshortfile)
	if len(os.Args) != 3 {
		usage()
		os.Exit(1)
	}

	source := bytes.NewBuffer(nil)
	tmpl, err := template.New("fixedTemplate").Parse(fixedTemplate)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Fprintln(source, "package fixed")
	fmt.Fprintln(source, "import \"fmt\"")

	err = tmpl.Execute(source, fromDecl(os.Args[1], os.Args[2]))
	if err != nil {
		log.Fatalln(err)
	}

	formattedSource, err := format.Source(source.Bytes())
	if err != nil {
		log.Fatalln(err)
	}
	err = os.WriteFile(strings.ToLower(os.Args[1])+"_fixed.go", formattedSource, 0644)
	if err != nil {
		log.Fatalln(err)
	}
}
