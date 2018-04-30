package main

import (
	//"strings"
	"go/ast"
	"go/token"
	"fmt"
	"reflect"
)

func (c *CCompiler) generateExpression(expr ast.Expr) (code string, resultType string, ok bool) {
	ok = true
	if litExpr, isLit := (expr).(*ast.BasicLit); isLit {
		return c.generateLiteral(*litExpr)
	} else if binExpr, isBinary := (expr).(*ast.BinaryExpr); isBinary {
		return c.generateBinary( *binExpr )
	} else if parensExpr, isParens := (expr).(*ast.ParenExpr); isParens {
		code, resultType, ok = c.generateExpression(parensExpr.X)
		if ok {
			code = "( " + code + " )"
		} 
	} else if identExpr, isIdent := (expr).(*ast.Ident); isIdent {
		return c.generateIdentExpr(*identExpr)
	} else if callExpr, isCallExpr := (expr).(*ast.CallExpr); isCallExpr {
		return c.generateCallExpr(*callExpr)
	} else {
		ok = false									
		x := reflect.ValueOf(expr).Elem()
		typeOfT := x.Type()
		applog("Don't know what to do with expression: %s", typeOfT)
	}
	return
}

func  (c *CCompiler) generateIdentExpr(identExpr ast.Ident) (code string, resultType string, ok bool){
	vardef := c.findVar(identExpr.Name, "")
	if vardef != nil {
		code = identExpr.Name
		resultType = vardef.ctype
		ok = true
	} else if constdef := c.findConst(identExpr.Name, ""); constdef != nil {
		code = identExpr.Name
		resultType = constdef.ctype
		ok = true
	} else if funcDef := c.findFunction(identExpr.Name, ""); funcDef != nil {
		code = identExpr.Name
		resultType = ""
		ok = true
	} else {
		applog("Identifier not found %s", identExpr.Name)
	}
	return
}

func  (c *CCompiler) generateBinary(binExpr ast.BinaryExpr) (code string, resultType string, ok bool){
	leftExpr, leftType, okLeft := c.generateExpression(binExpr.X)
	rightExpr, rightType, okRight := c.generateExpression(binExpr.Y)
	if okLeft && okRight {
		resultType, ok = mixTypes(leftType, rightType)
		if ok {
			code = fmt.Sprintf("%s %s %s", leftExpr, binExpr.Op, rightExpr)
		} else {
			applog("Applying operand %s to different types %s and %s", 
				binExpr.Op, leftType, rightType)
		}
	}
	return
}

func  (c *CCompiler) generateLiteral(litExpr ast.BasicLit) (code string, resultType string, ok bool){
	ok = true
	code = litExpr.Value
	switch litExpr.Kind {
	default:
		applog("Unknown literal %s", litExpr.Kind)
		ok = false
	case token.INT:
		resultType = "GoInt32_"	
	case token.FLOAT:
		resultType = "GoFloat64_"
	case token.CHAR:
		resultType = "GoUint32_"
	case token.STRING:
		resultType = "GoString_"
	}
	return
}

func (c *CCompiler) generateCallExpr(callExpr ast.CallExpr) (code string, resultType string, ok bool){
	_, _, okFunc := c.generateExpression(callExpr.Fun)
	//Ignore func type for now
	if okFunc {
		/*for _, arg := range callExpr.Args {
		}*/
	}
	return
}

func mixTypes(type1 string, type2 string) (string, bool) {
	if type1 == type2  {
		return type1, true
	} else if (type1 == "GoFloat64_" || type2 == "GoFloat64_") && (type1 == "GoInt64_" || type2 == "GoInt64_") {
		return "GoFloat64_", true
	} 
	return "", false
}