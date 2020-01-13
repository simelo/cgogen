package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"strings"
)

type CCompiler struct {
	source       *ast.File
	ccode        *CCode
	includes     []string
	currentType  *TypeDef
	identsCount  int
	defStack     []*CCode
	imports      map[string]*CCode
	initializers []string
}

type CCode struct {
	typedefs  []TypeDef
	constdefs []ConstDef
	vardefs   []VarDef
	forwards  []string
	functions map[string]*Function
}

type ConstDef struct {
	name  string
	ccode string
	ctype string
}

type VarDef struct {
	name  string
	ccode string
	ctype string
}

type TypeDef struct {
	originalName string
	packageName  string
	name         string
	ccode        string
	dependencies []string
	defType      string // struct, map, whatever this typedef represents
	// used for forwards
}

type Function struct {
	originalName string
	packageName  string
	name         string
	body         string
	parameters   []Parameter
	returnType   string
}

type Parameter struct {
	name  string
	ccode string
	ctype string
}

func NewCompiler() (compiler *CCompiler) {
	compiler = &CCompiler{}
	compiler.ccode = &CCode{}
	compiler.ccode.functions = make(map[string]*Function)
	compiler.defStack = append(compiler.defStack, compiler.ccode)
	return
}

func (c *CCompiler) GetHeaderCode() (header string) {
	header = ""
	header += "#pragma once\n"
	for _, include := range c.includes {
		header += fmt.Sprintf("#include \"%s\"\n", include)
	}

	header += "\n\n"

	for _, constDef := range c.ccode.constdefs {
		header += constDef.ccode + "\n"
	}

	header += "\n\n"

	for _, varDef := range c.ccode.vardefs {
		header += varDef.ccode + "\n"
	}

	header += "\n\n"

	for _, f := range c.ccode.forwards {
		header += f + ";\n"
	}

	header += "\n\n"

	//prefix := c.source.Name.Name + package_separator
	for _, typedef := range c.ccode.typedefs {
		header += "typedef " +
			buildTypeWithVarName(typedef.ccode,
				c.source.Name.Name+packageSeparator+typedef.name) + ";\n"
	}

	header += "\n\n"
	for _, funcDef := range c.ccode.functions {
		header += c.createSignature(funcDef) + ";\n"
	}

	return
}

func (c *CCompiler) GetCCode() (code string) {
	code = fmt.Sprintf("#include \"%s.h\"\n", c.source.Name.Name)
	code += "\n\n"
	for _, funcDef := range c.ccode.functions {
		code += c.createSignature(funcDef)
		code += funcDef.body + "\n"
	}
	return
}

func (c *CCompiler) Compile(source *ast.File) {
	c.source = source
	c.processPrototypes()
	c.processImplementation()
}

func (c *CCompiler) pushStack() {
	c.defStack = append(c.defStack, &CCode{})
}

func (c *CCompiler) popStack() {
	if len(c.defStack) > 0 {
		c.defStack = c.defStack[:len(c.defStack)-1]
	} else {
		reportError("Poping empty stack")
	}
}

func (c *CCompiler) getTopOfStack() *CCode {
	if len(c.defStack) > 0 {
		return c.defStack[len(c.defStack)-1]
	} else {
		reportError("Poping empty stack")
		return nil
	}
}

func (c *CCompiler) processImplementation() {
	for _, _decl := range c.source.Decls {
		if decl, ok := (_decl).(*ast.FuncDecl); ok {
			c.processFunctionBody(decl)
		}
	}
}

func (c *CCompiler) processPrototypes() {
	for _, _decl := range c.source.Decls {
		if decl, ok := (_decl).(*ast.FuncDecl); ok {
			c.processFunctionPrototype(decl)
		} else if decl, ok := (_decl).(*ast.GenDecl); ok {
			c.processDeclaration(decl)
		} else {
			c.processUnknown(_decl)
		}
	}
	c.processDependencies()
}

