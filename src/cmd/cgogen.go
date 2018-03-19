package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"

	"github.com/dave/jennifer/jen"
)

type Config struct {
	Path    string
	Verbose bool
}

func (c *Config) register() {
	flag.StringVar(&c.Path, "i", "", "PATH to source file")
	flag.BoolVar(&c.Verbose, "v", false, "Print debug message to stdout")
}

var (
	cfg    Config
	applog = func(format string, v ...interface{}) {
		// Logging disabled
	}
)

func main() {
	cfg.register()
	flag.Parse()

	if cfg.Verbose {
		applog = log.Printf
	}

	applog("Opening %v \n", cfg.Path)
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
	outFile.CgoPreamble(`
  #include <string.h>
  #include <stdlib.h>
  
  #include "../../include/skytypes.h"`)

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
	applog("Finished %v", cfg.Path)
}

func isAsciiUpper(c rune) bool {
	return c >= 'A' && c <= 'Z'
}

func typeSpecStr(_typeExpr *ast.Expr) string {
	addPointer := false
	spec := ""
	for _typeExpr != nil {
		if arrayExpr, isArray := (*_typeExpr).(*ast.ArrayType); isArray {
			spec += "[]"
			_typeExpr = &arrayExpr.Elt
			continue
		}
		if starExpr, isStar := (*_typeExpr).(*ast.StarExpr); isStar {
			spec += "*"
			_typeExpr = &starExpr.X
			continue
		}
		if _, isEllipse := (*_typeExpr).(*ast.Ellipsis); isEllipse {
			spec += "..."
			_typeExpr = nil
			continue
		}
		if _, isFunc := (*_typeExpr).(*ast.FuncType); isFunc {
			// TODO: Improve func type translation
			spec += "C.Handle"
			_typeExpr = nil
			continue
		}
		if _, isStruct := (*_typeExpr).(*ast.StructType); isStruct {
			spec += "struct{}"
			_typeExpr = nil
			continue
		}
		if _, isIntf := (*_typeExpr).(*ast.InterfaceType); isIntf {
			spec += "interface{}"
			_typeExpr = nil
			continue
		}
		if _, isChan := (*_typeExpr).(*ast.ChanType); isChan {
			// TODO: Improve func type translation
			spec += "C.GoChan_"
			_typeExpr = nil
			continue
		}
		if mapExpr, isMap := (*_typeExpr).(*ast.MapType); isMap {
			return spec + "map[" + typeSpecStr(&mapExpr.Key) + "]" + typeSpecStr(&mapExpr.Value)
		}
		identExpr, isIdent := (*_typeExpr).(*ast.Ident)
		selExpr, isSelector := (*_typeExpr).(*ast.SelectorExpr)
		if isIdent || isSelector {
			typeName := ""
			if isIdent {
				typeName = identExpr.Name
			} else {
				typeName = selExpr.Sel.Name
			}
			isExported := isAsciiUpper(rune(typeName[0]))
			if spec == "" && !addPointer && isExported {
				addPointer = true
			}
			if isExported {
				spec += "C."
			}
			spec += typeName
			_typeExpr = nil
		} else {
			applog("No rules to follow with %s", (*_typeExpr).(*ast.Ident))
			_typeExpr = nil
		}
	}
	if addPointer {
		return "*" + spec
	}
	return spec
}

func argName(name string) string {
	return "_" + name
}

func processFunc(fast *ast.File, fdecl *ast.FuncDecl, outFile *jen.File) {
	funcName := fdecl.Name.Name

	if !fdecl.Name.IsExported() {
		applog("Skipping %v \n", funcName)
		return
	}

	applog("Processing %v \n", funcName)
	var params jen.Statement
	if receiver := fdecl.Recv; receiver != nil {
		// Method
		_type := &receiver.List[0].Type
		typeName := ""
		if starExpr, isPointerRecv := (*_type).(*ast.StarExpr); isPointerRecv {
			_type = &starExpr.X
		}
		if identExpr, isIdent := (*_type).(*ast.Ident); isIdent {
			typeName = identExpr.Name
		}
		recvParam := jen.Id(argName(receiver.List[0].Names[0].Name))
		recvParam = recvParam.Id(typeSpecStr(_type))
		params = append(params, recvParam)
		funcName = typeName + "_" + funcName
	}

	cfuncName := "SKY_" + fast.Name.Name + "_" + funcName
	stmt := outFile.Comment("export " + cfuncName)
	stmt = outFile.Func().Id(cfuncName)

	allparams := fdecl.Type.Params.List[:]
	var retField *ast.Field = nil
	if fdecl.Type.Results != nil && fdecl.Type.Results.List != nil {
		lastFieldIdx := len(fdecl.Type.Results.List) - 1
		retField = fdecl.Type.Results.List[lastFieldIdx]
		_, isArray := retField.Type.(*ast.ArrayType)
		_, isQual := retField.Type.(*ast.SelectorExpr)
		_, isIdent := retField.Type.(*ast.Ident)

		if isArray || isQual || (isIdent && retField.Type.(*ast.Ident).IsExported()) {
			allparams = append(allparams, fdecl.Type.Results.List[:]...)
			retField = nil
		} else {
			allparams = append(allparams, fdecl.Type.Results.List[:lastFieldIdx]...)
		}
	}
	for fieldIdx, field := range allparams {
		if field.Names == nil {
			// Field in return types list
			typeName := typeSpecStr(&field.Type)
			if rune(typeName[0]) == '[' {
				typeName = "*C.GoSlice_"
			}
			params = append(params, jen.Id(
				argName("arg"+fmt.Sprintf("%d", fieldIdx))).Id(typeName))
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

	var callparams []jen.Code
	for _, field := range fdecl.Type.Params.List {
		for _, name := range field.Names {
			callparams = append(callparams, *jen.Id(name.Name)...)
		}
	}

	blockParams := []jen.Code{
		jen.Comment("TODO: Implement"),
	}
	if fdecl.Recv != nil {
		blockParams = append(blockParams,
			jen.Id(fdecl.Recv.List[0].Names[0].Name).Dot(fdecl.Name.Name).Call(callparams...),
		)
	} else {
		blockParams = append(blockParams,
			jen.Qual("github.com/skycoin/skycoin/src/"+fast.Name.Name,
				fdecl.Name.Name).Call(callparams...),
		)
	}

	if retField != nil {
		retName := typeSpecStr(&retField.Type)
		if retName == "error" {
			stmt = stmt.Id("C.uint")
		} else {
			stmt = stmt.Id(retName)
		}
	}
	stmt.Block(blockParams...)
}

/*
func processType(fast *ast.File, tdecl *ast.TypeDecl, outFile *jen.File) {

}
*/
