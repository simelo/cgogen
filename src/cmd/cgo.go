package main

import (
	"go/ast"
	"reflect"
	"go/token"
	"fmt"
)

type CCompiler struct{
	source 			*ast.File
	ccode  			*CCode
	includes		[]string
	currentType		*TypeDef
}

type CCode struct {
	typedefs 		[]TypeDef
}

type TypeDef struct {
	name 			string
	ccode 			string
	dependencies 	[]string
	defType			string // struct, map, whatever
}

func NewCompiler(source *ast.File) (compiler *CCompiler) {
	compiler = &CCompiler{}
	compiler.source = source
	compiler.ccode = &CCode{}
	return
}

func (c *CCompiler) GetHeaderCode() (header string) {
	header = "#pragma once\n"
	for _, include := range c.includes {
		header += fmt.Sprintf("#include \"%s\"\n", include)
	}
	
	for _, typedef := range c.ccode.typedefs {
		if typedef.defType != "" {
			prefix := c.source.Name.Name + package_separator
			header += typedef.defType + " " + prefix + typedef.name + ";\n"
		}
	}
	
	for _, typedef := range c.ccode.typedefs {
		prefix := c.source.Name.Name + package_separator
		header += "typedef " + typedef.ccode + " " + prefix + typedef.name + ";\n"
	}
	return
}

func (c *CCompiler) Compile() {
	for _, _decl := range c.source.Decls {
		if decl, ok := (_decl).(*ast.FuncDecl); ok {
			c.processFunction(decl)
		} else if decl, ok := (_decl).(*ast.GenDecl); ok {
			if decl.Tok == token.TYPE {
				c.processType(decl)
			} else if decl.Tok == token.IMPORT {
				c.processImport(decl)
			} else {
				c.processUnknown(_decl)
			}
		} else {
			c.processUnknown(_decl)
		}
	}
}

func (c *CCompiler) processUnknown(decl ast.Decl){
	s := reflect.ValueOf(decl).Elem()
	typeOfT := s.Type()
	applog("Don't know what to do with: %s", typeOfT)
}

func (c *CCompiler) processImport(decl *ast.GenDecl){
}

func (c *CCompiler) processType(tdecl *ast.GenDecl){
	for _, s := range tdecl.Specs{
		if typeSpec, isTypeSpec := (s).(*ast.TypeSpec); isTypeSpec {
			typedef := TypeDef{name:typeSpec.Name.Name}
			typedef.defType = ""
			c.currentType = &typedef
			code, ok := c.processTypeExpression( typeSpec.Type )
			if ok {
				typedef.ccode = code
				c.ccode.typedefs = append( c.ccode.typedefs, typedef) 
			}
			c.currentType = nil
		} else {
			c.processUnknown(tdecl)
		}
	}
}

func (c *CCompiler) processTypeExpression(type_expr ast.Expr) (string, bool) {
	if typeStruct, isTypeStruct := (type_expr).(*ast.StructType); isTypeStruct {
		return c.processStructType( typeStruct )
	} else if identExpr, isIdent := (type_expr).(*ast.Ident); isIdent {
		return c.processIdentifier(identExpr)
	} else if selectorExpr, isSelector := (type_expr).(*ast.SelectorExpr); isSelector {
		return c.processSelector(selectorExpr)
	}else if starExpr, isStart := (type_expr).(*ast.StarExpr); isStart {
		return c.processPointer(starExpr)
	} else {
		applog("Unknown type: %v", type_expr)
	}
	return "", false
}

func (c *CCompiler) processPointer(starExpr *ast.StarExpr) (string, bool) {
	targetTypeExpr := starExpr.X
	code, ok := c.processTypeExpression(targetTypeExpr)
	if ok {
		return code + "*", true
	} else {
		return "", false
	}
}

func (c *CCompiler) processSelector(selectorExpr *ast.SelectorExpr) (string, bool) {
	identExpr, isIdent := (selectorExpr.X).(*ast.Ident)
	if isIdent {
		return identExpr.Name + package_separator + selectorExpr.Sel.Name, true
	} else {
		applog("Selector with complex expression")
		return "", false
	}
}

func (c *CCompiler) processIdentifier(identExpr *ast.Ident) (string, bool) {
	type_code, isBasic := GetCTypeFromGoType(identExpr.Name)
	if isBasic {
		return type_code, true
	} else {
		//Asume type from current package
		if c.currentType != nil {
			c.currentType.dependencies = append( c.currentType.dependencies, 
				type_code )
		}
		return c.source.Name.Name + package_separator + type_code, true
	}
}

func (c *CCompiler) processStructType(typeStruct *ast.StructType) (string, bool) {
	c_code := "struct{\n"
	for _, field := range typeStruct.Fields.List{
		var names []string
		for _, fieldName := range field.Names{
			names = append( names, fieldName.Name )
		}
		if len(names) == 0 {
			names = append( names, "_unnamed")
		}
		for _, fieldName := range names{
			code, ok := c.processTypeExpression(field.Type)
			if ok {
				c_code += code + " " + fieldName + ";\n"
			} else {
				applog("Couldn't process %s", field.Type)
			}
		}
	}
	c_code += "}"
	if c.currentType != nil && c.currentType.defType == "" {
		c.currentType.defType = "struct"
	}
	return c_code, true
}

func (c *CCompiler) processFunction(decl *ast.FuncDecl){
}

/* Returns the corresponding C type for a GO type*/
func GetCTypeFromGoType(goType string) (string, bool) {
	if val, ok := basicTypesMap[goType]; ok {
		return val, true
	} else {
		return goType, false
	}
}

func IsBasicGoType(goType string) bool {
	if _, ok := basicTypesMap[goType]; ok {
		return true
	} else {
		return false
	}
}

func GetBasicTypes() map[string]string{
	return basicTypesMap
}

var basicTypesMap = map[string]string{
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
	  "error" : "GoInt32_",
	}

var package_separator = "__"