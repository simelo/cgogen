package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"

	"github.com/dave/jennifer/jen"
)

type Config struct {
	Path string
}

func (c *Config) register() {
	flag.StringVar(&c.Path, "i", "", "PATH to source file")
}

var (
	cfg Config
)

func main() {
	cfg.register()
	flag.Parse()

	fmt.Printf("Opening %v \n", cfg.Path)
	fo, err := os.Open(cfg.Path)
	if err != nil {
		fmt.Println(err)
		return
	}

	defer fo.Close()

	fset := token.NewFileSet()
	fast, err := parser.ParseFile(fset, "", fo, parser.AllErrors)
	if err != nil {
		fmt.Println(err)
		return
	}

	outFile := jen.NewFile("main")

	for _, _decl := range fast.Decls {
		if decl, ok := (_decl).(*ast.FuncDecl); ok {
			processFunc(fast, decl, outFile)
		}
		/*
			if decl, ok := _decl.(ast.FuncDecl); ok {
				processType(fast, decl, outFile)
			}
		*/
	}

	fmt.Printf("%#v", outFile)
}

func processFunc(fast *ast.File, fdecl *ast.FuncDecl, outFile *jen.File) {
	if !fdecl.Name.IsExported() {
		return
	}
	stmt := outFile.Func().Id(
		"SKY_" + fast.Name.Name + "_" + fdecl.Name.Name)
	var params jen.Statement
	if receiver := fdecl.Recv; receiver != nil {
		// Method
		// TODO: Param type
		params = append(params, *(jen.Id(receiver.List[0].Names[0].Name).Id("Type"))...)
	}
	for _, field := range fdecl.Type.Params.List {
		if field.Names == nil {
			// TODO: Param type
			params = append(params, jen.Id("Type"))
		} else {
			lastIdx := len(field.Names) - 1
			for idx, ident := range field.Names {
				if idx != lastIdx {
					params = append(params, jen.Id(ident.Name))
				} else {
					// TODO: Param type
					params = append(params, jen.Id(ident.Name).Id("Type"))
				}
			}
		}
	}
	stmt = stmt.Params(params...)
	// TODO: Function type
	stmt.String()
}

/*
func processType(fast *ast.File, tdecl *ast.TypeDecl, outFile *jen.File) {

}
*/