func (c *CCompiler) processDeclaration(decl *ast.GenDecl) {
	if decl.Tok == token.TYPE {
		c.processType(decl)
	} else if decl.Tok == token.IMPORT {
		c.processImport(decl)
	} else if decl.Tok == token.CONST {
		c.processConstExpression(decl)
	} else if decl.Tok == token.VAR {
		c.processVarExpression(decl)
	} else {
		//Handle in implementation
	}
}

func (c *CCompiler) processDependencies() {
	var orderedTypedefs []TypeDef
	removed := map[int]bool{}
	var noTypeAdded bool
	for !noTypeAdded {
		noTypeAdded = true
		for index, typeDef := range c.ccode.typedefs {
			is_removed, ok := removed[index]
			if !ok || !is_removed {
				all_deps_in := true
				for _, dep := range typeDef.dependencies {
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
					orderedTypedefs = append(orderedTypedefs, typeDef)
					noTypeAdded = false
					removed[index] = true
				}
			}
		}
	}
	prefix := c.source.Name.Name + packageSeparator
	for index, typeDef := range c.ccode.typedefs {
		is_removed, ok := removed[index]
		if !ok || !is_removed {
			if typeDef.defType == "struct" {

				f := "struct " + prefix + typeDef.name
				c.ccode.forwards = append(c.ccode.forwards, f)
			}
			orderedTypedefs = append(orderedTypedefs, typeDef)
		}
	}
	c.ccode.typedefs = orderedTypedefs
}

func getTypeOfVar(decl ast.Expr) string {
	s := reflect.ValueOf(decl).Elem()
	return fmt.Sprintf("%s", s.Type())
}

func (c *CCompiler) processConstExpression(decl *ast.GenDecl) {
	c.generateDeclaration(*decl)
}

func (c *CCompiler) processVarExpression(decl *ast.GenDecl) {
	c.generateDeclaration(*decl)
}

func (c *CCompiler) processUnknown(decl ast.Decl) {
	s := reflect.ValueOf(decl).Elem()
	typeOfT := s.Type()
	reportError("Don't know what to do with: %s", typeOfT)
}

func (c *CCompiler) processImport(decl *ast.GenDecl) {

}

func (c *CCompiler) processType(tdecl *ast.GenDecl) {
	for _, s := range tdecl.Specs {
		if typeSpec, isTypeSpec := (s).(*ast.TypeSpec); isTypeSpec {
			typedef := TypeDef{
				name:         typeSpec.Name.Name,
				originalName: typeSpec.Name.Name,
				packageName:  c.source.Name.Name}
			typedef.defType = ""
			c.currentType = &typedef
			typeName := typeSpec.Name.Name
			code, ok := c.processTypeExpression(typeSpec.Type)
			if ok {
				typedef.name = typeName
				typedef.ccode = code
				c.ccode.typedefs = append(c.ccode.typedefs, typedef)
			}
			c.currentType = nil
		} else {
			c.processUnknown(tdecl)
		}
	}
}

func (c *CCompiler) processTypeExpression(type_expr ast.Expr) (code string, result bool) {
	code = ""
	result = false
	if typeStruct, isTypeStruct := (type_expr).(*ast.StructType); isTypeStruct {
		code, result = c.processStructType(typeStruct)
	} else if identExpr, isIdent := (type_expr).(*ast.Ident); isIdent {
		code, result = c.processIdentifier(identExpr)
	} else if selectorExpr, isSelector := (type_expr).(*ast.SelectorExpr); isSelector {
		code, result = c.processSelector(selectorExpr)
	} else if starExpr, isStart := (type_expr).(*ast.StarExpr); isStart {
		code, result = c.processPointer(starExpr)
	} else if arrayExpr, isArray := (type_expr).(*ast.ArrayType); isArray {
		code, result = c.processArray(arrayExpr)
	} else if mapExpr, isMap := (type_expr).(*ast.MapType); isMap {
		code, result = c.processMap(mapExpr)
	} else {
		x := reflect.ValueOf(type_expr).Elem()
		typeOfT := x.Type()
		reportError("Unknown type: %s", typeOfT)
	}
	return
}

