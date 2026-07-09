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
{{ $I := .Name }}
{{ $castU := $I }}
{{ $castF := $I }}

{{ if ne .Frac 0 }}
{{ $castU = printf "%v%v" $I "U" }}
{{ $castF = printf "%v%v" $I "F" }}

type {{ $I }} {{ .Base }}

func {{ $castU }}(i int) {{ $I }}     { return {{ $I }}(i<<{{ .Frac }}) }
func {{ $castF }}(f float32) {{ $I }} { return {{ $I }}(f*(1<<{{ .Frac }})) }

func (x {{ $I }}) Floor() int              { return int(x >> {{ .Frac }}) }
func (x {{ $I }}) Ceil() int               { return int(({{ .MulType }}(x) + (1<<{{ .Frac }} - 1)) >> {{ .Frac }}) }
func (x {{ $I }}) Mul(y {{ $I }}) {{ $I }} { return {{ $I }}(({{ .MulType }}(x)*{{ .MulType }}(y))>>{{ .Frac }}) }
func (x {{ $I }}) Div(y {{ $I }}) {{ $I }} { return {{ $I }}({{ .MulType }}(x)<<{{ .Frac }}/{{ .MulType }}(y)) }
func (x {{ $I }}) String() string          { return asString(int64(x), {{ .Frac }}, {{ .IntDigits }}, {{ .FracDigits }}) }
{{ end }}

{{ $P := printf "Point%s" .Suffix }}
type {{ $P }} struct{
	X, Y {{ $I }}
}

func Pt{{ .Suffix }}U(x, y int) {{ $P }}      { return {{ $P }}{ {{ $castU }}(x), {{ $castU }}(y)} }
func Pt{{ .Suffix }}F(x, y float32) {{ $P }}  { return {{ $P }}{ {{ $castF }}(x), {{ $castF }}(y)} }
func Pt{{ .Suffix }}P(p image.Point) {{ $P }} { return {{ $P }}{ {{ $castU }}(p.X), {{ $castU }}(p.Y)} }

func (p {{ $P }}) Add(q {{ $P }}) {{ $P }} { return {{ $P }}{p.X + q.X, p.Y + q.Y} }
func (p {{ $P }}) Sub(q {{ $P }}) {{ $P }} { return {{ $P }}{p.X - q.X, p.Y - q.Y} }
func (p {{ $P }}) Mul(k {{ $I }}) {{ $P }} { return {{ $P }}{ {{ mul "p.X" "k" }}, {{ mul "p.Y" "k" }} } }
func (p {{ $P }}) Div(k {{ $I }}) {{ $P }} { return {{ $P }}{ {{ div "p.X" "k" }}, {{ div "p.Y" "k" }} } }
func (p {{ $P }}) Pt() image.Point         { return image.Point{ {{ floor "p.X" }}, {{ floor "p.Y" }} } }

{{ $R := printf "Rectangle%s" .Suffix }}
type {{ $R }} struct{
	Min, Max {{ $P }}
}

func Rect{{ .Suffix }}U(x0, y0, x1, y1 int)     {{ $R }} {
	return {{ $R }}{ Pt{{ .Suffix }}U(x0, y0), Pt{{ .Suffix }}U(x1, y1)}
}

func Rect{{ .Suffix }}F(x0, y0, x1, y1 float32) {{ $R }} {
	return {{ $R }}{ Pt{{ .Suffix }}F(x0, y0), Pt{{ .Suffix }}F(x1, y1)}
}

func Rect{{ .Suffix }}R(r image.Rectangle) {{ $R }} {
	return {{ $R }}{ Pt{{ .Suffix }}P(r.Min), Pt{{ .Suffix }}P(r.Max)}
}

func (r {{ $R }}) Add(p {{ $P }}) {{ $R }} {
	return {{ $R }}{
		{{ $P }}{r.Min.X + p.X, r.Min.Y + p.Y},
		{{ $P }}{r.Max.X + p.X, r.Max.Y + p.Y},
	}
}

