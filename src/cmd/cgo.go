package main

import (
	"go/ast"
	"reflect"
	"go/token"
	"fmt"
	"strings"
)

type CCompiler struct{
	source 			*ast.File
	ccode  			*CCode
	includes		[]string
	currentType		*TypeDef
	identsCount		int
}

type CCode struct {
	typedefs 		[]TypeDef
	constdefs		[]ConstDef
	forwards		[]string
	functions		[]Function
}

type ConstDef struct{
	name string
	value string
}

type TypeDef struct {
	name 			string
	ccode 			string
	suffix 			string
	dependencies 	[]string
	defType			string // struct, map, whatever this typedef represents
						   // used for forwards	
}

type Function struct{
	name 		string
	signature	string
	body		string
}

type Parameter struct {
	name 		string
	ccode 		string
	ctype		string
}

func NewCompiler() (compiler *CCompiler) {
	compiler = &CCompiler{}
	compiler.ccode = &CCode{}
	return
}

func (c *CCompiler) GetHeaderCode(addIncludes bool) (header string) {
	header = ""
	if addIncludes {
		header += "#pragma once\n"
		for _, include := range c.includes {
			header += fmt.Sprintf("#include \"%s\"\n", include)
		}
	}
	
	header += "\n\n"
	
	for _, constDef := range c.ccode.constdefs {
		header += "#define " + constDef.name + " " + constDef.value + "\n"
	}
	
	header += "\n\n"
	
	for _, f := range c.ccode.forwards {
		header += f + ";\n"
	}
	
	header += "\n\n"
	
	prefix := c.source.Name.Name + package_separator
	for _, typedef := range c.ccode.typedefs {
		header += "typedef " + typedef.ccode + " " + prefix + typedef.name + 
			typedef.suffix + ";\n"
	}
	
	header += "\n\n"
	for _, funcDef := range c.ccode.functions {
		header += funcDef.signature + ";\n"
	}
	
	return
}

func (c *CCompiler) Compile(source *ast.File) {
	c.source = source
	for _, _decl := range c.source.Decls {
		if decl, ok := (_decl).(*ast.FuncDecl); ok {
			c.processFunction(decl)
		} else if decl, ok := (_decl).(*ast.GenDecl); ok {
			if decl.Tok == token.TYPE {
				c.processType(decl)
			} else if decl.Tok == token.IMPORT {
				c.processImport(decl)
			} else if decl.Tok == token.CONST {
				c.processConstExpression(decl)
			} else {
				applog("Unknown declaration %s", decl.Tok)
			}
		} else {
			c.processUnknown(_decl)
		}
	}
	c.processDependencies()
}

func (c *CCompiler) processDependencies(){
	var orderedTypedefs []TypeDef
	removed := map[int] bool {}
	var noTypeAdded bool
	for !noTypeAdded {
		noTypeAdded = true
		for index, typeDef := range c.ccode.typedefs {
			is_removed, ok := removed[index]
			if !ok || !is_removed {
				all_deps_in := true
				for _, dep := range typeDef.dependencies{
					found_dep := false
					for _, t := range orderedTypedefs {
						if dep == t.name {
							found_dep = true
							break
						}
					}
					if !found_dep {
						all_deps_in = false
						break
					}
				}
				if all_deps_in {
					orderedTypedefs = append( orderedTypedefs, typeDef)
					noTypeAdded = false
					removed[index] = true
				}
			}
		}
	}
	prefix := c.source.Name.Name + package_separator
	for index, typeDef := range c.ccode.typedefs {
		is_removed, ok := removed[index]
		if !ok || !is_removed {
			if typeDef.defType == "struct" {
				
				f := "struct " + prefix + typeDef.name
				c.ccode.forwards = append( c.ccode.forwards, f )
			}
			orderedTypedefs = append( orderedTypedefs, typeDef)
		}
	}
	c.ccode.typedefs = orderedTypedefs
}

func getTypeOfVar(decl ast.Expr) string{
	s := reflect.ValueOf(decl).Elem()
	return fmt.Sprintf("%s", s.Type())
}

func (c *CCompiler) processConstExpression(decl *ast.GenDecl){
	for _, s := range decl.Specs{
		if valueSpec, isValueSpec := (s).(*ast.ValueSpec); isValueSpec {
			for index, name := range valueSpec.Names {
				if index < len(valueSpec.Values){
					value := valueSpec.Values[index]
					value_code, ok := c.processConstValueExpression(value)
					if ok {
						consDef := ConstDef{name: name.Name, value: value_code}
						c.ccode.constdefs = append(c.ccode.constdefs, consDef)
					} else {
						applog("Can't handle const expression value : %v", value)
					}
				}
			}
		} else {
			applog("Don't know what to this const or var expression")
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
			typeName := typeSpec.Name.Name
			code, ok, suffix := c.processTypeExpression( typeSpec.Type)
			if ok {
				typedef.name = typeName
				typedef.ccode = code
				typedef.suffix = suffix
				c.ccode.typedefs = append( c.ccode.typedefs, typedef) 
			}
			c.currentType = nil
		} else {
			c.processUnknown(tdecl)
		}
	}
}