func (c *CCompiler) processMap(mapExpr *ast.MapType) (string, bool) {
	_, okKey := c.processTypeExpression(mapExpr.Key)
	mapValueCode, okMap := c.processTypeExpression(mapExpr.Value)
	if okKey && okMap {
		//mapKeyCode = getMapTypeKeyword(  mapKeyCode )
		mapValueCode = getMapTypeKeyword(mapValueCode)
		return fmt.Sprintf("Go%sMap", mapValueCode), true
	}
	return "", false
}

func getMapTypeKeyword(typeName string) string {
	if typeName == "GoInt_" || typeName == "GoUint_" ||
		typeName == "GoInt16_" || typeName == "GoUint16_" ||
		typeName == "GoInt32_" || typeName == "GoUint32_" ||
		typeName == "GoInt64_" || typeName == "GoUint64_" {
		return "Int"
	} else if typeName == "GoFloat32_" || typeName == "GoFloat64_" {
		return "Float"
	} else if typeName == "GoString_" {
		return "String"
	} else {
		return "Object"
	}
}

func (c *CCompiler) processIntegerConstExpression(expr ast.Expr, isForArray bool) (string, bool) {
	if litExpr, isLit := (expr).(*ast.BasicLit); isLit {
		if litExpr.Kind == token.INT {
			return litExpr.Value, true
		} else {
			if isForArray {
				reportError("Array length must be integer")
			}
			return "", false
		}
	} else if binExpr, isBinary := (expr).(*ast.BinaryExpr); isBinary {
		leftExpr, okLeft := c.processIntegerConstExpression(binExpr.X, isForArray)
		rightExpr, okRight := c.processIntegerConstExpression(binExpr.Y, isForArray)
		if okLeft && okRight {
			c_code := fmt.Sprintf("%s %s %s", leftExpr, binExpr.Op, rightExpr)
			return c_code, true
		}
	} else if parensExpr, isParens := (expr).(*ast.ParenExpr); isParens {
		c_code, ok := c.processIntegerConstExpression(parensExpr.X, isForArray)
		if ok {
			return "(" + c_code + ")", true
		}
	} else if identExpr, isIdent := (expr).(*ast.Ident); isIdent {
		//TODO: Check if this an integer constant
		return identExpr.Name, true
	} else if selectorExpr, isSelector := (expr).(*ast.SelectorExpr); isSelector {
		//TODO: Check if this an integer constant
		identExpr, isIdent := (selectorExpr.X).(*ast.Ident)
		if isIdent {
			return identExpr.Name + packageSeparator + selectorExpr.Sel.Name, true
		} else {
			if isForArray {
				reportError("Selector with complex expression in array length")
			}
			return "", false
		}
	} else {
		if isForArray {
			reportError("Can't deal with this array len type %s", getTypeOfVar(expr))
		}
	}
	return "", false
}

func (c *CCompiler) processArray(arrayExpr *ast.ArrayType) (string, bool) {
	var arrayLenCode string
	var arrayElemCode string
	var ok bool
	if arrayExpr.Len == nil {
		ok = true
	} else {
		arrayLenCode, ok = c.processIntegerConstExpression(arrayExpr.Len, true)
		if !ok {
			reportError("Couldn't process array length expression")
		}
	}
	if ok {
		arrayElemCode, ok = c.processTypeExpression(arrayExpr.Elt)
	} else {
		reportError("Couldn't process array length expression")
	}
	if ok {
		if arrayExpr.Len == nil {
			arrayElemCode = c.createTypeDef(arrayElemCode)
			return fmt.Sprintf("GoSlice_(%s) ", arrayElemCode), true
		} else {
			result := buildTypeWithVarName(arrayElemCode, fmt.Sprintf("[[[__]]][%s]", arrayLenCode))
			return result, true
		}
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
		return identExpr.Name + packageSeparator + selectorExpr.Sel.Name, true
	} else {
		reportError("Selector with complex expression")
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
			c.currentType.dependencies = append(c.currentType.dependencies,
				type_code)
		}
		return c.source.Name.Name + packageSeparator + type_code, true
	}
}

