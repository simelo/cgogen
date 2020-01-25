package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"strings"

	"github.com/dave/jennifer/jen"
)

type Config struct {
	Path                string
	Verbose             bool
	ProcessFunctions    bool
	ProcessTypes        bool
	OutputFileGO        string
	OutputFileC         string
	OutputFileCH        string
	ProcessDependencies bool
	DependOnlyExternal  bool
	TypeDependencyFile  string
	FuncDependencyFile  string
	TypeConversionFile  string
	IgnoreDependants    bool
	FullTranspile       bool //Full conversion to c code
	FullTranspileDir    string
	FullTranspileOut    string
}

func (c *Config) register() {
	flag.StringVar(&c.Path, "i", "", "PATH to source file")
	flag.StringVar(&c.OutputFileGO, "g", "", "PATH to destination file for go code")
	flag.StringVar(&c.OutputFileC, "c", "", "PATH to destination file for C code")
	flag.StringVar(&c.OutputFileCH, "h", "", "PATH to destination file for header C code")
	flag.BoolVar(&c.Verbose, "v", true, "Print debug message to stdout")
	flag.BoolVar(&c.FullTranspile, "full", false, "Full conversion to C code")
	flag.BoolVar(&c.ProcessFunctions, "f", false, "Process functions")
	flag.BoolVar(&c.ProcessTypes, "t", false, "Process Types")
	flag.BoolVar(&c.ProcessDependencies, "d", false, "Analyze dependencies")
	flag.BoolVar(&c.DependOnlyExternal, "n", false, "Analyze only dependencies on external libraries")
	flag.StringVar(&c.TypeDependencyFile, "td", "", "PATH to destination file where dependant types will be stored")
	flag.StringVar(&c.FuncDependencyFile, "fd", "", "PATH to destination file where dependant functions will be stored")
	flag.StringVar(&c.TypeConversionFile, "tc", "", "PATH to file where types conversion settings will be stored")
	flag.BoolVar(&c.IgnoreDependants, "id", false, "Ignore dependants")
	flag.StringVar(&c.FullTranspileDir, "transdir", "", "Directory to get source code for full transpile")
	flag.StringVar(&c.FullTranspileOut, "transout", "", "Directory to put c files of full transpile")
}

var (
	cfg    Config
	applog = func(format string, v ...interface{}) {
		// Logging disabled
	}
)

//Map of types that will replaced by custom types
var customTypesMap = make(map[string]string)

//Types that will use functions of type inplace to convert
var inplaceConvertTypesPackages = map[string]string{
	"PubKeySlice":   "cipher",
	"Address":       "cipher",
	"BalanceResult": "cli",
}

var mainPackagePath = string("github.com/SkycoinProject/skycoin/src/")

var arrayTypes = map[string]string{
	"PubKey":    "cipher",
	"SHA256":    "cipher",
	"Sig":       "cipher",
	"SecKey":    "cipher",
	"Ripemd160": "cipher",
	"UxArray":   "coin",
}

//Imports used in this code file
var importDefs []*ast.GenDecl

//types that will be replaced by handles
var handleTypes map[string]string
var returnVarName = "____error_code"
var returnErrName = "____return_err"

func main() {
	handleTypes = make(map[string]string)

	cfg.register()
	flag.Parse()

	if cfg.Verbose {
		applog = log.Printf
	}

	if cfg.FullTranspile {
		doFullTranspile()
	} else {
		doGoFile()
	}
}

func doGoFile() {
	var dependantFunctions []string
	var dependantTypes []string
	var typeConversions []string
	if cfg.ProcessDependencies {
		if cfg.TypeDependencyFile != "" {
			dependantTypes = loadDependencyFile(cfg.TypeDependencyFile, "|")
		}
		if cfg.FuncDependencyFile != "" {
			dependantFunctions = loadDependencyFile(cfg.FuncDependencyFile, "\n")
		}
	}
	if cfg.TypeConversionFile != "" {
		typeConversions = loadDependencyFile(cfg.TypeConversionFile, "\n")
		for _, str := range typeConversions {
			processTypeSetting(str)
		}
	}
	applog("Opening %v \n", cfg.Path)
	fo, err := os.Open(cfg.Path)
	check(err)

	defer fo.Close()

	fset := token.NewFileSet()
	fast, err := parser.ParseFile(fset, "", fo, parser.AllErrors|parser.ParseComments)
	check(err)

	packagePath := getPackagePathFromFileName(cfg.Path) + "/" + fast.Name.Name
	applog("Package Path: %s ", packagePath)
	if packagePath == "" {
		packagePath = fast.Name.Name
	}

	var outFile *jen.File

	if cfg.ProcessFunctions {
		outFile = jen.NewFile("main")

		outFile.CgoPreamble(`
	  #include <string.h>
	  #include <stdlib.h>
	  
	  #include "skytypes.h"
	  #include "skyfee.h"`)
	}

	typeDefs := make([]*ast.GenDecl, 0)

	for _, _decl := range fast.Decls {

		if cfg.ProcessFunctions {
			if decl, ok := (_decl).(*ast.FuncDecl); ok {

				var plist *[]string
				if cfg.ProcessDependencies {
					plist = &dependantTypes
				}
				if isDependant := processFunc(fast, decl, outFile, plist); isDependant {
					addDependant(&dependantFunctions, packagePath+" "+decl.Name.Name)
				}
			}
		}
		if cfg.ProcessTypes {
			if decl, ok := (_decl).(*ast.GenDecl); ok {
				if decl.Tok == token.TYPE {
					typeDefs = append(typeDefs, decl)
				} else if decl.Tok == token.IMPORT {
					importDefs = append(importDefs, decl)
				}
			}
		}
	}
	if cfg.ProcessTypes {
		typeDefsCode := processTypeDefs(fast, typeDefs, &dependantTypes)
		if cfg.OutputFileCH != "" {
			saveTextToFile(cfg.OutputFileCH, typeDefsCode)
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
			saveDependencyFile(cfg.TypeDependencyFile, dependantTypes, "|")

		} else {
			fmt.Println("Dependant Types: ", dependantTypes)
		}
		if cfg.FuncDependencyFile != "" {
			saveDependencyFile(cfg.FuncDependencyFile, dependantFunctions, "\r\n")
		} else {
			fmt.Println("Dependant Functions: ", dependantFunctions)
		}
	}
	applog("Finished %v", cfg.Path)
	if cfg.OutputFileGO != "" {
		fixExportComment(cfg.OutputFileGO)
	}
}

