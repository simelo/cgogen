package main

import (
	"go/ast"
	"reflect"
	"go/token"
)

type CCompiler struct{
	source *ast.File
	ccode  *CCode
}

type CCode struct {
	
}

func NewCompiler(source *ast.File) (compiler *CCompiler) {
	compiler = &CCompiler{}
	compiler.source = source
	compiler.ccode = &CCode{}
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
				c.processUnknown(&_decl)
			}
		} else {
			c.processUnknown(&_decl)
		}
	}
}

func (c *CCompiler) processUnknown(decl *ast.Decl){
	s := reflect.ValueOf(decl).Elem()
	typeOfT := s.Type()
	applog("Don't know what to do with: %s", typeOfT)
}

func (c *CCompiler) processImport(decl *ast.GenDecl){
}

func (c *CCompiler) processType(tdecl *ast.GenDecl){
	for _, s := range tdecl.Specs{
		if _, isTypeSpec := (s).(*ast.TypeSpec); isTypeSpec {
		}
	}
}

func (c *CCompiler) processFunction(decl *ast.FuncDecl){
}
