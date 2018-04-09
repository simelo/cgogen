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
	"strings"
	"reflect"
)

type Config struct {
	Path    			string
	Verbose 			bool
	ProcessFunctions	bool
	ProcessTypes		bool
	OutputFileGO 		string
	OutputFileCH		string
}

func (c *Config) register() {
	flag.StringVar(&c.Path, "i", "", "PATH to source file")
	flag.StringVar(&c.OutputFileGO, "g", "", "PATH to destination file for go code")
	flag.StringVar(&c.OutputFileCH, "h", "", "PATH to destination file for C code")
	flag.BoolVar(&c.Verbose, "v", false, "Print debug message to stdout")
	flag.BoolVar(&c.ProcessFunctions, "f", false, "Process functions")
	flag.BoolVar(&c.ProcessTypes, "t", false, "Process Types")
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
	
func dumpObjectScope(pkg ast.Scope){
	s := reflect.ValueOf(pkg).Elem()
	typeOfT := s.Type()
	fmt.Println(typeOfT)
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		fmt.Printf("Field %d: %s %s = %v\n", i,
			typeOfT.Field(i).Name, f.Type(), f.Interface())
	}
}

func dumpObject(pkg ast.Object){
	s := reflect.ValueOf(pkg).Elem()
	typeOfT := s.Type()
	fmt.Println(typeOfT)
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		fmt.Printf("Field %d: %s %s = %v\n", i,
			typeOfT.Field(i).Name, f.Type(), f.Interface())
	}
}
	
func dumpVar(decl ast.Expr){
	s := reflect.ValueOf(decl).Elem()
	typeOfT := s.Type()
	fmt.Println(typeOfT)
	for i := 0; i < s.NumField(); i++ {
		f := s.Field(i)
		fmt.Printf("Field %d: %s %s = %v\n", i,
			typeOfT.Field(i).Name, f.Type(), f.Interface())
	}
	if identExpr, isIdent := (decl).(*ast.Ident); isIdent {
		if identExpr.Obj != nil {
			fmt.Println("ObjName: ", identExpr.Obj.Name)
			fmt.Println("Type: ", identExpr.Obj.Type)
		}
	}
}

var arrayTypes = []string{
	"PubKey", "SHA256", "Sig", "SecKey", "Ripemd160", 
}	

/*These types will be converted using inplace functions*/
var inplaceConvertTypes = []string{
	"PubKeySlice", "Address",
}
	
var return_var_name = "____error_code"
var return_err_name = "____return_err"
var deal_out_string_as_gostring = true
var get_package_path_from_file_name = true

func main() {
	cfg.register()
	flag.Parse()

	if cfg.Verbose {
		applog = log.Printf
	}

	applog("Opening %v \n", cfg.Path)
	fo, err := os.Open(cfg.Path)
	check(err)

	defer fo.Close()

	fset := token.NewFileSet()
	fast, err := parser.ParseFile(fset, "", fo, parser.AllErrors)
	check(err)

	outFile := jen.NewFile("main")
	
	outFile.CgoPreamble(`
  #include <string.h>
  #include <stdlib.h>
  
  #include "../../include/skytypes.h"`)

	
	typeDefs := make ( [](*ast.GenDecl), 0 )
	for _, _decl := range fast.Decls {
		if cfg.ProcessFunctions {
			if decl, ok := (_decl).(*ast.FuncDecl); ok {
				processFunc(fast, decl, outFile)
			} 
		}
		if cfg.ProcessTypes {
			if decl, ok := (_decl).(*ast.GenDecl); ok {
				if decl.Tok == token.TYPE {
					typeDefs = append ( typeDefs, decl )
				}
			}
		}
	}
	if cfg.ProcessTypes {
		typeDefsCode := processTypeDefs(fast, typeDefs)
		if cfg.OutputFileCH != "" {
			f, err := os.Create(cfg.OutputFileCH)
			check(err)
			defer f.Close()
			f.WriteString( typeDefsCode )
			f.Sync()
		} else {
			fmt.Println(typeDefsCode)
		}
	}
	if cfg.ProcessFunctions {
		if cfg.OutputFileGO != "" {
			err := outFile.Save(cfg.OutputFileGO)
			check(err)
		} else {
			fmt.Printf("%#v", outFile)
		}
	}
	applog("Finished %v", cfg.Path) 
}