func (c *CCompiler) processTypeExpression(type_expr ast.Expr) (code string, result bool, suffix string) {
	code = ""
	result = false
	suffix = ""
	if typeStruct, isTypeStruct := (type_expr).(*ast.StructType); isTypeStruct {
		code, result = c.processStructType( typeStruct)
	} else if identExpr, isIdent := (type_expr).(*ast.Ident); isIdent {
		code, result = c.processIdentifier(identExpr)
	} else if selectorExpr, isSelector := (type_expr).(*ast.SelectorExpr); isSelector {
		code, result = c.processSelector(selectorExpr)
	}else if starExpr, isStart := (type_expr).(*ast.StarExpr); isStart {
		code, result = c.processPointer(starExpr)
	} else if arrayExpr, isArray := (type_expr).(*ast.ArrayType); isArray{
		code, result, suffix = c.processArray(arrayExpr)
	} else if mapExpr, isMap := (type_expr).(*ast.MapType); isMap {
		code, result = c.processMap(mapExpr)
	} else {
		applog("Unknown type: %v", type_expr)
	}
	return
}

func (c *CCompiler) processMap(mapExpr *ast.MapType) (string, bool) {
	mapKeyCode, okKey, keySuffix := c.processTypeExpression(mapExpr.Key)
	mapValueCode, okMap, valueSuffix := c.processTypeExpression(mapExpr.Value)
	if okKey && okMap {
		mapKeyCode = c.createTypeDef( mapKeyCode + keySuffix )
		mapValueCode = c.createTypeDef( mapValueCode + valueSuffix )
		return fmt.Sprintf("GoMap_(%s,%s)", mapKeyCode, mapValueCode), true
	}
	return "", false
}

func (c *CCompiler) processConstValueExpression(arrayLen ast.Expr) (string, bool) {
	if litExpr, isLit := (arrayLen).(*ast.BasicLit); isLit {
		return litExpr.Value, true
	} else if binExpr, isBinary := (arrayLen).(*ast.BinaryExpr); isBinary {
		leftExpr, okLeft := c.processConstValueExpression(binExpr.X)
		rightExpr, okRight := c.processConstValueExpression(binExpr.Y)
		if okLeft && okRight {
			c_code := fmt.Sprintf("%s %s %s", leftExpr, binExpr.Op, rightExpr)
			return c_code, true
		}
	} else if parensExpr, isParens := (arrayLen).(*ast.ParenExpr); isParens {
		c_code, ok := c.processConstValueExpression(parensExpr.X)
		if ok {
			return "(" + c_code + ")", true
		} 
	} else if identExpr, isIdent := (arrayLen).(*ast.Ident); isIdent {
		return identExpr.Name, true
	} else if selectorExpr, isSelector := (arrayLen).(*ast.SelectorExpr); isSelector {
		identExpr, isIdent := (selectorExpr.X).(*ast.Ident)
		if isIdent {
			return identExpr.Name + package_separator + selectorExpr.Sel.Name, true
		} else {
			applog("Selector with complex expression in array length")
			return "", false
		}
	} else {
		applog("Can't deal with this array len type %s", getTypeOfVar(arrayLen))
	}
	return "", false
}

func (c *CCompiler) processArray(arrayExpr *ast.ArrayType) (string, bool, string) {
	var arrayLenCode string
	var arrayElemCode string
	var ok bool
	var suffix string
	if arrayExpr.Len == nil {
		ok = true
	} else {
		arrayLenCode, ok = c.processConstValueExpression(arrayExpr.Len)
		if !ok {
			applog("Couldn't process array length expression")
		}
	}
	if ok {
		arrayElemCode, ok, suffix = c.processTypeExpression(arrayExpr.Elt)
	} else {
		applog("Couldn't process array length expression")
	}
	if ok {
		if arrayExpr.Len == nil {
			arrayElemCode = c.createTypeDef(arrayElemCode)
			return fmt.Sprintf("GoSlice_(%s) ", arrayElemCode), true, ""
		} else {
			return arrayElemCode, true, fmt.Sprintf("%s[%s]", suffix, arrayLenCode)
		}
	}
	return "", false, ""
}