func (r {{ $R }}) Sub(p {{ $P }}) {{ $R }} {
	return {{ $R }}{
		{{ $P }}{r.Min.X - p.X, r.Min.Y - p.Y},
		{{ $P }}{r.Max.X - p.X, r.Max.Y - p.Y},
	}
}

func (r {{ $R }}) Intersect(s {{ $R }}) {{ $R }} {
	r.Min.X = max(r.Min.X, s.Min.X)
	r.Min.Y = max(r.Min.Y, s.Min.Y)
	r.Max.X = min(r.Max.X, s.Max.X)
	r.Max.Y = min(r.Max.Y, s.Max.Y)
	if r.Empty() {
		return {{ $R }}{}
	}
	return r
}

func (r {{ $R }}) Union(s {{ $R }}) {{ $R }} {
	if r.Empty() {
		return s
	}
	if s.Empty() {
		return r
	}
	r.Min.X = min(r.Min.X, s.Min.X)
	r.Min.Y = min(r.Min.Y, s.Min.Y)
	r.Max.X = max(r.Max.X, s.Max.X)
	r.Max.Y = max(r.Max.Y, s.Max.Y)
	return r
}

func (r {{ $R }}) Empty() bool {
	return r.Min.X >= r.Max.X || r.Min.Y >= r.Max.Y
}

func (r {{ $R }}) In(s {{ $R }}) bool {
	if r.Empty() {
		return true
	}
	return s.Min.X <= r.Min.X && r.Max.X <= s.Max.X &&
		s.Min.Y <= r.Min.Y && r.Max.Y <= s.Max.Y
}

func (r {{ $R }}) Rect() image.Rectangle {
	return image.Rectangle{ {{ $P }}(r.Min).Pt(), {{ $P }}(r.Max).Pt()}
}
`

type fixedType struct {
	Name, Base, Suffix, MulType string
	Frac, FracDigits, IntDigits uint
}

func fromDecl(name, basetype string) (f fixedType) {
	f.Name = name
	f.Base = basetype
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

	var err error
	var intbits, width uint
	if strings.Contains(name, "_") {
		_, err = fmt.Sscanf(name, "%d_%d", &intbits, &f.Frac)
	} else {
		_, err = fmt.Sscanf(name, "%d", &intbits)
		f.Name = basetype
	}
	if err != nil && err != io.EOF {
		log.Fatalln(err)
	}

	if signed {
		f.Suffix = name
		_, err = fmt.Sscanf(basetype, "int%d", &width)
	} else {
		f.Suffix = "U" + name
		_, err = fmt.Sscanf(basetype, "uint%d", &width)
	}
	if err != nil && err != io.EOF {
		log.Fatalln(err)
	}

	if f.Frac+intbits != width {
		log.Fatalln("must use all bits")
	}
	f.FracDigits = digits(f.Frac)
	f.IntDigits = digits(intbits)
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

	var funcMap template.FuncMap
	f := fromDecl(os.Args[1], os.Args[2])
	if f.Frac == 0 {
		funcMap = template.FuncMap{
			"mul":   func(a, b any) string { return fmt.Sprintf("%v * %v", a, b) },
			"div":   func(a, b any) string { return fmt.Sprintf("%v / %v", a, b) },
			"floor": func(a any) string { return fmt.Sprintf("int(%v)", a) },
		}
	} else {
		funcMap = template.FuncMap{
			"mul":   func(a, b any) string { return fmt.Sprintf("%v.Mul(%v)", a, b) },
			"div":   func(a, b any) string { return fmt.Sprintf("%v.Div(%v)", a, b) },
			"floor": func(a any) string { return fmt.Sprintf("%v.Floor()", a) },
		}
	}

	source := bytes.NewBuffer(nil)
	tmpl, err := template.New("fixedTemplate").Funcs(funcMap).Parse(fixedTemplate)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Fprintln(source, "package fixed")
	fmt.Fprintln(source, "import \"image\"")

	err = tmpl.Execute(source, f)
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
