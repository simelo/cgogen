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
	allparams := fdecl.Type.Params.List[:]
	var retField *ast.Field = nil
	if fdecl.Type.Results.List != nil {
		lastFieldIdx := len(fdecl.Type.Results.List) - 1
		retField = fdecl.Type.Results.List[lastFieldIdx]
		_, isArray := retField.Type.(*ast.ArrayType)
		if isArray || retField.Type.(*ast.Ident).IsExported() {
			allparams = append(allparams, fdecl.Type.Results.List[:]...)
			retField = nil
		} else {
			allparams = append(allparams, fdecl.Type.Results.List[:lastFieldIdx]...)
		}
	}
	for fieldIdx, field := range allparams {
		if field.Names == nil {
			// Field in return types list
			typeExpr := typeSpecStr(&field.Type)
			if typeExpr == "C.GoSlice" {
				typeExpr = "*C.GoSlice"
			}
			params = append(params, jen.Id(
				argName("arg"+fmt.Sprintf("%d", fieldIdx))).Id(typeExpr))
		} else {
			lastNameIdx := len(field.Names) - 1
			for nameIdx, ident := range field.Names {
				if nameIdx != lastNameIdx {
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
	if retField != nil {
		retName := retField.Type.(*ast.Ident).Name
		if retName == "error" {
			stmt = stmt.Id("C.uint")
		} else {
			stmt = stmt.Id(retName)
		}
	}
}

/*
func processType(fast *ast.File, tdecl *ast.TypeDecl, outFile *jen.File) {

}
*/
