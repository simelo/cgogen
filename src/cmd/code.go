package main

import (
	"strings"
	"go/ast"
	"go/token"
	"reflect"
)

func (c *CCompiler) createSignature(f *Function) string {
	signature := buildTypeWithVarName( f.returnType, "")  + " " + f.name + "("
	var paramsCode []string
	for _, p := range f.parameters {
		paramsCode = append( paramsCode, p.ccode )
	}
	signature += strings.Join( paramsCode, ", " )
	signature += ")"
	return signature
}

func (c *CCompiler) generateBody(f *Function, block *ast.BlockStmt) string {
	return c.generateBlock(block)
}

func (c *CCompiler) generateBlock(block *ast.BlockStmt) (code string){
	code = "{\n"
	c.pushStack()
	for _, stmt := range block.List {
		code += c.generateStatement(stmt) + "\n"
	}
	c.popStack()
	code += "}\n"
	return
}

func (c *CCompiler) generateStatement(stmt ast.Stmt) (code string) {
	if _, ok := (stmt).(*ast.AssignStmt); ok {
		code = "assignment"
	} else if decl, ok := (stmt).(*ast.DeclStmt); ok {
		declStmt := (decl.Decl).(*ast.GenDecl)
		code = c.generateDeclaration(*declStmt)
	} else {
		s := reflect.ValueOf(stmt).Elem()
		typeOfT := s.Type()
		reportError("Don't know what to do with: %s", typeOfT)
	}
	return
}

func (c *CCompiler) generateDeclaration(decl ast.GenDecl) string {
	if decl.Tok == token.CONST {
		return c.generateConst(decl)
	} else if decl.Tok == token.VAR {
		return c.generateVar(decl)
	} else {
		return "" 
	}
}

func (c *CCompiler) generateConst(decl ast.GenDecl) ( code string ) {
	code = ""
	for _, s := range decl.Specs{
		if valueSpec, isValueSpec := (s).(*ast.ValueSpec); isValueSpec {
			typecode := ""
			typeok := false
			if valueSpec.Type != nil {
				typecode, typeok = c.processTypeExpression( valueSpec.Type)
			}
			if !typeok {
				typecode = "" 
			}
			for index, name := range valueSpec.Names {
				sname := name.Name
				if sname == "_" {
					sname = c.createIdent("var")
				}
				tc := typecode
				ntok := typeok
				ntc := ""
				value := ""
				valueok := false
				if index < len(valueSpec.Values) {
					value, ntc, valueok = c.generateExpression(valueSpec.Values[index])
					if valueok && tc == "" {
						tc = ntc
						ntok = true
					}
				}
				if ntok && valueok {
					if tc == "GoUint32_" || tc == "GoFloat32_" || 
						tc == "GoUint64_" || 
						tc == "GoFloat32_" || tc == "GoInt32_" ||
						tc == "GoInt64_" {
						code += "#define " + sname + " " + value + "\n"
					} else {
						code += buildTypeWithVarName( tc , sname )
						code += " = " + value
						code += ";\n"
					}
					stack := c.getTopOfStack()
					consDef := ConstDef{ccode: code, 
							name : sname, 
							ctype : tc}
					stack.constdefs = append(stack.constdefs, consDef)
				}
			}
		} else {
			x := reflect.ValueOf(s).Elem()
			typeOfT := x.Type()
			reportError("Don't know what to do with: %s", typeOfT)
		}
	}
	return code
}

func (c *CCompiler) generateVar(decl ast.GenDecl) (code string) {
	code = ""
	for _, s := range decl.Specs{
		if valueSpec, isValueSpec := (s).(*ast.ValueSpec); isValueSpec {
			typecode := ""
			typeok := false
			if valueSpec.Type != nil {
				typecode, typeok = c.processTypeExpression( valueSpec.Type)
			}
			if !typeok {
				typecode = "" 
			}
			for index, name := range valueSpec.Names {
				sname := name.Name
				if sname == "_" {
					sname = c.createIdent("var")
				}
				tc := typecode
				ntok := typeok
				ntc := ""
				value := ""
				valueok := false
				if index < len(valueSpec.Values) {
					value, ntc, valueok = c.generateExpression(valueSpec.Values[index])
					if valueok && tc == "" {
						tc = ntc
						ntok = true
					}
				}
				if ntok {
					code += buildTypeWithVarName( tc , sname)
					if valueok && value != "" {
						code += " = " + value
					}
					code += ";\n"
					varDef := VarDef{ccode: code, name : sname, 
						ctype : tc}
					stack := c.getTopOfStack()
					stack.vardefs = append(stack.vardefs, varDef)
				}
			}
		} else {
			x := reflect.ValueOf(s).Elem()
			typeOfT := x.Type()
			reportError("Don't know what to do with: %s", typeOfT)
		}
	}
	return code
}

