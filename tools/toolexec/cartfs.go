package toolexec

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

type cartfsEmbed struct {
	Name     string
	Patterns []string
}

// scanCartfsEmbed searches the package at path for global cartfs.FS variable
// declarations initialized with cartfs.Embed.
func scanCartfsEmbed(files []string, pkgname string) (decls []cartfsEmbed, err error) {
	importsCartfs := false
	fset := token.NewFileSet()
	pkgast := make(map[string]*ast.File)
	for _, file := range files {
		pkgast[file], err = parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			return
		}
		for _, importSpec := range pkgast[file].Imports {
			if importSpec.Path.Value == `"github.com/clktmr/n64/drivers/cartfs"` {
				importsCartfs = true
			}
		}
	}

	if !importsCartfs {
		return
	}

	// Inspect all global variable declarations
	mappings := make(map[string]cartfsEmbed)
	for _, file := range pkgast {
		for _, decl := range file.Decls {
			if decl, ok := decl.(*ast.GenDecl); ok {
				if decl.Tok != token.VAR {
					continue
				}

				err = inspectVarDecl(decl, mappings)
				if err != nil {
					return nil, fmt.Errorf("%v: %v", file.Name.Name, err)
				}
			}
		}
	}

	decls = make([]cartfsEmbed, 0)
	for _, v := range mappings {
		decls = append(decls, cartfsEmbed{
			Patterns: v.Patterns,
			Name:     pkgname + "." + v.Name,
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
										if m.Name != "" {
											return fmt.Errorf("Multiple cartfs.FS embed the same embed.FS")
										}
										m.Name = spec.Names[i].Name
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
				m.Patterns = patterns
				mapping[spec.Names[0].Name] = m
			}
		}
	}

	return nil
}
