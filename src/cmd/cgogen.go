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
	"bytes"
)

type Config struct {
	Path    			string
	Verbose 			bool
	ProcessFunctions	bool
	ProcessTypes		bool
	OutputFileGO 		string
	OutputFileCH		string
	ProcessDependencies bool
	DependOnlyExternal  bool
	TypeDependencyFile	string
	FuncDependencyFile	string
	IgnoreDependants	bool
}

func (c *Config) register() {
	flag.StringVar(&c.Path, "i", "", "PATH to source file")
	flag.StringVar(&c.OutputFileGO, "g", "", "PATH to destination file for go code")
	flag.StringVar(&c.OutputFileCH, "h", "", "PATH to destination file for C code")
	flag.BoolVar(&c.Verbose, "v", false, "Print debug message to stdout")
	flag.BoolVar(&c.ProcessFunctions, "f", false, "Process functions")
	flag.BoolVar(&c.ProcessTypes, "t", false, "Process Types")
	flag.BoolVar(&c.ProcessDependencies, "d", false, "Analyze dependencies")
	flag.BoolVar(&c.DependOnlyExternal, "n", false, "Analyze only dependencies on external libraries")
	flag.StringVar(&c.TypeDependencyFile, "td", "", "PATH to destination file where dependant types will be stored")
	flag.StringVar(&c.FuncDependencyFile, "fd", "", "PATH to destination file where dependant functions will be stored")
	flag.BoolVar(&c.IgnoreDependants, "id", false, "Ignore dependants")
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
	  "error" : "GoInt32_",
	}
	
var inplaceConvertTypesPackages = map[string]string {
	"PubKeySlice" : "cipher", 
	"Address" : "cipher",
	"BalanceResult" : "cli",
}

/*These types will be converted using inplace functions*/
var inplaceConvertTypes = []string{
	"PubKeySlice", "Address", "BalanceResult",
}

var mainPackagePath = string ("github.com/skycoin/skycoin/src/")
//var mainPackagePath = string ("")
	
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



var importDefs [](*ast.GenDecl)

var package_separator = "__"
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
	
	var dependant_functions []string
	var dependant_types []string
	if cfg.ProcessDependencies {
		if cfg.TypeDependencyFile != "" {
			dependant_types = loadDependencyFile(cfg.TypeDependencyFile, "|")
		}
		if cfg.FuncDependencyFile != "" {
			dependant_functions = loadDependencyFile(cfg.FuncDependencyFile, "\r\n")
		}
	}
	applog("Opening %v \n", cfg.Path)
	fo, err := os.Open(cfg.Path)
	check(err)

	defer fo.Close()

	fset := token.NewFileSet()
	fast, err := parser.ParseFile(fset, "", fo, parser.AllErrors)
	check(err)
	
	packagePath := ""
	if get_package_path_from_file_name {
		packagePath = getPackagePathFromFileName(cfg.Path) + "/" + fast.Name.Name
		applog("Package Path: %s " , packagePath)
	}
	if packagePath == "" {
		packagePath = fast.Name.Name
	}

	outFile := jen.NewFile("main")
	
	outFile.CgoPreamble(`
  #include <string.h>
  #include <stdlib.h>
  
  #include "../../include/skytypes.h"`)

	
	typeDefs := make ( [](*ast.GenDecl), 0 )
	
	for _, _decl := range fast.Decls {
		if cfg.ProcessFunctions {
			if decl, ok := (_decl).(*ast.FuncDecl); ok {
				var plist *[]string
				plist = nil
				if cfg.ProcessDependencies {
					plist = &dependant_types
				}
				if isDependant := processFunc(fast, decl, outFile, plist); isDependant {
					addDependant(&dependant_functions, packagePath + " " + decl.Name.Name)
				}
			} 
		}
		if cfg.ProcessTypes {
			if decl, ok := (_decl).(*ast.GenDecl); ok {
				if decl.Tok == token.TYPE {
					typeDefs = append ( typeDefs, decl )
				} else if decl.Tok == token.IMPORT {
					importDefs = append( importDefs, decl )
				}
			}
		}
	}
	if cfg.ProcessTypes {
		typeDefsCode := processTypeDefs(fast, typeDefs, &dependant_types)
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
	if cfg.ProcessDependencies {
		if cfg.TypeDependencyFile != "" {
			saveDependencyFile(cfg.TypeDependencyFile, dependant_types, "|")
			
		} else {
			fmt.Println("Dependant Types: ", dependant_types)
		}
		if cfg.FuncDependencyFile != "" {
			saveDependencyFile(cfg.FuncDependencyFile, dependant_functions, "\r\n")
		} else {
			fmt.Println("Dependant Functions: ", dependant_functions)
		}
	}
	applog("Finished %v", cfg.Path) 
	fixExportComment(cfg.OutputFileGO)
}

