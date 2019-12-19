package main

import (
	"strings"
	"go/ast"
	"go/token"
	"reflect"
	"fmt"
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
		stmtCode := c.generateStatement(stmt) + "\n"
		for _, init := range c.initializers {
			code += init
		}
		c.initializers = nil
		code += stmtCode
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
				const_code := ""
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
						tc == "GoInt64_" || tc == "GoString_" {
						const_code += "#define " + sname + " " + value + "\n"
					} else {
						const_code += buildTypeWithVarName( tc , sname )
						const_code += " = " + value
						const_code += ";\n"
					}
					stack := c.getTopOfStack()
					consDef := ConstDef{ccode: const_code, 
							name : sname, 
							ctype : tc}
					stack.constdefs = append(stack.constdefs, consDef)
				}
				code += const_code
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
				varcode := ""
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
					varcode += buildTypeWithVarName( tc , sname)
					initcode := ""
					if valueok && value != "" {
						varcode += " = " + value
					} else {
						initcode = fmt.Sprintf("memset(&%s, 0, sizeof(%s));\n",
							sname, sname)
					}
					varcode += ";\n"
					varcode += initcode
					varDef := VarDef{ccode: varcode, name : sname, 
						ctype : tc}
					stack := c.getTopOfStack()
					stack.vardefs = append(stack.vardefs, varDef)
				}
				code += varcode
			}
			
		} else {
			x := reflect.ValueOf(s).Elem()
			typeOfT := x.Type()
			reportError("Don't know what to do with: %s", typeOfT)
		}
	}
	return code
}