func (c *CCompiler) processStructType(typeStruct *ast.StructType) (string, bool) {
	c_code := "struct{\n"
	for _, field := range typeStruct.Fields.List {
		var names []string
		for _, fieldName := range field.Names {
			names = append(names, fieldName.Name)
		}
		if len(names) == 0 {
			names = append(names, "_unnamed")
		}
		for _, fieldName := range names {
			code, ok := c.processTypeExpression(field.Type)
			if ok {
				c_code += buildTypeWithVarName(code, fieldName) + ";\n"
			} else {
				reportError("Couldn't process %s", field.Type)
			}
		}
	}
	c_code += "}"
	if c.currentType != nil && c.currentType.defType == "" {
		c.currentType.defType = "struct"
	}
	return c_code, true
}

func (c *CCompiler) createIdent(prefix string) string {
	c.identsCount++
	return fmt.Sprintf("%s%d", prefix, c.identsCount)
}

func (c *CCompiler) createTypeDef(typeDefinition string) string {
	if !isComplexType(typeDefinition) {
		return typeDefinition
	} else {
		prefix := c.source.Name.Name + packageSeparator

		for _, typedef := range c.ccode.typedefs {
			if typedef.ccode == typeDefinition {
				return prefix + typedef.name
			}
		}

		typeName := c.createIdent("_typeIdent")
		typedef := TypeDef{name: typeName, ccode: typeDefinition}
		c.ccode.typedefs = append(c.ccode.typedefs, typedef)
		return prefix + typedef.name
	}

}

func (c *CCompiler) processFunctionBody(fdecl *ast.FuncDecl) {
	packageName := c.source.Name.Name
	funcName := fdecl.Name.Name
	function, found := c.ccode.functions[packageName+"."+funcName]
	if found {
		function.body = c.generateBody(function, fdecl.Body)
	} else {
		reportError("Function name %s.%s not found to generate body", packageName, funcName)
	}
}

func (c *CCompiler) processFunctionPrototype(fdecl *ast.FuncDecl) {
	prefix := c.source.Name.Name + packageSeparator
	funcName := fdecl.Name.Name
	receiver := c.getFuncReceiverParam(fdecl)
	var parameters []Parameter
	if receiver != nil {
		parameters = append(parameters, *receiver)
		recType := receiver.ctype
		if strings.HasSuffix(recType, "*") {
			recType = recType[:len(recType)-1]
		}
		funcName = recType + "_" + funcName
	}
	funcName = prefix + funcName
	f := Function{name: funcName,
		originalName: fdecl.Name.Name,
		packageName:  c.source.Name.Name}
	parameters = append(parameters, c.getFuncParams(fdecl)...)
	resultType := c.getFuncResultType(fdecl)
	f.parameters = parameters
	f.returnType = resultType
	c.ccode.functions[f.packageName+"."+f.originalName] = &f
}

func (c *CCompiler) getFuncReceiverParam(fdecl *ast.FuncDecl) *Parameter {
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
		prefix := c.source.Name.Name + packageSeparator
		ccode := prefix + typeName + " " + recvParamName
		p := Parameter{name: recvParamName, ccode: ccode, ctype: typeName}
		return &p
	}
	return nil
}

