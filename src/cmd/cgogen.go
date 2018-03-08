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

func isAsciiUpper(c rune) bool {
	return c >= 'A' && c <= 'Z'
}

func typeSpecStr(_typeExpr *ast.Expr) string {
	spec := ""
	if starExpr, isStar := (*_typeExpr).(*ast.StarExpr); isStar {
		spec += "*"
		_typeExpr = &starExpr.X
	}
	typeName := ""
	isIdent := false
	if identExpr, _isIdent := (*_typeExpr).(*ast.Ident); _isIdent {
		typeName = identExpr.Name
		isIdent = true
	}
	if _, isArray := (*_typeExpr).(*ast.ArrayType); isArray {
		typeName = "GoSlice"
	}
	if isAsciiUpper(rune(typeName[0])) {
		if isIdent && spec == "" {
			spec += "*C."
		} else {
			spec += "C."
		}
	}
	return spec + typeName
}

func argName(name string) string {
	return "_" + name
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
		recvParam := jen.Id(argName(receiver.List[0].Names[0].Name))
		recvParam = recvParam.Id(typeSpecStr(&receiver.List[0].Type))
		params = append(params, recvParam)
	}
	for _, field := range fdecl.Type.Params.List {
		fmt.Println("param")
		if field.Names == nil {
			params = append(params, jen.Id(typeSpecStr(&field.Type)))
		} else {
			lastIdx := len(field.Names) - 1
			for idx, ident := range field.Names {
				if idx != lastIdx {
					params = append(params, jen.Id(argName(ident.Name)))
				} else {
					params = append(params, jen.Id(
						argName(ident.Name)).Id(typeSpecStr(&field.Type)))
				}
			}
		}
	}
	stmt = stmt.Params(params...)
	// TODO: Function type
	stmt.Id("Type")
}

/*
func processType(fast *ast.File, tdecl *ast.TypeDecl, outFile *jen.File) {

}
*/