func doFullTranspile() {
	if cfg.FullTranspileDir == "" {
		fmt.Println("Must specify full transpile source directory")
		return
	}
	if cfg.FullTranspileOut == "" {
		fmt.Println("Must specify full transpile destination directory")
		return
	}
	Full_Transpile(cfg.FullTranspileDir, cfg.FullTranspileOut)
}

func saveTextToFile(fileName string, text string) {
	f, err := os.Create(fileName)
	check(err)
	defer f.Close()
	_, err = f.WriteString(text)
	check(err)
	err = f.Sync()
	check(err)
}

func saveDependencyFile(path string, list []string, separator string) {
	f, err := os.Create(path)
	check(err)
	defer f.Close()
	_, err = f.WriteString(strings.Join(list, separator))
	check(err)
	err = f.Sync()
	check(err)
}

func loadDependencyFile(path string, separator string) (list []string) {
	f, err := os.Open(path)
	if err == nil {
		defer f.Close()
		buf := new(bytes.Buffer)
		_, err = buf.ReadFrom(f)
		check(err)
		contents := buf.String()
		tlist := strings.Split(contents, separator)
		for _, str := range tlist {
			nstr := strings.Replace(str, "\r", "", -1)
			nstr = strings.Replace(nstr, "\n", "", -1)
			if nstr != "" {
				list = append(list, nstr)
			}
		}
	}
	return
}

