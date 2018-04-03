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

var typesMap = map[string]string{
	  "int": "GoInt_",
	  "uint": "GoUint_",
	  "int8": "GoInt8_",
	  "int16": "GoInt16_",
	  "int32": "GoInt32_",
	  "int64": "GoInt64_",
	  "byte": "GoUint8_",
	  "uint8": "GoUint8_",
	  "uint16": "GoUint16_",
	  "uint32": "GoUint32_",
	  "uint64": "GoUint64_",
	  "float32" : "GoFloat32_",
	  "float64" : "GoFloat64_",
	  "complex64" : "GoComplex64_",
	  "complex128" : "GoComplex128_",
	  "string" : "GoString_",
	  "bool" : "bool",
	}

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
	
	//Traverse all type declarations to convert them to c type definitions
	typeDefs := make ( [](*ast.GenDecl), 0 )
	for _, _decl := range fast.Decls {
		if decl, ok := (_decl).(*ast.GenDecl); ok {
			if decl.Tok == token.TYPE {
				typeDefs = append ( typeDefs, decl )
			}
		}
	}
	ctype_defs := processTypeDefs(fast, typeDefs)
	
	outFile.CgoPreamble(`
  #include <string.h>
  #include <stdlib.h>
  
  #include "../../include/skytypes.h"`)
	outFile.CgoPreamble(ctype_defs)

	
  
	for _, _decl := range fast.Decls {
		if decl, ok := (_decl).(*ast.FuncDecl); ok {
			processFunc(fast, decl, outFile)
		}
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

func convertCVarToGoVar(){
}

func processFunc(fast *ast.File, fdecl *ast.FuncDecl, outFile *jen.File) {

	funcName := fdecl.Name.Name

	if !fdecl.Name.IsExported() {
		applog("Skipping %v \n", funcName)
		return
	}

	applog("Processing %v \n", funcName)
	var blockParams []jen.Code
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
		recvParamName := receiver.List[0].Names[0].Name
		recvParam := jen.Id(argName(recvParamName))
		recvParam = recvParam.Id(typeSpecStr(_type))
		params = append(params, recvParam)
		funcName = typeName + "_" + funcName
		blockParams = append( blockParams, jen.Id(recvParamName).Op(":=").Id(argName(recvParamName)) )
	}

	cfuncName := "SKY_" + fast.Name.Name + "_" + funcName
	stmt := outFile.Comment("export " + cfuncName)
	stmt = outFile.Func().Id(cfuncName)

	allparams := fdecl.Type.Params.List[:]
	return_fields_index := len(allparams)
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
		if fieldIdx >= return_fields_index {
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
				blockParams = append( blockParams, jen.Id(ident.Name).Op(":=").Id(argName(ident.Name)) )
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

	retName := ""
	if retField != nil {
		retName = typeSpecStr(&retField.Type)
		if retName == "error" {
			retName = "C.uint32"
		}
		blockParams = append(blockParams, jen.Var().Id(argName("return_var")).Id(retName) )
	}
	
	var retvars []jen.Code
	if return_fields_index < len(allparams) {
		for i := return_fields_index; i < len(allparams); i++ {
			retvars = append(retvars, jen.Id(argName("arg"+fmt.Sprintf("%d", i))))
		}		
	}
	if retField != nil {
		retvars = append(retvars, jen.Id(argName("return_var")))
	}
	var call_func_code jen.Code
	if len(retvars) > 0 {
		if fdecl.Recv != nil {
			call_func_code = 
				jen.List(retvars...).Op("=").Id(fdecl.Recv.List[0].Names[0].Name).Dot(fdecl.Name.Name).Call(callparams...)
		} else {
			call_func_code = 
				jen.List(retvars...).Op("=").Qual("github.com/skycoin/skycoin/src/"+fast.Name.Name,
					fdecl.Name.Name).Call(callparams...)
		}
	} else {
		if fdecl.Recv != nil {
			call_func_code = jen.Id(fdecl.Recv.List[0].Names[0].Name).Dot(fdecl.Name.Name).Call(callparams...)
		} else {
			call_func_code = jen.Qual("github.com/skycoin/skycoin/src/"+fast.Name.Name,
					fdecl.Name.Name).Call(callparams...)
		}
	}
	blockParams = append(blockParams, call_func_code,)

	if retField != nil {
		stmt = stmt.Id(retName)
		blockParams = append( blockParams, jen.Return().Id(argName("return_var")) )
	}
	stmt.Block(blockParams...)
}

/* Returns the corresponding C type for a GO type*/
func goTypeToCType(goType string) string {
	if val, ok := typesMap[goType]; ok {
		return val
	} else {
		return goType
	}
}

/* Process a type expression. Returns the code in C for the type and ok if successfull */
func processTypeExpression(fast *ast.File, type_expr ast.Expr, 
							defined_types *[]string, 
							forwards_declarations *[]string, depth int) (string, bool) {
	c_code := ""
	result := false
	
	if typeStruct, isTypeStruct := (type_expr).(*ast.StructType); isTypeStruct {
		c_code += "struct{\n"
		error := false
		for _, field := range typeStruct.Fields.List{
			type_code, result := processTypeExpression(fast, field.Type, defined_types, forwards_declarations, depth + 1)
			if result {
				for i := 0; i < depth * 4; i++{
					c_code += " "
				}
				c_code += type_code
				for i, fieldName := range field.Names{
					if i > 0{
						c_code += ", "
					}
					c_code += fieldName.Name
				}
				c_code += ";\n"
			} else {
				error = true
			}
		}
		for i := 0; i < (depth - 1) * 4; i++{
			c_code += " "
		}
		c_code += "}"
		result = !error
	}else if _, isArray := (type_expr).(*ast.ArrayType); isArray {
		c_code += "GoSlice_ "
		result = true
	}else if _, isFunc := (type_expr).(*ast.FuncType); isFunc {
		c_code += "Handle "
		result = true
	}else if _, isIntf := (type_expr).(*ast.InterfaceType); isIntf {
		c_code += "GoInterface_ "
		result = true
	}else if _, isChan := (type_expr).(*ast.ChanType); isChan {
		c_code += "GoChan_ "
		result = true
	}else if _, isMap := (type_expr).(*ast.MapType); isMap {
		c_code += "GoMap_ "
		result = true
	}else if starExpr, isStart := (type_expr).(*ast.StarExpr); isStart {
		targetTypeExpr := starExpr.X
		type_code, ok := processTypeExpression(fast, targetTypeExpr, defined_types, forwards_declarations, depth + 1)
		if ok {
			c_code += type_code
			c_code += "* "
			result = true
		}
	}else if identExpr, isIdent := (type_expr).(*ast.Ident); isIdent {
		c_code = goTypeToCType(identExpr.Name) + " "
		type_found := false
		for _, defined_type := range *defined_types{
			if defined_type == identExpr.Name{
				type_found = true
			}
		}
		if !type_found{
			if forwards_declarations != nil {
				*forwards_declarations = append(*forwards_declarations, identExpr.Name)
				result = true
			} else {
				result = false
			}
		} else {
			result = true
		}
	}
	return c_code, result
}

/* Process a type definition in GO and returns the c code for the definition */
func processTypeDef(fast *ast.File, tdecl *ast.GenDecl, 
					defined_types *[]string, forwards_declarations *[]string) (string, bool) {
	result_code := ""
	result := true
	for _, s := range tdecl.Specs{
		if typeSpec, isTypeSpec := (s).(*ast.TypeSpec); isTypeSpec {
			type_c_code, ok := processTypeExpression(fast, typeSpec.Type, defined_types, forwards_declarations, 1)
			if ok {
				result_code += "typedef "
				result_code += type_c_code
				result_code += typeSpec.Name.Name
				result_code += ";\n"
				*defined_types = append( *defined_types, typeSpec.Name.Name )
			} else {
				result = false
			}
		}
	}
	return result_code, result
}

/* Process all type definitions. Returns c code for all the defintions */
func processTypeDefs(fast *ast.File, typeDecls []*ast.GenDecl) string {
	result_code := ""
	var defined_types []string
	for key, _ := range typesMap {
		defined_types = append( defined_types, key )
	}
	
	unprocessed := len( typeDecls )
	went_blank := false
	for unprocessed > 0 && !went_blank {
		went_blank = true
		for index, typeDecl := range typeDecls{
			if typeDecl != nil {
				typeCode, ok := processTypeDef(fast, typeDecl, &defined_types, nil)
				if ok {
					went_blank = false
					typeDecls[index] = nil
					result_code += typeCode
					unprocessed -= 1
				}
			}
		}
	}
	//TODO: if unprocessed > 0 then there cyclic type references. Use forward declarations.
	return result_code
}

