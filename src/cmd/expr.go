package main

import (
	//"strings"
	"go/ast"
	"go/token"
	"fmt"
	"reflect"
)

func (c *CCompiler) generateExpression(expr ast.Expr) (code string, resultType string, typeSuffix string, ok bool) {
	typeSuffix = ""
	ok = true
	if litExpr, isLit := (expr).(*ast.BasicLit); isLit {
		return c.generateLiteral(*litExpr)
	} else if binExpr, isBinary := (expr).(*ast.BinaryExpr); isBinary {
		return c.generateBinary( *binExpr )
	} else if parensExpr, isParens := (expr).(*ast.ParenExpr); isParens {
		code, resultType, typeSuffix, ok = c.generateExpression(parensExpr.X)
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

func  (c *CCompiler) generateIdentExpr(identExpr ast.Ident) (code string, resultType string, typeSuffix string, ok bool){
	vardef := c.findVar(identExpr.Name, "")
	if vardef != nil {
		code = identExpr.Name
		resultType = vardef.ctype
		typeSuffix = vardef.ctypesuffix
		ok = true
	} else if constdef := c.findConst(identExpr.Name, ""); constdef != nil {
		code = identExpr.Name
		resultType = constdef.ctype
		typeSuffix = constdef.ctypesuffix
		ok = true
	} else if funcDef := c.findFunction(identExpr.Name, ""); funcDef != nil {
		code = identExpr.Name
		resultType = ""
		typeSuffix = ""
		ok = true
	} else {
		applog("Identifier not found %s", identExpr.Name)
	}
	return
}

func  (c *CCompiler) generateBinary(binExpr ast.BinaryExpr) (code string, resultType string, typeSuffix string, ok bool){
	leftExpr, leftType, leftS, okLeft := c.generateExpression(binExpr.X)
	rightExpr, rightType, rightS, okRight := c.generateExpression(binExpr.Y)
	if okLeft && okRight {
		resultType, typeSuffix, ok = mixTypes(leftType, rightType, leftS, rightS)
		if ok {
			code = fmt.Sprintf("%s %s %s", leftExpr, binExpr.Op, rightExpr)
		} else {
			applog("Applying operand %s to different types %s and %s", 
				binExpr.Op, leftType + leftS, rightType + rightS)
		}
	}
	return
}

func  (c *CCompiler) generateLiteral(litExpr ast.BasicLit) (code string, resultType string, typeSuffix string, ok bool){
	typeSuffix = ""
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

func (c *CCompiler) generateCallExpr(callExpr ast.CallExpr) (code string, resultType string, typeSuffix string, ok bool){
	_, _, _, okFunc := c.generateExpression(callExpr.Fun)
	//Ignore func type for now
	if okFunc {
		/*for _, arg := range callExpr.Args {
		}*/
	}
	return
}

func mixTypes(type1 string, type2 string, suffix1 string, suffix2 string) (string, string, bool) {
	if type1 == type2 && suffix1 == suffix2 {
		return type1, suffix1, true
	} else if (suffix1 == "" && suffix2 == ""){
		if (type1 == "GoFloat64_" || type2 == "GoFloat64_") && (type1 == "GoInt64_" || type2 == "GoInt64_") {
			return "GoFloat64_", "", true
		}
	} 
	return "", "", false
}