func saveDependencyFile(path string, list []string, separator string){
	f, err := os.Create(path)
	check(err)
	defer f.Close()
	f.WriteString( strings.Join( list, separator ) )
	f.Sync()
}

func loadDependencyFile(path string, separator string) (list []string) {
	f, err := os.Open(path)
	if err == nil {
		defer f.Close()
		buf := new(bytes.Buffer)
		buf.ReadFrom(f)
		contents := buf.String()
		list = strings.Split(contents, separator)
	}
	return 
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

func findImportPath(importName string) (string, bool) {
	for _, importDef := range importDefs {
		for _, s := range importDef.Specs{
			if importSpec, isImportSpec := (s).(*ast.ImportSpec); isImportSpec {
				name := ""
				path := importSpec.Path.Value
				
				if strings.HasPrefix( path, "\"") {
					path = path[1:]
				}
				if strings.HasSuffix( path, "\"") {
					path = path[:len(path)-1]
				}
				if importSpec.Name != nil {
					name = importSpec.Name.Name
				} else {
					path_parts := strings.Split(path, "/")
					if len(path_parts) > 0 {
						name = path_parts[ len(path_parts) -1 ]
					}
				}
				if name == importName {
					return path, true
				}
			}
		}
	}
	return "", false
}

func isSkycoinName(importName string) bool {
	path, result := findImportPath(importName)
	if result {
		return strings.HasPrefix(path, "github.com/skycoin/")
	} else {
		return false
	}
}

func isExternalName(importName string) bool {
	path, result := findImportPath(importName)
	if result {
		return strings.HasPrefix(path, "github.com/") || strings.HasPrefix(path, "golang.org/")
	} else {
		return false
	}
}

func typeSpecStr(_typeExpr *ast.Expr, package_name string, isOutput bool) string {
	addPointer := false
	spec := ""
	for _typeExpr != nil {
		if arrayExpr, isArray := (*_typeExpr).(*ast.ArrayType); isArray {
			if arrayExpr.Len != nil || isOutput {
				return "*C.GoSlice_"
			} else {
				spec += "[]"
				_typeExpr = &arrayExpr.Elt
				continue
			}
		}
		if starExpr, isStar := (*_typeExpr).(*ast.StarExpr); isStar {
			spec += "*"
			_typeExpr = &starExpr.X
			continue
		}
		if ellipsisExpr, isEllipsis := (*_typeExpr).(*ast.Ellipsis); isEllipsis {
			spec += "..." + typeSpecStr(&ellipsisExpr.Elt, package_name, isOutput)
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
			spec += "*C.GoInterface_"
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
			return spec + "map[" + typeSpecStr(&mapExpr.Key, package_name, false) + "]" + 
				typeSpecStr(&mapExpr.Value, package_name, false)
		}
		identExpr, isIdent := (*_typeExpr).(*ast.Ident)
		selExpr, isSelector := (*_typeExpr).(*ast.SelectorExpr)
		if isIdent || isSelector {
			extern_package := package_name
			typeName := ""
			if isIdent {
				typeName = identExpr.Name
			} else {
				identSelExpr, isSelIdent := (selExpr.X).(*ast.Ident)
				if isSelIdent {
					extern_package = identSelExpr.Name
					if extern_package != package_name && !isSkycoinName(extern_package){
						extern_package = "_" + extern_package
					}
				}
				typeName = selExpr.Sel.Name
			}
			isExported := !isBasicGoType(typeName)//isAsciiUpper(rune(typeName[0]))
			if spec == "" && !addPointer && isExported {
				addPointer = true
			}
			if isExported {
				spec += "C." + extern_package + package_separator
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

/*
Get the package path from file name. Assumes that file is formed by joining path folders with dot
Example: pack.folder1.folder2  ==>  pack/folder1/folder2
*/
func getPackagePathFromFileName(filePath string) string {
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

func processFunc(fast *ast.File, fdecl *ast.FuncDecl, outFile *jen.File, dependant_types *[]string) (isDependant bool) {
	isDependant = false
	packagePath := ""
	if get_package_path_from_file_name {
		packagePath = getPackagePathFromFileName(cfg.Path)
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
		typeSpec := typeSpecStr(_type, fast.Name.Name, false)
		if isTypeSpecInDependantList( typeSpec, dependant_types ) {
			isDependant = true
			if cfg.IgnoreDependants {
				return
			}
		}
		recvParam = recvParam.Id(typeSpec)
		params = append(params, recvParam)
		funcName = typeName + "_" + funcName
		convertCode := getCodeToConvertInParameter(_type, fast.Name.Name, recvParamName, isPointerRecv, outFile)
		if convertCode != nil {
			blockParams = append( blockParams, convertCode )
		}
	}
	
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
			typeName := typeSpecStr(&field.Type, fast.Name.Name, true)
			if isTypeSpecInDependantList( typeName, dependant_types ) {
				isDependant = true
				if cfg.IgnoreDependants {
					return
				}
			}
			if rune(typeName[0]) == '[' {
				typeName = "*C.GoSlice_"
			} else if(deal_out_string_as_gostring && typeName == "string") {
				typeName = "*C.GoString_"
			} else if isBasicGoType(typeName) {
				typeName = "*" + typeName
			} else if typeName == "map[string]string" {
				typeName = "*C.GoStringMap_"
			}
			paramName := argName("arg"+fmt.Sprintf("%d", fieldIdx))
			params = append(params, jen.Id(paramName).Id(typeName))
			convertCode := getCodeToConvertOutParameter(&field.Type, fast.Name.Name, paramName, false)
			if convertCode != nil {
				output_vars_convert_code = append( output_vars_convert_code, convertCode )
			}
			
		} else {
			lastNameIdx := len(field.Names) - 1
			for nameIdx, ident := range field.Names {
				if nameIdx != lastNameIdx {
					params = append(params, jen.Id(argName(ident.Name)))
				} else {
					typeName := typeSpecStr(&field.Type, fast.Name.Name, false)
					if isTypeSpecInDependantList( typeName, dependant_types ) {
						isDependant = true
						if cfg.IgnoreDependants {
							return
						}
					}
					params = append(params, jen.Id(
						argName(ident.Name)).Id(typeName))
				}
				convertCode := getCodeToConvertInParameter(&field.Type, fast.Name.Name, ident.Name, false, outFile)
				if convertCode != nil {
					blockParams = append( blockParams, convertCode )
				}
			}
		}
	}
	
	cfuncName := "SKY_" + fast.Name.Name + "_" + funcName
	stmt := outFile.Comment("export " + cfuncName)
	stmt = outFile.Func().Id(cfuncName)
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
			if mainPackagePath != "" {
				call_func_code = 
					jen.List(retvars...).Op(":=").Qual(mainPackagePath + packagePath,
						fdecl.Name.Name).Call(callparams...)
			} else {
				call_func_code = 
					jen.List(retvars...).Op(":=").Id(fdecl.Name.Name).Call(callparams...)
			}
		}
	} else {
		if fdecl.Recv != nil {
			call_func_code = jen.Id(fdecl.Recv.List[0].Names[0].Name).Dot(fdecl.Name.Name).Call(callparams...)
		} else {
			if mainPackagePath != "" {
				call_func_code = jen.Qual(mainPackagePath + packagePath,
					fdecl.Name.Name).Call(callparams...)
			} else {
				call_func_code = jen.Id(fdecl.Name.Name).Call(callparams...)
			}
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
	return
}

func isTypeSpecInDependantList(typeSpec string, dependant_list *[]string) bool{
	if dependant_list == nil {
		return false
	}
	//Do not allow extern types in function parameters
	if strings.Index( typeSpec, "C._") >= 0 { 
		return true
	}
	for _, t := range *dependant_list {
		if strings.HasSuffix(typeSpec, "C." + t) {
			return true
		}
	}
	return false
}

/*Returns jen code to convert an input parameter from wrapper to original function*/
func getCodeToConvertInParameter(_typeExpr *ast.Expr, packName string, name string, isPointer bool, outFile *jen.File) jen.Code{
	leftPart := jen.Id(name).Op(":=")
	return getRightCodeToConvertInParameter(leftPart, _typeExpr, packName, name, isPointer, outFile)
}

func getTypeCastCode(arrayPart *jen.Statement, typeExpr *ast.Expr, 
					packName string, name string, outFile *jen.File) jen.Code {
	if identExpr, isIdent := (*typeExpr).(*ast.Ident); isIdent {
		typeName := identExpr.Name
		if isBasicGoType(typeName) {
			return arrayPart.Id(identExpr.Name)
		} else {
			return arrayPart.Id(packName).Dot(typeName)
		}
	} else if selectorExpr, isSelector := (*typeExpr).(*ast.SelectorExpr); isSelector {
		if identExpr, isIdent := (selectorExpr.X).(*ast.Ident); isIdent {
			extern_package, found := findImportPath(identExpr.Name)
			typeName := selectorExpr.Sel.Name
			if found {
				outFile.ImportAlias(extern_package, identExpr.Name)
				return arrayPart.Qual(extern_package, typeName)
			} else {
				return arrayPart.Id(identExpr.Name).Dot(typeName)
			}
		}
	}
	return nil
}

func getRightCodeToConvertInParameter(leftPart *jen.Statement, _typeExpr *ast.Expr, packName string, name string, isPointer bool, outFile *jen.File) jen.Code{
	if arrayExpr, isArray := (*_typeExpr).(*ast.ArrayType); isArray {
		typeExpr := arrayExpr.Elt
		arrayLen := ""
		if arrayExpr.Len != nil {
			if litExpr, isLit := (arrayExpr.Len).(*ast.BasicLit); isLit {
				arrayLen = litExpr.Value
			}
		}
		arrayPart := jen.Op("*").Op("[" + arrayLen + "]")
		arrayTypeCode := getTypeCastCode(arrayPart, &typeExpr, packName, name, outFile)
		if arrayTypeCode != nil {
			if !isPointer {
				leftPart = leftPart.Op("*")
			}
			leftPart = leftPart.Parens( arrayTypeCode )
			var argCode jen.Code
			if !isPointer && arrayExpr.Len == nil {
				argCode = jen.Op("&").Id(argName(name))
			} else {
				argCode = jen.Id(argName(name))
			}
			rightCode := jen.Qual("unsafe", "Pointer").Parens(argCode)
			leftPart = leftPart.Parens(rightCode)
			return leftPart
		}
	} else if starExpr, isPointerParam := (*_typeExpr).(*ast.StarExpr); isPointerParam {
		_type := &starExpr.X
		return getCodeToConvertInParameter(_type, packName, name, true, outFile)
	} else if identExpr, isIdent := (*_typeExpr).(*ast.Ident); isIdent {
		typeName := identExpr.Name
		if isBasicGoType(typeName) {
			return leftPart.Id(argName(name))
		} else if isInplaceConvertType(typeName) {
			if !isPointer {
				leftPart = leftPart.Op("*")
			} 
			return leftPart.Id("inplace"+typeName).Call(jen.Id(argName(name)))
		} else {
			packagePath := ""
			if get_package_path_from_file_name {
				packagePath = getPackagePathFromFileName(cfg.Path)
			}
			if !isPointer {
				leftPart = leftPart.Op("*")
			} 
			return leftPart.Parens(jen.Op("*").Qual(mainPackagePath + packagePath, typeName)).
					Parens( jen.Qual("unsafe", "Pointer").Parens(jen.Id(argName(name))) )
		}
	} else if _, isSelector := (*_typeExpr).(*ast.SelectorExpr); isSelector {
		if !isPointer {
			leftPart = leftPart.Op("*")
		}
		typeCastCode := getTypeCastCode(jen.Op("*"), _typeExpr, packName, name, outFile)
		return leftPart.Parens(typeCastCode).
			Parens( jen.Qual("unsafe", "Pointer").Parens(jen.Id(argName(name))) )
	} else if _, isEllipsis := (*_typeExpr).(*ast.Ellipsis); isEllipsis {
		//TODO: stdevEclipse Implement
		return leftPart.Id(argName(name))
	} else if _, isIntf := (*_typeExpr).(*ast.InterfaceType); isIntf {
		return leftPart.Id("convertToInterface").Call(jen.Id(argName(name)))
	} else if _, isFunc := (*_typeExpr).(*ast.FuncType); isFunc {
		return leftPart.Id("copyToFunc").Call(jen.Id(argName(name)))
	}
	return nil
}

/*Returns jen Code to convert an output parameter from original to wrapper function*/
func getCodeToConvertOutParameter(_typeExpr *ast.Expr, package_name string, name string, isPointer bool) jen.Code{
	
	if _, isArray := (*_typeExpr).(*ast.ArrayType); isArray {
		return jen.Id("copyToGoSlice").Call(jen.Qual("reflect", "ValueOf").Call(jen.Id(argName(name))),
			jen.Id(name))
	} else if starExpr, isPointerRecv := (*_typeExpr).(*ast.StarExpr); isPointerRecv {
		_type := &starExpr.X
		return getCodeToConvertOutParameter(_type, package_name, name, true)
	} else if identExpr, isIdent := (*_typeExpr).(*ast.Ident); isIdent {
		typeName := identExpr.Name
		if deal_out_string_as_gostring && typeName == "string" {
			return jen.Id("copyString").Call(jen.Id(argName(name)), jen.Id(name))
		} else if isBasicGoType(typeName) {
			return jen.Op("*").Id(name).Op("=").Id(argName(name))
		} else if isSkyArrayType(typeName) {
			if isPointer {
				return jen.Id("copyToBuffer").Call(jen.Qual("reflect", "ValueOf").Call(jen.Parens( jen.Op("*").Id(argName(name)) ).Op("[:]")),
							jen.Qual("unsafe", "Pointer").Call(jen.Id(name)),			
							jen.Id("uint").Parens(jen.Id("Sizeof" + typeName)))
			} else {
				return jen.Id("copyToBuffer").Call(jen.Qual("reflect", "ValueOf").Call(jen.Id(argName(name)).Op("[:]")),
						jen.Qual("unsafe", "Pointer").Call(jen.Id(name)), 
						jen.Id("uint").Parens(jen.Id("Sizeof" + typeName)))
			}
		} else {
			if isPointer {
				return jen.Op("*").Id(name).Op("=").Op("*").Parens(jen.Op("*").
					Qual("C", package_name + package_separator + typeName)).
						Parens( jen.Qual("unsafe", "Pointer").Parens(jen.Id(argName(name))) )
			} else {
				return jen.Op("*").Id(name).Op("=").Op("*").Parens(jen.Op("*").
					Qual("C", package_name + package_separator + typeName)).
						Parens( jen.Qual("unsafe", "Pointer").Parens(jen.Op("&").Id(argName(name))) )
			}
		}
	} else if selectorExpr, isSelector := (*_typeExpr).(*ast.SelectorExpr); isSelector {
		identExpr, isIdent := (selectorExpr.X).(*ast.Ident)
		if isIdent {
			typeName := selectorExpr.Sel.Name
			if isPointer {
				return jen.Op("*").Id(name).Op("=").Op("*").Parens(jen.Op("*").
					Qual("C", identExpr.Name + package_separator + typeName)).
						Parens( jen.Qual("unsafe", "Pointer").Parens(jen.Id(argName(name))) )
			} else {
				return jen.Op("*").Id(name).Op("=").Op("*").Parens(jen.Op("*").
					Qual("C", identExpr.Name + package_separator + typeName)).
						Parens( jen.Qual("unsafe", "Pointer").Parens(jen.Op("&").Id(argName(name))) )
			}
		}
	} else if mapExpr, isMap := (*_typeExpr).(*ast.MapType); isMap {
		identKeyExpr, isKeyIdent := (mapExpr.Key).(*ast.Ident)
		identValueExpr, isValueIdent := (mapExpr.Value).(*ast.Ident)
		if isKeyIdent && isValueIdent {
			if identKeyExpr.Name == "string" && identValueExpr.Name == "string" {
				return jen.Id("copyToStringMap").Call(jen.Id(argName(name)), jen.Id(name))
			}
		}
	}
	return nil
}

/* Returns the corresponding C type for a GO type*/
func goTypeToCType(goType string) (string, bool) {
	if val, ok := typesMap[goType]; ok {
		return val, true
	} else {
		return goType, false
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

func getInplaceConvertTypePackage(typeName string) string {
	if val, ok := inplaceConvertTypesPackages[typeName]; ok {
		return val
	} else {
		return "cipher"
	}
}


/* Process a type expression. Returns the code in C for the type and ok if successfull */
func processTypeExpression(fast *ast.File, type_expr ast.Expr, 
							package_name string, name string, 
							defined_types *[]string, 
							forwards_declarations *[]string, depth int,
							dependant_types *[]string) (string, bool, bool) {
	c_code := ""
	result := false
	dependant := false
	if typeStruct, isTypeStruct := (type_expr).(*ast.StructType); isTypeStruct {
		c_code += "struct{\n"
		error := false
		for _, field := range typeStruct.Fields.List{
			var names []string
			for _, fieldName := range field.Names{
				names = append( names, fieldName.Name )
			}
			if len(names) == 0 {
				names = append( names, "_unnamed")
			}
			for _, fieldName := range names{
				for i := 0; i < depth * 4; i++{
					c_code += " "
				}
				type_code, result, isFieldDependant := processTypeExpression(fast, field.Type, package_name, fieldName, 
						defined_types, forwards_declarations, depth + 1, dependant_types)
				if result {
					if isFieldDependant {
						dependant = true
					}
					c_code += type_code
				} else {
					error = true
				}
				c_code += ";\n"
			}
			
			
		}
		for i := 0; i < (depth - 1) * 4; i++{
			c_code += " "
		}
		c_code += "} ";
		typeName := name
		if depth == 1 {
			typeName = package_name + package_separator + typeName
		}
		c_code += typeName
		if dependant && depth == 1 {
			addDependant( dependant_types, typeName )
		}
		result = !error
	}else if arrayExpr, isArray := (type_expr).(*ast.ArrayType); isArray {
		var arrayCode string
		var arrayElCode string
		result = false
		new_name := name
		if depth == 1 {
			new_name = package_name + package_separator + name
		}
		if arrayExpr.Len == nil {
			arrayCode = new_name
			arrayElCode = "GoSlice_ "
			result = true
		} else if litExpr, isLit := (arrayExpr.Len).(*ast.BasicLit); isLit {
			arrayElCode, result, dependant = processTypeExpression(fast, arrayExpr.Elt, package_name, "", 
					defined_types, forwards_declarations, depth + 1, dependant_types)
			if result {
				arrayCode = new_name+"[" + litExpr.Value + "]"
			}
		}
		if result {
			if dependant && depth == 1 {
				addDependant( dependant_types, new_name )
			}
			c_code += arrayElCode + " " + arrayCode
		}
	}else if _, isFunc := (type_expr).(*ast.FuncType); isFunc {
		new_name := name
		if depth == 1 {
			new_name = package_name + package_separator + name
		}
		c_code += "Handle " + new_name
		result = true
	}else if _, isIntf := (type_expr).(*ast.InterfaceType); isIntf {
		new_name := name
		if depth == 1 {
			new_name = package_name + package_separator + name
		}
		c_code += "GoInterface_ " + new_name
		result = true
	}else if _, isChan := (type_expr).(*ast.ChanType); isChan {
		new_name := name
		if depth == 1 {
			new_name = package_name + package_separator + name
		}
		c_code += "GoChan_ " + new_name
		result = true
	}else if _, isMap := (type_expr).(*ast.MapType); isMap {
		new_name := name
		if depth == 1 {
			new_name = package_name + package_separator + name
		}
		c_code += "GoMap_ " + new_name
		result = true
	}else if starExpr, isStart := (type_expr).(*ast.StarExpr); isStart {
		targetTypeExpr := starExpr.X
		type_code, ok, isFieldDependant := processTypeExpression(fast, targetTypeExpr, package_name, "", 
			defined_types, forwards_declarations, depth + 1, dependant_types)
		if ok {
			if isFieldDependant {
				dependant = true
			}
			c_code += type_code
			new_name := name
			if depth == 1 {
				new_name = package_name + package_separator + name
			}
			c_code += "* "  + new_name
			if dependant  && depth == 1{
				addDependant( dependant_types, new_name)
			}
			result = true
		}
	}else if identExpr, isIdent := (type_expr).(*ast.Ident); isIdent {
		type_code, isBasic := goTypeToCType(identExpr.Name)
		if !isBasic {
			pack_prefix := ""
			addDependency := false
			if package_name != fast.Name.Name && !isSkycoinName(package_name) {
				pack_prefix = "_"
				if cfg.DependOnlyExternal {
					if isExternalName(package_name) {
						addDependency = true
					}
				} else {
					addDependency = true
				}
			}
			type_code = pack_prefix + package_name + package_separator + type_code
			if addDependency {
				addDependant(dependant_types, type_code)
				dependant = true
			}
		}
		c_code = type_code
		c_code += " "
		new_name := name
		if depth == 1 {
			new_name = package_name + package_separator + name
		}
		c_code += new_name
		if !dependant {
			if isDependantType(dependant_types, type_code) {
				dependant = true
			}
		}
		type_found := false
		for _, defined_type := range *defined_types{
			if defined_type == type_code{
				type_found = true
			}
		}
		if dependant && depth == 1 {
			addDependant( dependant_types, new_name )
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
	} else if selectorExpr, isSelector := (type_expr).(*ast.SelectorExpr); isSelector {
		extern_package := package_name
		identExpr, isIdent := (selectorExpr.X).(*ast.Ident)
		if isIdent {
			extern_package = identExpr.Name
		}
		new_name := name
		if depth == 1 {
			new_name = package_name + package_separator + name
		}
		type_code, ok, isFieldDependant := processTypeExpression(fast, selectorExpr.Sel, extern_package, new_name, 
			defined_types, forwards_declarations, depth + 1, dependant_types)
		if isFieldDependant {
			dependant = true
		}
		if dependant && depth == 1 {
			addDependant( dependant_types, new_name )
		}
		if ok {
			c_code = type_code
			result = true
		}
	}
	return c_code, result, dependant
}

func isDependantType(dependant_types *[]string, typeName string) bool {
	for _, t := range *dependant_types {
		if t == typeName {
			return true
		}
	}
	return false
}

func addDependant(dependant_types *[]string, typeName string){
	for _, t := range *dependant_types {
		if t == typeName {
			return
		}
	}
	*dependant_types = append( *dependant_types, typeName )
} 

/* Process a type definition in GO and returns the c code for the definition */
func processTypeDef(fast *ast.File, tdecl *ast.GenDecl, 
					defined_types *[]string, forwards_declarations *[]string,
					dependant_types *[]string) (string, bool, bool) {
	result_code := ""
	result := true
	isDependant := false
	for _, s := range tdecl.Specs{
		if typeSpec, isTypeSpec := (s).(*ast.TypeSpec); isTypeSpec {
			applog("Processing %v \n", typeSpec.Name.Name)
			type_c_code, ok, isDependantExpr := processTypeExpression(fast, typeSpec.Type, 
				fast.Name.Name, typeSpec.Name.Name, defined_types, forwards_declarations, 1,
				dependant_types)
			if ok {
				if isDependantExpr {
					isDependant = true
				}
				result_code += "typedef "
				result_code += type_c_code
				result_code += ";\n"
				*defined_types = append( *defined_types, fast.Name.Name + package_separator + typeSpec.Name.Name )
			} else {
				result = false
			}
		}
	}
	return result_code, result, isDependant
}

/* Process all type definitions. Returns c code for all the defintions */
func processTypeDefs(fast *ast.File, typeDecls []*ast.GenDecl, dependant_types *[]string) string {
	result_code := ""
	var defined_types []string
	for key, _ := range typesMap {
		ctype, ok := goTypeToCType(key)
		if ok {
			defined_types = append( defined_types, ctype )
		}
	}
	
	unprocessed := len( typeDecls )
	went_blank := false
	for unprocessed > 0 && !went_blank {
		went_blank = true
		for index, typeDecl := range typeDecls{
			if typeDecl != nil {
				typeCode, ok, isDependant := processTypeDef(fast, typeDecl, &defined_types, nil, dependant_types)
				if ok {
					went_blank = false
					typeDecls[index] = nil
					if !(cfg.IgnoreDependants && isDependant) {
						result_code += typeCode
					}
					unprocessed -= 1
				}
			}
		}
	}
	
	//TODO: if unprocessed > 0 then there are cyclic type references. Use forward declarations.
	var forwards_declarations []string
	if unprocessed > 0 {
		for _, typeDecl := range typeDecls{
			if typeDecl != nil {
				typeCode, ok, isDependant := processTypeDef(fast, typeDecl, &defined_types, &forwards_declarations, dependant_types)
				if ok {
					if !(cfg.IgnoreDependants && isDependant) {
						result_code += typeCode
					}
				}
			}
		}
	}
	return result_code
}

func fixExportComment(filePath string){
	f, err := os.Open(filePath)
	check(err)
	buf := new(bytes.Buffer)
	buf.ReadFrom(f)
	contents := buf.String()
	f.Close()
	
	contents = strings.Replace( contents, "// export SKY_", "//export SKY_", -1)
	f, err = os.Create(filePath)
	check(err)
	f.WriteString( contents )
	f.Sync()
	f.Close()
}

