package main

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"io/fs"
	"slices"
	"strings"
)

type cartfsEmbed struct {
	pkgname  string
	dir      string
	path     string
	name     string
	patterns []string
}

func (p *cartfsEmbed) SymbolName() string {
	if p.pkgname == "main" {
		return strings.Join([]string{p.pkgname, p.name}, ".")
	} else {
		return strings.Join([]string{p.path, p.name}, ".")
	}
}

// scanCartfsEmbed searches the package at path for global cartfs.FS variable
// declarations initialized with cartfs.Embed.
func scanCartfsEmbed(path string) (decls []*cartfsEmbed, err error) {
	pkg, err := build.Default.Import(path, ".", 0)
	if err != nil {
		return
	}

	for _, importPath := range pkg.Imports {
		if importPath == "github.com/clktmr/n64/drivers/cartfs" {
			goto found
		}
	}
	return
found:

	filter := func(finfo fs.FileInfo) bool {
		return slices.Contains(pkg.GoFiles, finfo.Name())
	}

	fset := token.NewFileSet()
	pkgast, err := parser.ParseDir(fset, pkg.Dir, filter, parser.ParseComments)
	if err != nil {
		return
	}

	// Inspect all global variable declarations
	mappings := make(map[string]cartfsEmbed)
	for _, file := range pkgast {
		ast.Inspect(file, func(n ast.Node) bool {
			switch c := n.(type) {
			case *ast.File:
				return true
			case *ast.Package:
				return true
			case *ast.GenDecl:
				if c.Tok != token.VAR {
					return false
				}

				err1 := inspectVarDecl(c, mappings)
				if err1 != nil {
					err = fmt.Errorf("%v: %v", pkg.ImportPath, err1)
					return false
				}
			}
			return false
		})
	}

	decls = make([]*cartfsEmbed, 0)
	for _, v := range mappings {
		decls = append(decls, &cartfsEmbed{
			pkgname:  pkg.Name,
			path:     pkg.ImportPath,
			dir:      pkg.Dir,
			patterns: v.patterns,
			name:     v.name,
		})
	}
	return
}

// inspectVarDecl searches decl for cartfs.FS initializations via cartfs.Embed()
// and for embed.FS initializations via go:embed. The results are stored in
// mapping, using the embed.FS variables name as key.
//
// FIXME package cartfs or embed might be imported under a different name
func inspectVarDecl(decl *ast.GenDecl, mapping map[string]cartfsEmbed) error {
	var embedfsSpecs, cartfsSpecs []*ast.ValueSpec
	for _, spec := range decl.Specs {
		if spec, ok := spec.(*ast.ValueSpec); ok {
			if stype, ok := spec.Type.(*ast.SelectorExpr); ok {
				if stype.Sel.String() != "FS" {
					continue
				}
				if ident, ok := stype.X.(*ast.Ident); ok {
					if ident.String() == "cartfs" {
						cartfsSpecs = append(cartfsSpecs, spec)
					} else if ident.String() == "embed" {
						if spec.Doc == nil && decl.Lparen == 0 {
							spec.Doc = decl.Doc // TODO hackish
						}
						embedfsSpecs = append(embedfsSpecs, spec)
					}
				}
			}
		}
	}

	// Check for cartfs.FS initializations
	for _, spec := range cartfsSpecs {
		for i := range spec.Values {
			if initcall, ok := spec.Values[i].(*ast.CallExpr); ok {
				if len(initcall.Args) == 1 {
					if initfun, ok := initcall.Fun.(*ast.SelectorExpr); ok {
						if initfun.Sel.Name == "Embed" {
							if pkgident, ok := initfun.X.(*ast.Ident); ok {
								if pkgident.String() == "cartfs" {
									if embedfsRef, ok := initcall.Args[0].(*ast.Ident); ok {
										m := mapping[embedfsRef.Name]
										if m.name != "" {
											return fmt.Errorf("Multiple cartfs.FS embed the same embed.FS")
										}
										m.name = spec.Names[i].Name
										mapping[embedfsRef.Name] = m
										continue
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Find go:embed patterns
	for _, spec := range embedfsSpecs {
		if len(spec.Names) != 1 {
			return fmt.Errorf("Multiple embed.FS per go:embed directive")
		}
		if spec.Doc == nil {
			continue
		}
		var patterns []string
		for _, doc := range spec.Doc.List {
			if args, found := strings.CutPrefix(doc.Text, "//go:embed"); found {
				var err error
				p, err := parseGoEmbed(args)
				if err != nil {
					return err
				}
				patterns = append(patterns, p...)
				m := mapping[spec.Names[0].Name]
				m.patterns = patterns
				mapping[spec.Names[0].Name] = m
			}
		}
	}

	return nil
}