func (c *CCompiler) getFuncParams(fdecl *ast.FuncDecl) (parameters []Parameter) {
	for index, param := range fdecl.Type.Params.List {
		typeCode, ok := c.processTypeExpression(param.Type)
		if ok {
			if isComplexTypeForArgument(typeCode) {
				typeCode = c.createTypeDef(typeCode)
			}
			var names []string
			for _, name := range param.Names {
				names = append(names, name.Name)
			}
			if len(names) == 0 {
				names = append(names, c.createIdent("_param"))
			}
			for _, name := range names {
				p := Parameter{name: name}
				p.ccode = buildTypeWithVarName(typeCode, name)
				p.ctype = buildTypeWithVarName(typeCode, "")
				parameters = append(parameters, p)
			}
		} else {
			reportError("Couldn't process parameter %d in function %s", index+1, fdecl.Name.Name)
		}
	}
	return
}

func (c *CCompiler) getFuncResultType(fdecl *ast.FuncDecl) (r string) {
	var typeCode string
	var parameters []Parameter
	var ok bool
	if fdecl.Type.Results != nil {
		for index, param := range fdecl.Type.Results.List {
			typeCode, ok = c.processTypeExpression(param.Type)
			if ok {
				if isComplexTypeForArgument(typeCode) {
					typeCode = c.createTypeDef(typeCode)
				}
				var names []string
				for _, name := range param.Names {
					names = append(names, name.Name)
				}
				if len(names) == 0 {
					names = append(names, c.createIdent("_result"))
				}
				for _, name := range names {
					p := Parameter{name: name}
					p.ccode = buildTypeWithVarName(typeCode, name)
					p.ctype = buildTypeWithVarName(typeCode, "")
					parameters = append(parameters, p)
				}
			} else {
				reportError("Couldn't process return parameter %d in function %s", index, fdecl.Name.Name)
			}
		}
	}
	if len(parameters) == 0 {
		r = "void"
	} else if len(parameters) == 1 {
		r = typeCode
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

func isComplexType(typeDefinition string) bool {
	return strings.Index(typeDefinition, "*") >= 0 ||
		strings.Index(typeDefinition, "{") >= 0 ||
		strings.Index(typeDefinition, "[") >= 0 ||
		strings.Index(typeDefinition, "(") >= 0
}

func isComplexTypeForArgument(typeDefinition string) bool {
	return strings.Index(typeDefinition, "{") >= 0 ||
		strings.Index(typeDefinition, "(") >= 0
}

func (c *CCompiler) findFunction(name string, packageName string) *Function {
	for i := len(c.defStack) - 1; i >= 0; i-- {
		def := c.defStack[i]
		for _, f := range def.functions {
			if f.originalName == name && packageName == "" {
				return f
			}
		}
	}
	if packageName != "" {
		extern, found := c.imports[packageName]
		if found {
			for _, f := range extern.functions {
				if f.originalName == name {
					return f
				}
			}
		}
	}
	return nil
}

func (c *CCompiler) findConst(name string, packageName string) *ConstDef {
	for i := len(c.defStack) - 1; i >= 0; i-- {
		def := c.defStack[i]
		for _, c := range def.constdefs {
			if c.name == name {
				return &c
			}
		}
	}
	if packageName != "" {
		extern, found := c.imports[packageName]
		if found {
			for _, c := range extern.constdefs {
				if c.name == name {
					return &c
				}
			}
		}
	}
	return nil
}

func (c *CCompiler) findVar(name string, packageName string) *VarDef {
	for i := len(c.defStack) - 1; i >= 0; i-- {
		def := c.defStack[i]
		for _, c := range def.vardefs {
			if c.name == name {
				return &c
			}
		}
	}
	if packageName != "" {
		extern, found := c.imports[packageName]
		if found {
			for _, c := range extern.vardefs {
				if c.name == name {
					return &c
				}
			}
		}
	}
	return nil
}

func buildTypeWithVarName(typedef string, varname string) string {
	if strings.Index(typedef, "[[[__]]]") >= 0 {
		return strings.Replace(typedef, "[[[__]]]", varname, -1)
	} else {
		return typedef + " " + varname
	}
}