func check(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(0)
	}
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
		if ellipsisExpr, isEllipsis := (*_typeExpr).(*ast.Ellipsis); isEllipsis {
			spec += "..." + typeSpecStr(&ellipsisExpr.Elt)
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
			if spec == "" && !addPointer && (isExported) {
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

func resultName(name string) string {
	return "__" + name
}

func getPackagePath(filePath string) string {
	packagePath := ""
	folders := strings.Split(filePath, "/")
	if len(folders) > 0 {
		fileName := folders[len(folders) - 1]
		packageFolders := strings.Split(fileName, ".")
		if len(packageFolders) > 2 {
			packageFolders = packageFolders[:len(packageFolders)-2]
			var result []string
			for _, s := range packageFolders {
				if s == "internal" || s == "example" {
					break
				} else {
					result = append( result, s)
				}
			}
			packagePath = strings.Join(result, "/")
		}
	}
	return packagePath
}

func processFunc(fast *ast.File, fdecl *ast.FuncDecl, outFile *jen.File) {
	packagePath := ""
	if get_package_path_from_file_name {
		packagePath = getPackagePath(cfg.Path)
	}
	if packagePath == "" {
		packagePath = fast.Name.Name
	}

	funcName := fdecl.Name.Name

	if !fdecl.Name.IsExported() {
		applog("Skipping %v \n", funcName)
		return
	}

	applog("Processing %v \n", funcName)
	var blockParams []jen.Code
	
	blockParams = append( blockParams, jen.Id(return_var_name).Op("=").Lit(0) )
	//call_catch_panic_code := jen.Id(return_var_name).Op("=").Id("catchApiPanic").Call(jen.Id("recover").Call())
	call_catch_panic_code := jen.Id(return_var_name).Op("=").Id("catchApiPanic").Call(jen.Id(return_var_name), jen.Id("recover").Call())
	blockParams = append( blockParams, jen.Defer().Func().Params().Block(call_catch_panic_code).Call() )
	
	var params jen.Statement
	var isPointerRecv bool
	if receiver := fdecl.Recv; receiver != nil {
		// Method
		_type := &receiver.List[0].Type
		typeName := ""
		if starExpr, _isPointerRecv := (*_type).(*ast.StarExpr); _isPointerRecv {
			_type = &starExpr.X
			isPointerRecv = _isPointerRecv
		}
		if identExpr, isIdent := (*_type).(*ast.Ident); isIdent {
			typeName = identExpr.Name
		}
		recvParamName := receiver.List[0].Names[0].Name
		recvParam := jen.Id(argName(recvParamName))
		recvParam = recvParam.Id(typeSpecStr(_type))
		params = append(params, recvParam)
		funcName = typeName + "_" + funcName
		convertCode := getCodeToConvertInParameter(_type, recvParamName, isPointerRecv)
		if convertCode != nil {
			blockParams = append( blockParams, convertCode )
		}
	}

	cfuncName := "SKY_" + fast.Name.Name + "_" + funcName
	stmt := outFile.Comment("export " + cfuncName)
	stmt = outFile.Func().Id(cfuncName)

	allparams := fdecl.Type.Params.List[:]
	return_fields_index := len(allparams)
	var retField *ast.Field = nil
	
	if fdecl.Type.Results != nil && fdecl.Type.Results.List != nil {
		//Find the return argument of type error.
		//It should always be the last argument but search just in case
		error_index := -1
		for index, field := range fdecl.Type.Results.List {
			identExpr, isIdent := (field.Type).(*ast.Ident)
			if isIdent && identExpr.Name == "error" {
				error_index = index
				break
			}
		}
		if error_index >= 0 {
			retField = fdecl.Type.Results.List[error_index]
			return_params := append(fdecl.Type.Results.List[0:error_index], fdecl.Type.Results.List[error_index+1:]...)
			allparams = append(allparams, return_params...)
		} else {
			allparams = append(allparams, fdecl.Type.Results.List[:]...)
		}
	}

	var output_vars_convert_code []jen.Code
	
	for fieldIdx, field := range allparams {
		if fieldIdx >= return_fields_index {
			// Field in return types list
			typeName := typeSpecStr(&field.Type)
			if rune(typeName[0]) == '[' {
				typeName = "*C.GoSlice_"
			} else if(deal_out_string_as_gostring && typeName == "string") {
				typeName = "*C.GoString_"
			} else if isBasicGoType(typeName) {
				typeName = "*" + typeName
			}
			paramName := argName("arg"+fmt.Sprintf("%d", fieldIdx))
			params = append(params, jen.Id(paramName).Id(typeName))
			convertCode := getCodeToConvertOutParameter(&field.Type, paramName, false)
			if convertCode != nil {
				output_vars_convert_code = append( output_vars_convert_code, convertCode )
			}
			
		} else {
			lastNameIdx := len(field.Names) - 1
			for nameIdx, ident := range field.Names {
				if nameIdx != lastNameIdx {
					params = append(params, jen.Id(argName(ident.Name)))
				} else {
					typeName := typeSpecStr(&field.Type)
					if rune(typeName[0]) == '[' {
						typeName = "*C.GoSlice_"
					}
					params = append(params, jen.Id(
						argName(ident.Name)).Id(typeName))
				}
				convertCode := getCodeToConvertInParameter(&field.Type, ident.Name, false)
				if convertCode != nil {
					blockParams = append( blockParams, convertCode )
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
	var retvars []jen.Code
	if return_fields_index < len(allparams) {
		for i := return_fields_index; i < len(allparams); i++ {
			retvars = append(retvars, jen.Id(resultName("arg"+fmt.Sprintf("%d", i))))
		}		
	}
	if retField != nil {
		retvars = append(retvars, jen.Id(return_err_name))
	}
	var call_func_code jen.Code
	if len(retvars) > 0 {
		if fdecl.Recv != nil {
			call_func_code = 
				jen.List(retvars...).Op(":=").Id(fdecl.Recv.List[0].Names[0].Name).Dot(fdecl.Name.Name).Call(callparams...)
		} else {
			call_func_code = 
				jen.List(retvars...).Op(":=").Qual("github.com/skycoin/skycoin/src/" + packagePath,
					fdecl.Name.Name).Call(callparams...)
		}
	} else {
		if fdecl.Recv != nil {
			call_func_code = jen.Id(fdecl.Recv.List[0].Names[0].Name).Dot(fdecl.Name.Name).Call(callparams...)
		} else {
			call_func_code = jen.Qual("github.com/skycoin/skycoin/src/" + packagePath,
					fdecl.Name.Name).Call(callparams...)
		}
	}
	blockParams = append(blockParams, call_func_code,)
	
	stmt = stmt.Parens(jen.Id(return_var_name).Id("uint32"))
	if retField != nil {
		blockParams = append(blockParams, jen.Id(return_var_name).Op("=").Id("libErrorCode").Call(jen.Id(return_err_name)))
		convertOutputCode := jen.If(jen.Id(return_err_name).Op("==").Nil()).Block(output_vars_convert_code...)
		blockParams = append(blockParams, convertOutputCode)
	} else {
		blockParams = append(blockParams, output_vars_convert_code...)
	}
	
	blockParams = append(blockParams, jen.Return())
	
	stmt.Block(blockParams...)
}

/*Returns jen code to convert an input parameter from wrapper to original function*/
func getCodeToConvertInParameter(_typeExpr *ast.Expr, name string, isPointer bool) jen.Code{
	if arrayExpr, isArray := (*_typeExpr).(*ast.ArrayType); isArray {
		typeExpr := arrayExpr.Elt
		if identExpr, isIdent := (typeExpr).(*ast.Ident); isIdent {
			typeName := identExpr.Name
			if isBasicGoType(typeName) {
				return jen.Id(name).Op(":=").Op("*").Parens(jen.Op("*").Op("[]").Id(identExpr.Name)).
						Parens(jen.Qual("unsafe", "Pointer").Parens(jen.Id(argName(name))))
			} else {
				return jen.Id(name).Op(":=").Op("*").Parens(jen.Op("*").Op("[]").Id(identExpr.Name)).
						Parens(jen.Qual("unsafe", "Pointer").Parens(jen.Id(argName(name))))
			}
		}
	} else if starExpr, isPointerParam := (*_typeExpr).(*ast.StarExpr); isPointerParam {
		_type := &starExpr.X
		return getCodeToConvertInParameter(_type, name, true)
	} else if identExpr, isIdent := (*_typeExpr).(*ast.Ident); isIdent {
		typeName := identExpr.Name
		if isBasicGoType(typeName) {
			return jen.Id(name).Op(":=").Id(argName(name))
		} else if isInplaceConvertType(typeName) {
			if isPointer {
				return jen.Id(name).Op(":=").Id("inplace"+typeName).Call(jen.Id(argName(name)));
			} else {
				return jen.Id(name).Op(":=").Op("*").Id("inplace"+typeName).Call(jen.Id(argName(name)));
			}
		} else {
			if isPointer {
				return jen.Id(name).Op(":=").Parens(jen.Op("*").Id(typeName)).
						Parens( jen.Qual("unsafe", "Pointer").Parens(jen.Id(argName(name))) )
			} else {
				return jen.Id(name).Op(":=").Op("*").Parens(jen.Op("*").Id(typeName)).
						Parens( jen.Qual("unsafe", "Pointer").Parens(jen.Id(argName(name))) )
			}
		}
	
	} else if _, isEllipsis := (*_typeExpr).(*ast.Ellipsis); isEllipsis {
		return jen.Id(name).Op(":=").Id(argName(name))
		/*typeExpr := ellipsisExpr.Elt
		if identExpr, isIdent := (typeExpr).(*ast.Ident); isIdent {
			typeName := identExpr.Name
			if isBasicGoType(typeName) {
				return jen.Id(name).Op(":=").Id(argName(name))
			} else {
			}
		} */
	}
	return nil
}

/*Returns jen Code to convert an output parameter from original to wrapper function*/
func getCodeToConvertOutParameter(_typeExpr *ast.Expr, name string, isPointer bool) jen.Code{
	if _, isArray := (*_typeExpr).(*ast.ArrayType); isArray {
		return jen.Id("copyToGoSlice").Call(jen.Qual("reflect", "ValueOf").Call(jen.Id(argName(name))),
			jen.Id(name))
	} else if starExpr, isPointerRecv := (*_typeExpr).(*ast.StarExpr); isPointerRecv {
		_type := &starExpr.X
		return getCodeToConvertOutParameter(_type, name, true)
	} else if identExpr, isIdent := (*_typeExpr).(*ast.Ident); isIdent {
		typeName := identExpr.Name
		if deal_out_string_as_gostring && typeName == "string" {
			return jen.Id("copyString").Call(jen.Id(argName(name)), jen.Id(name))
		} else if isBasicGoType(typeName) {
			return jen.Op("*").Id(name).Op("=").Id(argName(name))
		} else if isInplaceConvertType(typeName) {
			if isPointer {
				return jen.Id(name).Op("=").Op("*").Parens(jen.Op("*").
					Qual("C",typeName)).
						Parens( jen.Qual("unsafe", "Pointer").Parens(jen.Id(argName(name))) )
			} else {
				return jen.Id(name).Op("=").Parens(jen.Op("*").
					Qual("C",typeName)).
						Parens( jen.Qual("unsafe", "Pointer").Parens(jen.Op("&").Id(argName(name))) )
			}
		} else {
			if isPointer {
				return jen.Id("copyToBuffer").Call(jen.Qual("reflect", "ValueOf").Call(jen.Parens( jen.Op("*").Id(argName(name)) ).Op("[:]")),
							jen.Qual("unsafe", "Pointer").Call(jen.Id(name)),			
							jen.Id("uint").Parens(jen.Id("Sizeof" + typeName)))
			} else {
				return jen.Id("copyToBuffer").Call(jen.Qual("reflect", "ValueOf").Call(jen.Id(argName(name)).Op("[:]")),
						jen.Qual("unsafe", "Pointer").Call(jen.Id(name)), 
						jen.Id("uint").Parens(jen.Id("Sizeof" + typeName)))
			}
		}
	}
	return nil
}

/* Returns the corresponding C type for a GO type*/
func goTypeToCType(goType string) string {
	if val, ok := typesMap[goType]; ok {
		return val
	} else {
		return goType
	}
}

func isBasicGoType(goType string) bool {
	if _, ok := typesMap[goType]; ok {
		return true
	} else {
		return false
	}
}

func isSkyArrayType(typeName string) bool {
	for _, t := range arrayTypes {
		if t == typeName {
			return true
		}
	}
	return false
}

func isInplaceConvertType(typeName string) bool {
	for _, t := range inplaceConvertTypes {
		if t == typeName {
			return true
		}
	}
	return false
}


/* Process a type expression. Returns the code in C for the type and ok if successfull */
func processTypeExpression(fast *ast.File, type_expr ast.Expr, name string, 
							defined_types *[]string, 
							forwards_declarations *[]string, depth int) (string, bool) {
	c_code := ""
	result := false
	
	if typeStruct, isTypeStruct := (type_expr).(*ast.StructType); isTypeStruct {
		c_code += "struct{\n"
		error := false
		for _, field := range typeStruct.Fields.List{
			for _, fieldName := range field.Names{
				for i := 0; i < depth * 4; i++{
					c_code += " "
				}
				type_code, result := processTypeExpression(fast, field.Type, fieldName.Name, defined_types, forwards_declarations, depth + 1)
				if result {
					c_code += type_code
				} else {
					error = true
				}
			}
			c_code += ";\n"
			
		}
		for i := 0; i < (depth - 1) * 4; i++{
			c_code += " "
		}
		c_code += "} " + name
		result = !error
	}else if arrayExpr, isArray := (type_expr).(*ast.ArrayType); isArray {
		if arrayExpr.Len == nil {
			c_code += "GoSlice_ " + name
		} else if litExpr, isLit := (arrayExpr.Len).(*ast.BasicLit); isLit {
			arrayElCode, result := processTypeExpression(fast, arrayExpr.Elt, "", defined_types, forwards_declarations, depth)
			if result {
				c_code += arrayElCode + " " + name+"[" + litExpr.Value + "]"
			}
		}
		result = true
	}else if _, isFunc := (type_expr).(*ast.FuncType); isFunc {
		c_code += "Handle " + name
		result = true
	}else if _, isIntf := (type_expr).(*ast.InterfaceType); isIntf {
		c_code += "GoInterface_ " + name
		result = true
	}else if _, isChan := (type_expr).(*ast.ChanType); isChan {
		c_code += "GoChan_ " + name
		result = true
	}else if _, isMap := (type_expr).(*ast.MapType); isMap {
		c_code += "GoMap_ " + name
		result = true
	}else if starExpr, isStart := (type_expr).(*ast.StarExpr); isStart {
		targetTypeExpr := starExpr.X
		type_code, ok := processTypeExpression(fast, targetTypeExpr, "", defined_types, forwards_declarations, depth + 1)
		if ok {
			c_code += type_code
			c_code += "* "  + name
			result = true
		}
	}else if identExpr, isIdent := (type_expr).(*ast.Ident); isIdent {
		c_code = goTypeToCType(identExpr.Name) + " " + name
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
			type_c_code, ok := processTypeExpression(fast, typeSpec.Type, typeSpec.Name.Name, defined_types, forwards_declarations, 1)
			if ok {
				result_code += "typedef "
				result_code += type_c_code
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
	//TODO: if unprocessed > 0 then there are cyclic type references. Use forward declarations.
	return result_code
}