func (c *CCompiler) processPointer(starExpr *ast.StarExpr) (string, bool) {
	targetTypeExpr := starExpr.X
	code, ok, suffix := c.processTypeExpression(targetTypeExpr)
	if ok {
		return code + suffix + "*", true
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
			code, ok, suffix := c.processTypeExpression(field.Type)
			if ok {
				c_code += code + " " + fieldName + suffix + ";\n"
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

func (c *CCompiler) createIdent(prefix string) string{
	c.identsCount++
	return fmt.Sprintf("%s%d", prefix, c.identsCount)
}

func (c *CCompiler) createTypeDef(typeDefinition string) string{
	if !isComplexType(typeDefinition) {
		return typeDefinition
	} else {
		prefix := c.source.Name.Name + package_separator
	
		for _, typedef := range c.ccode.typedefs {
			if typedef.ccode == typeDefinition {
				return prefix + typedef.name
			}
		}
		
		typeName := c.createIdent("_typeIdent")
		typedef := TypeDef{name: typeName, ccode : typeDefinition}
		c.ccode.typedefs = append( c.ccode.typedefs, typedef)
		return prefix + typedef.name
	}
	
}

func (c *CCompiler) processFunction(fdecl *ast.FuncDecl){
	prefix := c.source.Name.Name + package_separator
	funcName := fdecl.Name.Name
	receiver := c.getFuncReceiverParam(fdecl)
	var parameters []Parameter
	if receiver != nil {
		parameters = append( parameters, *receiver )
		recType := receiver.ctype
		if strings.HasSuffix(recType, "*"){
			recType = recType[:len(recType)-1]
		}
		funcName = recType + "_" + funcName
	}
	funcName = prefix + funcName
	f := Function{name: funcName}
	parameters = append( parameters, c.getFuncParams(fdecl)... )
	resultType := c.getFuncResultType(fdecl)
	f.signature = resultType + " " + funcName + "("
	var paramsCode []string
	for _, p := range parameters {
		paramsCode = append( paramsCode, p.ccode )
	}
	f.signature += strings.Join( paramsCode, ", " )
	f.signature += ")"
	c.ccode.functions = append( c.ccode.functions, f )
}

func (c* CCompiler) getFuncReceiverParam(fdecl *ast.FuncDecl) *Parameter {
	if receiver := fdecl.Recv; receiver != nil {
		_type := &receiver.List[0].Type
		typeName := ""
		if starExpr, _isPointerRecv := (*_type).(*ast.StarExpr); _isPointerRecv {
			_type = &starExpr.X
			typeName = "*"
		}
		if identExpr, isIdent := (*_type).(*ast.Ident); isIdent {
			typeName = identExpr.Name + typeName
		}
		recvParamName := ""
		if len(receiver.List[0].Names) > 0 {
			recvParamName = receiver.List[0].Names[0].Name
		} else {
			recvParamName = c.createIdent("_recv")
		}
		prefix := c.source.Name.Name + package_separator
		ccode := prefix + typeName + " " + recvParamName
		p := Parameter{name : recvParamName, ccode : ccode, ctype : typeName}
		return &p
	}
	return nil
}

func (c *CCompiler) getFuncParams(fdecl *ast.FuncDecl) (parameters []Parameter){
	for index, param := range fdecl.Type.Params.List {
		typeCode, ok, suffix := c.processTypeExpression( param.Type )
		if ok {
			if isComplexTypeForArgument(typeCode){
				typeCode = c.createTypeDef(typeCode + suffix)
				suffix = ""
			}
			var names []string
			for _, name := range param.Names {
				names = append( names, name.Name )
			}
			if len(names) == 0 {
				names = append( names, c.createIdent("_param") )
			}
			for _, name := range names {
				p := Parameter{name : name}
				p.ccode = typeCode + " " + name + suffix
				p.ctype = typeCode + suffix
				parameters = append( parameters, p)
			}
		} else {
			applog("Couldn't process parameter %d in function %s", index, fdecl.Name.Name)
		}
	}
	return
}

func (c *CCompiler) getFuncResultType(fdecl *ast.FuncDecl) (r string) {
	var typeCode string
	var suffix string
	var parameters []Parameter
	var ok bool
	if fdecl.Type.Results != nil {
		for index, param := range fdecl.Type.Results.List {
			typeCode, ok, suffix = c.processTypeExpression( param.Type )
			if ok {
				if isComplexTypeForArgument(typeCode){
					typeCode = c.createTypeDef(typeCode + suffix)
					suffix = ""
				}
				var names []string
				for _, name := range param.Names {
					names = append( names, name.Name )
				}
				if len(names) == 0 {
					names = append( names, c.createIdent("_result") )
				}
				for _, name := range names {
					p := Parameter{name : name}
					p.ccode = typeCode + " " + name + suffix
					p.ctype = typeCode + suffix
					parameters = append( parameters, p)
				}
			} else {
				applog("Couldn't process return parameter %d in function %s", index, fdecl.Name.Name)
			}
		}
	}
	if len(parameters) == 0 {
		r = "void"
	} else if len(parameters) == 1{
		r = typeCode + suffix
	} else {
		r = "struct{\n"
		for _, p := range parameters {
			r += p.ccode + ";\n"
		}
		r += "}\n"
		r = c.createTypeDef(r)
	}
	return 
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

func isComplexType(typeDefinition string) bool{
	return strings.Index(typeDefinition, "*") >= 0 || 
		strings.Index(typeDefinition, "{") >= 0 ||
		strings.Index(typeDefinition, "[") >= 0 ||
		strings.Index(typeDefinition, "(") >= 0 
}

func isComplexTypeForArgument(typeDefinition string) bool{
	return strings.Index(typeDefinition, "{") >= 0 ||
		strings.Index(typeDefinition, "(") >= 0 
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