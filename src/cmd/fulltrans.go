package main

import (
	"io/ioutil"
	"os"
	//"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"path/filepath"
)

func FullTranspile(sourcedir string, outdir string){
	applog("Processing dir %s", sourcedir)
	compilers := make( map[string]*CCompiler)
	fset := token.NewFileSet()
	traverseDir(sourcedir, func(file string) error{
		fo, err := os.Open(file)
		applog("opening %s", file)
		if err != nil {
			reportError("error: %v", err)
			return err
		}
		defer fo.Close()
		fast, err := parser.ParseFile(fset, "", fo, parser.AllErrors | parser.ParseComments)
		if err == nil {
			packName := fast.Name.Name
			var compiler *CCompiler
			var found bool
			if compiler, found = compilers[packName]; !found {
				compiler = NewCompiler()
				compilers[packName] = compiler
			}
			compiler.Compile(fast)
		} else {
			reportError("error: %v", err)
		}
		return nil
	})
	generateCode( compilers, outdir)
}

func generateCode(compilers map[string]*CCompiler, outdir string){
	cleanDir(outdir)
	for pack, compiler := range compilers{
		headerName := pack + ".h"
		headerCode := compiler.GetHeaderCode()
		path := filepath.Join(outdir, headerName)
		saveToFile( path, headerCode )
		fileName := pack + ".c"
		cCode := compiler.GetCCode()
		path = filepath.Join(outdir, fileName)
		saveToFile( path, cCode )
	}
}

func cleanDir(dir string){
	traverseDir(dir, func(file string) error{
		return os.RemoveAll(file)
	})
}

func traverseDir(sourcedir string, callback func(file string) error ) error {
	files, err := ioutil.ReadDir(sourcedir)
	if err != nil {
		return err
	}
	for _, f := range files {
		if f.Mode().IsRegular() {
			name := f.Name()
			if strings.HasSuffix(name, ".go"){
				path := filepath.Join(sourcedir, name)
				callback(path)
			}
		}
	}
	return nil
}

func saveToFile(fileName string, text string){
	f, err := os.Create(fileName)
	check(err)
	defer f.Close()
	f.WriteString( text )
	f.Sync()
}