func check(err error) {
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func isAsciiUpper(c rune) bool {
	return c >= 'A' && c <= 'Z'
}

//Returns the path of the package imported
func findImportPath(importName string) (string, bool) {
	for _, importDef := range importDefs {
		for _, s := range importDef.Specs {
			if importSpec, isImportSpec := (s).(*ast.ImportSpec); isImportSpec {
				name := ""
				path := importSpec.Path.Value

				path = strings.TrimPrefix(path, "\"")

				path = strings.TrimSuffix(path, "\"")

				if importSpec.Name != nil {
					name = importSpec.Name.Name
				} else {
					pathParts := strings.Split(path, "/")
					if len(pathParts) > 0 {
						name = pathParts[len(pathParts)-1]
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
		return strings.HasPrefix(path, "github.com/SkycoinProject")
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

func typeSpecStr(_typeExpr *ast.Expr, packageName string, isOutput bool) (string, bool) {
	addPointer := false
	spec := ""
	for _typeExpr != nil {
		if arrayExpr, isArray := (*_typeExpr).(*ast.ArrayType); isArray {
			if arrayExpr.Len != nil || isOutput {
				return "*C.GoSlice_", true
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
			tspec, ok := typeSpecStr(&ellipsisExpr.Elt, packageName, isOutput)
			if ok {
				spec += "..." + tspec
				_typeExpr = nil
				continue
			} else {
				return "", false
			}
		}
		if _, isFunc := (*_typeExpr).(*ast.FuncType); isFunc {
			return "", false
		}
		if _, isStruct := (*_typeExpr).(*ast.StructType); isStruct {
			spec += "struct{}"
			_typeExpr = nil
			continue
		}
		if _, isIntf := (*_typeExpr).(*ast.InterfaceType); isIntf {
			return "", false
		}
		if _, isChan := (*_typeExpr).(*ast.ChanType); isChan {
			// TODO: Improve func type translation
			spec += "C.GoChan_"
			_typeExpr = nil
			continue
		}
		if mapExpr, isMap := (*_typeExpr).(*ast.MapType); isMap {
			tspeckey, okkey := typeSpecStr(&mapExpr.Key, packageName, false)
			tspecvalue, okvalue := typeSpecStr(&mapExpr.Key, packageName, false)
			if okkey && okvalue {
				return spec + "map[" + tspeckey + "]" + tspecvalue, true
			} else {
				return "", false
			}
		}
		identExpr, isIdent := (*_typeExpr).(*ast.Ident)
		selExpr, isSelector := (*_typeExpr).(*ast.SelectorExpr)
		if isIdent || isSelector {
			isDealt := false
			externPackage := packageName
			typeName := ""
			if isIdent {
				typeName = identExpr.Name
				isDealt = isInHandleTypesList(typeName)
				if isDealt {
					spec = getHandleName(typeName)
				} else if isInCustomTypesList(typeName) {
					spec = getCustomTypeName(typeName)
					isDealt = true
				}
			} else {
				typeName = selExpr.Sel.Name
				identSelExpr, isSelIdent := (selExpr.X).(*ast.Ident)
				if isSelIdent {
					externPackage = identSelExpr.Name
					isDealt = isInHandleTypesList(externPackage + "." + typeName)
					if isDealt {
						spec = getHandleName(externPackage + "." + typeName)
					} else if isInCustomTypesList(externPackage + "." + typeName) {
						spec = getCustomTypeName(externPackage + "." + typeName)
						isDealt = true
					} else if !isSkycoinName(externPackage) {
						return externPackage, false
					}
				}
			}
			if !isDealt {
				if isInHandleTypesList(externPackage + packageSeparator + typeName) {
					spec = getHandleName(externPackage + packageSeparator + typeName)
				} else {
					isExported := isAsciiUpper(rune(typeName[0]))
					if spec == "" && !addPointer && isExported {
						addPointer = true
					}
					if isExported {
						spec += "C." + externPackage + packageSeparator
					} else {
						if !IsBasicGoType(typeName) {
							return "", false //Don't deal with unexported types
						}
					}
					spec += typeName
				}
			}
			_typeExpr = nil
		} else {
			applog("No rules to follow with %s", (*_typeExpr).(*ast.Ident))
			_typeExpr = nil
		}
	}
	if addPointer {
		return "*" + spec, true
	}
	return spec, true
}

func argName(name string) string {
	return "_" + name
}

func resultName(name string) string {
	return "__" + name
}

func isInHandleTypesList(typeName string) bool {
	_, ok := handleTypes[typeName]
	return ok
}

func isInCustomTypesList(typeName string) bool {
	_, ok := customTypesMap[typeName]
	return ok
}

func getHandleName(typeName string) string {
	return "*C." + handleTypes[typeName] + packageSeparator + "Handle"
}

func getCustomTypeName(typeName string) string {
	return "*C." + customTypesMap[typeName]
}

/*
Get the package path from file name. Assumes that file is formed by joining path folders with dot
Example: pack.folder1.folder2  ==>  pack/folder1/folder2
*/
func getPackagePathFromFileName(filePath string) string {
	packagePath := ""
	folders := strings.Split(filePath, "/")
	if len(folders) > 0 {
		fileName := folders[len(folders)-1]
		packageFolders := strings.Split(fileName, ".")
		if len(packageFolders) > 2 {
			packageFolders = packageFolders[:len(packageFolders)-2]
			var result []string
			for _, s := range packageFolders {
				if s == "internal" || s == "example" {
					break
				} else {
					result = append(result, s)
				}
			}
			packagePath = strings.Join(result, "/")
		}
	}
	return packagePath
}

//Create code for wrapper function
func processFunc(fast *ast.File, fdecl *ast.FuncDecl, outFile *jen.File, dependantTypes *[]string) (isDependant bool) {
	isDependant = false
	packagePath := getPackagePathFromFileName(cfg.Path)
	if packagePath == "" {
		packagePath = fast.Name.Name
	}

	funcName := fdecl.Name.Name

	if !fdecl.Name.IsExported() || strings.HasPrefix(funcName, "Must") {
		applog("Skipping %v \n", funcName)
		return
	}

	applog("Processing %v \n", funcName)
	var blockParams []jen.Code

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
		typeSpec, ok := typeSpecStr(_type, fast.Name.Name, false)
		if !ok || isTypeSpecInDependantList(typeSpec, dependantTypes) {
			isDependant = true
			if cfg.IgnoreDependants {
				//TODO: stdevEclipse Check if type can be replaced by another type or handle
				return
			}
		}
		recvParam = recvParam.Id(typeSpec)
		params = append(params, recvParam)
		funcName = typeName + "_" + funcName
		convertCodes := getCodeToConvertInParameter(_type, fast.Name.Name, recvParamName, isPointerRecv, outFile)
		if convertCodes != nil {
			blockParams = append(blockParams, convertCodes...)
		}
	}

	allparams := fdecl.Type.Params.List[:]
	returnFieldsIndex := len(allparams)
	var retField *ast.Field = nil

	if fdecl.Type.Results != nil && fdecl.Type.Results.List != nil {
		//Find the return argument of type error.
		//It should always be the last argument but search just in case
		errorIndex := -1
		for index, field := range fdecl.Type.Results.List {
			identExpr, isIdent := (field.Type).(*ast.Ident)
			if isIdent && identExpr.Name == "error" {
				errorIndex = index
				break
			}
		}
		if errorIndex >= 0 {
			retField = fdecl.Type.Results.List[errorIndex]
			returnParams := append(fdecl.Type.Results.List[0:errorIndex], fdecl.Type.Results.List[errorIndex+1:]...)
			allparams = append(allparams, returnParams...)
		} else {
			allparams = append(allparams, fdecl.Type.Results.List[:]...)
		}
	}

	var outputVarsConvertCode []jen.Code

	for fieldIdx, field := range allparams {
		if fieldIdx >= returnFieldsIndex {
			// Field in return types list
			typeName, ok := typeSpecStr(&field.Type, fast.Name.Name, true)
			if !ok || isTypeSpecInDependantList(typeName, dependantTypes) {
				isDependant = true
				if cfg.IgnoreDependants {
					//TODO: stdevEclipse Check if type can be replaced by another type or handle
					return
				}
			}
			if len(typeName) > 0 && rune(typeName[0]) == '[' {
				typeName = "*C.GoSlice_"
			} else if typeName == "string" {
				typeName = "*C.GoString_"
			} else if IsBasicGoType(typeName) {
				typeName = "*" + typeName
			} else if typeName == "map[string]string" {
				typeName = "*C.GoStringMap_"
			}
			paramName := argName("arg" + fmt.Sprintf("%d", fieldIdx))
			params = append(params, jen.Id(paramName).Id(typeName))
			convertCode := getCodeToConvertOutParameter(&field.Type, fast.Name.Name, paramName, false)
			if convertCode != nil {
				outputVarsConvertCode = append(outputVarsConvertCode, convertCode)
			}

		} else {
			lastNameIdx := len(field.Names) - 1
			for nameIdx, ident := range field.Names {
				if nameIdx != lastNameIdx {
					params = append(params, jen.Id(argName(ident.Name)))
				} else {
					typeName, ok := typeSpecStr(&field.Type, fast.Name.Name, false)
					if !ok || isTypeSpecInDependantList(typeName, dependantTypes) {
						isDependant = true
						if cfg.IgnoreDependants {
							//TODO: stdevEclipse Check if type can be replaced by another type or handle
							return
						}
					}
					params = append(params, jen.Id(
						argName(ident.Name)).Id(typeName))
				}
				convertCodes := getCodeToConvertInParameter(&field.Type, fast.Name.Name, ident.Name, false, outFile)
				if convertCodes != nil {
					blockParams = append(blockParams, convertCodes...)
				}
			}
		}
	}

	cfuncName := "SKY_" + fast.Name.Name + "_" + funcName
	stmt := outFile.Comment("export " + cfuncName) //nolint staticcheck
	stmt = outFile.Func().Id(cfuncName)
	stmt = stmt.Params(params...)

	var callparams []jen.Code
	for _, field := range fdecl.Type.Params.List {
		for _, name := range field.Names {
			callparams = append(callparams, *jen.Id(name.Name)...)
		}
	}
	var retvars []jen.Code
	if returnFieldsIndex < len(allparams) {
		for i := returnFieldsIndex; i < len(allparams); i++ {
			retvars = append(retvars, jen.Id(resultName("arg"+fmt.Sprintf("%d", i))))
		}
	}
	if retField != nil {
		retvars = append(retvars, jen.Id(returnErrName))
	}
	var callFuncCode jen.Code
	if len(retvars) > 0 {
		if fdecl.Recv != nil {
			callFuncCode =
				jen.List(retvars...).Op(":=").Id(fdecl.Recv.List[0].Names[0].Name).Dot(fdecl.Name.Name).Call(callparams...)
		} else {
			callFuncCode =
				jen.List(retvars...).Op(":=").Qual(mainPackagePath+packagePath,
					fdecl.Name.Name).Call(callparams...)
		}
	} else {
		if fdecl.Recv != nil {
			callFuncCode = jen.Id(fdecl.Recv.List[0].Names[0].Name).Dot(fdecl.Name.Name).Call(callparams...)
		} else {
			callFuncCode = jen.Qual(mainPackagePath+packagePath,
				fdecl.Name.Name).Call(callparams...)
		}
	}
	blockParams = append(blockParams, callFuncCode)

	stmt = stmt.Parens(jen.Id(returnVarName).Id("uint32"))
	if retField != nil {
		blockParams = append(blockParams, jen.Id(returnVarName).Op("=").Id("libErrorCode").Call(jen.Id(returnErrName)))
		convertOutputCode := jen.If(jen.Id(returnErrName).Op("==").Nil()).Block(outputVarsConvertCode...)
		blockParams = append(blockParams, convertOutputCode)
	} else {
		blockParams = append(blockParams, outputVarsConvertCode...)
	}

	blockParams = append(blockParams, jen.Return())

	stmt.Block(blockParams...)
	return
}

//Check if type is in dependant list
func isTypeSpecInDependantList(typeSpec string, dependantList *[]string) bool {
	if dependantList == nil {
		return false
	}
	//Do not allow extern types in function parameters
	if strings.Contains(typeSpec, "C._") {
		return true
	}
	for _, t := range *dependantList {
		if strings.HasSuffix(typeSpec, "C."+t) {
			return true
		}
	}
	return false
}

//Creates code to make a typecast
//nolint unparam
func getTypeCastCode(leftPart *jen.Statement, typeExpr *ast.Expr,
	packName string, name string, outFile *jen.File) jen.Code {
	if identExpr, isIdent := (*typeExpr).(*ast.Ident); isIdent {
		typeName := identExpr.Name
		if IsBasicGoType(typeName) {
			return leftPart.Id(identExpr.Name)
		} else {
			return leftPart.Id(packName).Dot(typeName)
		}
	} else if selectorExpr, isSelector := (*typeExpr).(*ast.SelectorExpr); isSelector {
		if identExpr, isIdent := (selectorExpr.X).(*ast.Ident); isIdent {
			externPackage, found := findImportPath(identExpr.Name)
			typeName := selectorExpr.Sel.Name
			if found {
				outFile.ImportAlias(externPackage, identExpr.Name)
				return leftPart.Qual(externPackage, typeName)
			} else {
				return leftPart.Id(identExpr.Name).Dot(typeName)
			}
		}
	}
	return nil
}

func getLookupHandleCode(name string, typeName string, isPointer bool) []jen.Code {
	varname := name
	if !isPointer {
		varname = "__" + name
	}
	listVar := jen.List(jen.Id(varname), jen.Id("ok"+name)).Op(":=")
	lookUpName := "lookup" + handleTypes[typeName] + "Handle"
	listVar = listVar.Id(lookUpName).Call(jen.Op("*").Id(argName(name)))
	checkError := jen.If(jen.Op("!").Id("ok"+name)).
		Block(jen.Id(returnVarName).Op("=").Id("SKY_BAD_HANDLE"), jen.Return())
	if !isPointer {
		assign := jen.Id(name).Op(":=").Op("*").Id(varname)
		return jenCodeToArray(listVar, checkError, assign)
	} else {
		return jenCodeToArray(listVar, checkError)
	}
}

func jenCodeToArray(statements ...jen.Code) []jen.Code {
	var codeArray []jen.Code
	codeArray = append(codeArray, statements...)
	return codeArray
}

/*Returns jen code to convert an input parameter from wrapper to original function*/
func getCodeToConvertInParameter(_typeExpr *ast.Expr, packName string, name string, isPointer bool, outFile *jen.File) []jen.Code {
	leftPart := jen.Id(name).Op(":=")
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
			leftPart = leftPart.Parens(arrayTypeCode)
			var argCode jen.Code
			if !isPointer && arrayExpr.Len == nil {
				argCode = jen.Op("&").Id(argName(name))
			} else {
				argCode = jen.Id(argName(name))
			}
			rightCode := jen.Qual("unsafe", "Pointer").Parens(argCode)
			leftPart = leftPart.Parens(rightCode)
			return jenCodeToArray(leftPart)
		}
	} else if starExpr, isPointerParam := (*_typeExpr).(*ast.StarExpr); isPointerParam {
		_type := &starExpr.X
		return getCodeToConvertInParameter(_type, packName, name, true, outFile)
	} else if identExpr, isIdent := (*_typeExpr).(*ast.Ident); isIdent {
		typeName := identExpr.Name
		if IsBasicGoType(typeName) {
			return jenCodeToArray(leftPart.Id(argName(name)))
		} else if isInHandleTypesList(typeName) {
			return getLookupHandleCode(name, typeName, isPointer)
		} else if isInplaceConvertType(typeName) {
			if !isPointer {
				leftPart = leftPart.Op("*")
			}
			return jenCodeToArray(leftPart.Id("inplace" + typeName).Call(jen.Id(argName(name))))
		} else {
			if isInHandleTypesList(packName + packageSeparator + typeName) {
				return getLookupHandleCode(name, packName+packageSeparator+typeName, isPointer)
			} else {
				if !isPointer {
					leftPart = leftPart.Op("*")
				}
				if typeName == "FeeCalculator" {
					leftPart = jen.Id(name).Op(":=").Add(getCallbackCode(name))
				} else {
					leftPart = leftPart.Parens(jen.Op("*").Id(packName).Id(".").Id(typeName)).
						Parens(jen.Qual("unsafe", "Pointer").Parens(jen.Id(argName(name))))
				}
				return jenCodeToArray(leftPart)
			}
		}
	} else if selectorExpr, isSelector := (*_typeExpr).(*ast.SelectorExpr); isSelector {
		if identExpr, isIdent := (selectorExpr.X).(*ast.Ident); isIdent {
			packName = identExpr.Name
			typeName := selectorExpr.Sel.Name
			if isInHandleTypesList(packName + packageSeparator + typeName) {
				return getLookupHandleCode(name, packName+packageSeparator+typeName, isPointer)
			}
		}
		if !isPointer {
			leftPart = leftPart.Op("*")
		}
		typeCastCode := getTypeCastCode(jen.Op("*"), _typeExpr, packName, name, outFile)
		return jenCodeToArray(leftPart.Parens(typeCastCode).
			Parens(jen.Qual("unsafe", "Pointer").Parens(jen.Id(argName(name)))))
	} else if _, isEllipsis := (*_typeExpr).(*ast.Ellipsis); isEllipsis {
		//TODO: stdevEclipse Implement
		return jenCodeToArray(leftPart.Id(argName(name)))
	} else if _, isIntf := (*_typeExpr).(*ast.InterfaceType); isIntf {
		return jenCodeToArray(leftPart.Id("convertToInterface").Call(jen.Id(argName(name))))
	} else if _, isFunc := (*_typeExpr).(*ast.FuncType); isFunc {
		return jenCodeToArray(leftPart.Id("copyToFunc").Call(jen.Id(argName(name))))
	}
	return nil
}

/*Returns jen Code to convert an output parameter from original to wrapper function*/
func getCodeToConvertOutParameter(_typeExpr *ast.Expr, package_name string, name string, isPointer bool) jen.Code {

	if _, isArray := (*_typeExpr).(*ast.ArrayType); isArray {
		return jen.Id("copyToGoSlice").Call(jen.Qual("reflect", "ValueOf").Call(jen.Id(argName(name))),
			jen.Id(name))
	} else if starExpr, isPointerRecv := (*_typeExpr).(*ast.StarExpr); isPointerRecv {
		_type := &starExpr.X
		return getCodeToConvertOutParameter(_type, package_name, name, true)
	} else if identExpr, isIdent := (*_typeExpr).(*ast.Ident); isIdent {
		typeName := identExpr.Name
		if typeName == "string" {
			return jen.Id("copyString").Call(jen.Id(argName(name)), jen.Id(name))
		} else if IsBasicGoType(typeName) {
			return jen.Op("*").Id(name).Op("=").Id(argName(name))
		} else if isInHandleTypesList(package_name + packageSeparator + typeName) {
			var argCode jen.Code
			if isPointer {
				argCode = jen.Id(argName(name))
			} else {
				argCode = jen.Op("&").Id(argName(name))
			}
			return jen.Op("*").Id(name).Op("=").Id("register" + handleTypes[package_name+packageSeparator+typeName] + "Handle").Call(argCode)
		} else if isSkyArrayType(typeName) {
			var argCode jen.Code
			if isPointer {
				argCode = jen.Parens(jen.Op("*").Id(argName(name))).Op("[:]")
			} else {
				argCode = jen.Id(argName(name)).Op("[:]")
			}

			return jen.Id("copyToBuffer").Call(jen.Qual("reflect", "ValueOf").Call(argCode),
				jen.Qual("unsafe", "Pointer").Call(jen.Id(name)),
				jen.Id("uint").Parens(jen.Id("Sizeof"+typeName)))

		} else {
			var argCode jen.Code
			if isPointer {
				argCode = jen.Id(argName(name))
			} else {
				argCode = jen.Op("&").Id(argName(name))
			}
			return jen.Op("*").Id(name).Op("=").Op("*").Parens(jen.Op("*").
				Qual("C", package_name+packageSeparator+typeName)).
				Parens(jen.Qual("unsafe", "Pointer").Parens(argCode))
		}
	} else if selectorExpr, isSelector := (*_typeExpr).(*ast.SelectorExpr); isSelector {
		identExpr, isIdent := (selectorExpr.X).(*ast.Ident)
		if isIdent {
			selName := identExpr.Name
			typeName := selectorExpr.Sel.Name
			var argCode jen.Code
			if isPointer {
				argCode = jen.Id(argName(name))
			} else {
				argCode = jen.Op("&").Id(argName(name))
			}
			if isInHandleTypesList(selName + packageSeparator + typeName) {
				return jen.Op("*").Id(name).Op("=").
					Id("register" + handleTypes[selName+packageSeparator+typeName] + "Handle").
					Call(argCode)
			} else {
				return jen.Op("*").Id(name).Op("=").Op("*").Parens(jen.Op("*").
					Qual("C", selName+packageSeparator+typeName)).
					Parens(jen.Qual("unsafe", "Pointer").Parens(argCode))
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

func isSkyArrayType(typeName string) bool {
	if _, ok := arrayTypes[typeName]; ok {
		return true
	}
	return false
}

func isInplaceConvertType(typeName string) bool {
	if _, ok := inplaceConvertTypesPackages[typeName]; ok {
		return true
	}
	return false

}

/* Process a type expression. Returns the code in C for the type and ok if successful */
// typedata
func processTypeExpression(fast *ast.File, type_expr ast.Expr,
	packageName string, name string,
	definedTypes *[]string,
	forwardsDeclarations *[]string, depth int,
	dependantTypes *[]string) (string, bool, bool) {
	cCode := ""
	result := false
	dependant := false
	if typeStruct, isTypeStruct := (type_expr).(*ast.StructType); isTypeStruct {
		cCode += "struct{\n"
		e := false
		for _, field := range typeStruct.Fields.List {
			var names []string
			for _, fieldName := range field.Names {
				names = append(names, fieldName.Name)
			}
			if len(names) == 0 {
				names = append(names, "_unnamed")
			}
			for _, fieldName := range names {
				for i := 0; i < depth*4; i++ {
					cCode += " "
				}
				typeCode, result, isFieldDependant := processTypeExpression(fast, field.Type, packageName, fieldName,
					definedTypes, forwardsDeclarations, depth+1, dependantTypes)
				if result {
					if isFieldDependant {
						dependant = true
					}
					cCode += typeCode
				} else {
					e = true
				}
				cCode += ";\n"
			}

		}
		for i := 0; i < (depth-1)*4; i++ {
			cCode += " "
		}
		cCode += "} "
		typeName := name
		if depth == 1 {
			typeName = packageName + packageSeparator + typeName
		}
		cCode += typeName
		if dependant && depth == 1 {
			addDependant(dependantTypes, typeName)
		}
		result = !e
	} else if arrayExpr, isArray := (type_expr).(*ast.ArrayType); isArray {
		var arrayCode string
		var arrayElCode string
		result = false
		newName := name
		if depth == 1 {
			newName = packageName + packageSeparator + name
		}
		if arrayExpr.Len == nil {
			arrayCode = newName
			arrayElCode = "GoSlice_ "
			result = true
		} else if litExpr, isLit := (arrayExpr.Len).(*ast.BasicLit); isLit {
			arrayElCode, result, dependant = processTypeExpression(fast, arrayExpr.Elt, packageName, "",
				definedTypes, forwardsDeclarations, depth+1, dependantTypes)
			if result {
				arrayCode = newName + "[" + litExpr.Value + "]"
			}
		}
		if result {
			if dependant && depth == 1 {
				addDependant(dependantTypes, newName)
			}
			cCode += arrayElCode + " " + arrayCode
		}
	} else if _, isFunc := (type_expr).(*ast.FuncType); isFunc {
		newName := name
		if depth == 1 {
			newName = packageName + packageSeparator + name
		}
		cCode += "Handle " + newName
		result = true
		dependant = true
	} else if _, isIntf := (type_expr).(*ast.InterfaceType); isIntf {
		newName := name
		if depth == 1 {
			newName = packageName + packageSeparator + name
		}
		cCode += "GoInterface_ " + newName
		result = true
		dependant = true
	} else if _, isChan := (type_expr).(*ast.ChanType); isChan {
		newName := name
		if depth == 1 {
			newName = packageName + packageSeparator + name
		}
		cCode += "GoChan_ " + newName
		result = true
		dependant = true
	} else if _, isMap := (type_expr).(*ast.MapType); isMap {
		newName := name
		if depth == 1 {
			newName = packageName + packageSeparator + name
		}
		cCode += "GoMap_ " + newName
		result = true
		dependant = true
	} else if starExpr, isStart := (type_expr).(*ast.StarExpr); isStart {
		targetTypeExpr := starExpr.X
		typeCode, ok, isFieldDependant := processTypeExpression(fast, targetTypeExpr, packageName, "",
			definedTypes, forwardsDeclarations, depth+1, dependantTypes)
		if ok {
			if isFieldDependant {
				dependant = true
			}
			cCode += typeCode
			newName := name
			if depth == 1 {
				newName = packageName + packageSeparator + name
			}
			cCode += "* " + newName
			if dependant && depth == 1 {
				addDependant(dependantTypes, newName)
			}
			result = true
		}
	} else if identExpr, isIdent := (type_expr).(*ast.Ident); isIdent {
		typeCode, isBasic := GetCTypeFromGoType(identExpr.Name)
		if !isBasic {
			addDependency := false
			if packageName != fast.Name.Name && !isSkycoinName(packageName) {
				if cfg.DependOnlyExternal {
					if isExternalName(packageName) {
						addDependency = true
					}
				} else {
					addDependency = true
				}
			}
			typeCode = packageName + packageSeparator + typeCode
			if addDependency {
				addDependant(dependantTypes, typeCode)
				dependant = true
			}
		}
		cCode = typeCode
		cCode += " "
		newName := name
		if depth == 1 {
			newName = packageName + packageSeparator + name
		}
		cCode += newName
		if !dependant {
			if isDependantType(dependantTypes, typeCode) {
				dependant = true
			}
		}
		typeFound := false
		for _, definedType := range *definedTypes {
			if definedType == typeCode {
				typeFound = true
			}
		}
		if dependant && depth == 1 {
			addDependant(dependantTypes, newName)
		}
		if !typeFound {
			if forwardsDeclarations != nil {
				*forwardsDeclarations = append(*forwardsDeclarations, identExpr.Name)
				result = true
			} else {
				result = false
			}
		} else {
			result = true
		}
	} else if selectorExpr, isSelector := (type_expr).(*ast.SelectorExpr); isSelector {
		externPackage := packageName
		identExpr, isIdent := (selectorExpr.X).(*ast.Ident)
		if isIdent {
			externPackage = identExpr.Name
		}
		newName := name
		if depth == 1 {
			newName = packageName + packageSeparator + name
		}
		typeCode, ok, isFieldDependant := processTypeExpression(fast, selectorExpr.Sel, externPackage, newName,
			definedTypes, forwardsDeclarations, depth+1, dependantTypes)
		if isFieldDependant {
			dependant = true
		}
		if dependant && depth == 1 {
			addDependant(dependantTypes, newName)
		}
		if ok {
			cCode = typeCode
			result = true
		}
	}
	return cCode, result, dependant
}

func isDependantType(dependantTypes *[]string, typeName string) bool {
	for _, t := range *dependantTypes {
		if t == typeName {
			return true
		}
	}
	return false
}

func addDependant(dependantTypes *[]string, typeName string) {
	for _, t := range *dependantTypes {
		if t == typeName {
			return
		}
	}
	*dependantTypes = append(*dependantTypes, typeName)
}

/* Process a type definition in GO and returns the c code for the definition */
func processTypeDef(fast *ast.File, tdecl *ast.GenDecl,
	definedTypes *[]string, forwardsDeclarations *[]string,
	dependantTypes *[]string) (string, bool, bool) {
	resultCode := ""
	result := true
	isDependant := false
	for _, s := range tdecl.Specs {
		if typeSpec, isTypeSpec := (s).(*ast.TypeSpec); isTypeSpec {
			typeCCode, ok, isDependantExpr := processTypeExpression(fast, typeSpec.Type,
				fast.Name.Name, typeSpec.Name.Name, definedTypes, forwardsDeclarations, 1,
				dependantTypes)
			if ok {
				if isDependantExpr {
					isDependant = true
				}
				resultCode += "typedef "
				resultCode += typeCCode
				resultCode += ";\n"
				*definedTypes = append(*definedTypes, fast.Name.Name+packageSeparator+typeSpec.Name.Name)
			} else {
				result = false
			}
		}
	}
	return resultCode, result, isDependant
}

/* Process all type definitions. Returns c code for all the defintions */
func processTypeDefs(fast *ast.File, typeDecls []*ast.GenDecl, dependantTypes *[]string) string {
	resultCode := ""
	var definedTypes []string
	for key := range GetBasicTypes() {
		ctype, ok := GetCTypeFromGoType(key)
		if ok {
			definedTypes = append(definedTypes, ctype)
		}
	}

	unprocessed := len(typeDecls)
	wentBlank := false
	for unprocessed > 0 && !wentBlank {
		wentBlank = true
		for index, typeDecl := range typeDecls {
			if typeDecl != nil {
				typeCode, ok, isDependant := processTypeDef(fast, typeDecl, &definedTypes, nil, dependantTypes)
				if ok {
					wentBlank = false
					typeDecls[index] = nil
					if !(cfg.IgnoreDependants && isDependant) {
						resultCode += typeCode
					}
					unprocessed -= 1
				}
			}
		}
	}

	//TODO: if unprocessed > 0 then there are cyclic type references. Use forward declarations.
	var forwardsDeclarations []string
	if unprocessed > 0 {
		for _, typeDecl := range typeDecls {
			if typeDecl != nil {
				typeCode, ok, isDependant := processTypeDef(fast, typeDecl, &definedTypes, &forwardsDeclarations, dependantTypes)
				if ok {
					if !(cfg.IgnoreDependants && isDependant) {
						resultCode += typeCode
					}
				}
			}
		}
	}
	return resultCode
}

//Remove extra space in export indication
func fixExportComment(filePath string) {
	f, err := os.Open(filePath)
	check(err)
	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(f)
	check(err)

	contents := buf.String()
	err = f.Close()
	check(err)

	contents = strings.Replace(contents, "// export SKY_", "//export SKY_", -1)
	f, err = os.Create(filePath)
	check(err)
	_, err = f.WriteString(contents)
	check(err)
	err = f.Sync()
	check(err)
	err = f.Close()
	check(err)
}

func processTypeSetting(comment string) {
	handlePrefix := "CGOGEN HANDLES "
	typeConversionPrefix := "CGOGEN TYPES_CONVERSION "
	if strings.HasPrefix(comment, handlePrefix) {
		handlesPart := comment[len(handlePrefix):]
		handles := strings.Split(handlesPart, ",")
		for _, handle := range handles {
			handleParts := strings.Split(handle, "|")
			if len(handleParts) > 1 {
				handleTypes[handleParts[0]] = handleParts[1]
			} else if len(handleParts) > 0 {
				handleTypes[handleParts[0]] = handleParts[0]
			}
		}
	} else if strings.HasPrefix(comment, typeConversionPrefix) {
		typesPart := comment[len(typeConversionPrefix):]
		types := strings.Split(typesPart, ",")
		for _, t := range types {
			typesPart := strings.Split(t, "|")
			if len(typesPart) > 1 {
				customTypesMap[typesPart[0]] = typesPart[1]
			} else if len(typesPart) > 0 {
				customTypesMap[typesPart[0]] = typesPart[0]
			}
		}
	}
}

func IsBasicGoType(goType string) bool {
	if _, ok := basicTypesMap[goType]; ok {
		return true
	} else {
		return false
	}
}

/* Returns the corresponding C type for a GO type*/
func GetCTypeFromGoType(goType string) (string, bool) {
	if val, ok := basicTypesMap[goType]; ok {
		return val, true
	} else {
		return goType, false
	}
}

func GetBasicTypes() map[string]string {
	return basicTypesMap
}

var basicTypesMap = map[string]string{
	"int":        "GoInt_",
	"uint":       "GoUint_",
	"int8":       "GoInt8_",
	"int16":      "GoInt16_",
	"int32":      "GoInt32_",
	"int64":      "GoInt64_",
	"byte":       "GoUint8_",
	"uint8":      "GoUint8_",
	"uint16":     "GoUint16_",
	"uint32":     "GoUint32_",
	"uint64":     "GoUint64_",
	"float32":    "GoFloat32_",
	"float64":    "GoFloat64_",
	"complex64":  "GoComplex64_",
	"complex128": "GoComplex128_",
	"string":     "GoString_",
	"bool":       "bool",
	"error":      "GoInt32_",
}

var packageSeparator = "__"

func getCallbackCode(varName string) *jen.Statement {

	varFunction := jen.Func().Params(
		jen.Id("pTx").Id("*coin.Transaction"),
	).Parens(jen.Uint64().Op(",").Error()).Block(
		jen.Var().Id("fee").Id("C.GoUint64_"),
		jen.Id("handle").Op(":=").Id("registerTransactionHandle").Call(jen.Id("pTx")),
		jen.Id("result").Op(":=").Id("C.callFeeCalculator").Call(jen.Id("_"+varName), jen.Id("handle"), jen.Id("&fee")),
		jen.Id("closeHandle").Call(jen.Id("Handle").Call(jen.Id("handle"))),
		jen.If(jen.Id("result").Op("==").Id("SKY_OK")).Block(
			jen.Return(jen.Id("uint64").Call(jen.Id("fee")), jen.Nil())),
		jen.Return(jen.Lit(0), jen.Qual("errors", "New").Call(jen.Lit("Error calculating fee"))),
	)

	return varFunction
}
