package main

import (
	"strings"
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
	} else if compLit, isCompLit := (expr).(*ast.CompositeLit); isCompLit {
		return c.generateCompositeLiteral(*compLit)
	} else if keyValue, isKeyValue := (expr).(*ast.KeyValueExpr); isKeyValue {
		return c.generateKeyValueExpression(*keyValue)
	} else {
		ok = false									
		x := reflect.ValueOf(expr).Elem()
		typeOfT := x.Type()
		reportError("Don't know what to do with expression: %s", typeOfT)
	}
	return
}

func (c *CCompiler) generateKeyValueExpression(keyValue ast.KeyValueExpr) (code string, resultType string, ok bool){
	if identExpr, isIdent := (keyValue.Key).(*ast.Ident); isIdent {
		value, typeValue, okValue := c.generateExpression(keyValue.Value)
		if okValue {
			code = fmt.Sprintf("%s : %s", identExpr.Name, value)
			ok = true
			resultType = typeValue //resultType should be KeyValue
		} else {
			reportError("Error generating value in key value expression")
		}
	} else {
		key, typeKey, okKey := c.generateExpression(keyValue.Key)
		value, typeValue, okValue := c.generateExpression(keyValue.Value)
		if !okKey {
			reportError("Couldn't generate map key expression")
		}
		if !okValue {
			reportError("Couldn't generate map value expression")
		}
		if okKey && okValue {
			mapKeyCode := getMapTypeKeyword(  typeKey )
			mapValueCode := getMapTypeKeyword(  typeValue )
			init := fmt.Sprintf("Map%s%sKeyValue", mapKeyCode, mapValueCode)
			code = fmt.Sprintf("%s(%s,%s)", init, key, value)
			ok = true
		} 
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
		resultType = funcDef.returnType //Return type should be function
		ok = true
	} else {
		reportError("Identifier not found %s", identExpr.Name)
	}
	return
}

func  (c *CCompiler) generateBinary(binExpr ast.BinaryExpr) (code string, resultType string, ok bool){
	leftExpr, leftType, okLeft := c.generateExpression(binExpr.X)
	rightExpr, rightType, okRight := c.generateExpression(binExpr.Y)
	if okLeft && okRight {
		resultType, ok = mixTypes(leftType, rightType)
		if ok {
			if leftType == "GoString_" && rightType == "GoString_" {
				return c.generateStringBinary(binExpr, leftExpr, rightExpr)
			} else {
				code = fmt.Sprintf("%s %s %s", leftExpr, binExpr.Op, rightExpr)
			}
		} else {
			reportError("Applying operand %s to different types %s and %s", 
				binExpr.Op, leftType, rightType)
		}
	}
	return
}

func  (c *CCompiler) generateStringBinary(binExpr ast.BinaryExpr, left string, right string) (code string, resultType string, ok bool){
	resultType = "bool"
	stringOp := map[token.Token]string {
		token.ADD : "string_concat",
		token.EQL : "string_is_equal",
		token.GTR : "string_is_greater",
		token.LSS : "string_is_lesser",
		token.LEQ : "string_is_lesser_than_or_equal",
		token.GEQ : "string_is_greater_than_or_equal",
	}
	if f, found := stringOp[binExpr.Op]; found {
		if binExpr.Op == token.ADD {
			resultType = "GoString_"
		}
		ok = true
		code = fmt.Sprintf("%s(%s,%s)", f, left, right)
	} else {
		ok = false
		reportError("Invalid string operator")
	}
	return
}

func  (c *CCompiler) generateLiteral(litExpr ast.BasicLit) (code string, resultType string, ok bool){
	ok = true
	code = litExpr.Value
	switch litExpr.Kind {
	default:
		reportError("Unknown literal %s", litExpr.Kind)
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
	funcCode, funcType, okFunc := c.generateExpression(callExpr.Fun)
	if okFunc {
		var argsCode []string
		ok = true
		for index, arg := range callExpr.Args {
			argCode, _, okArg := c.generateExpression( arg )
			if okArg {
				argsCode = append( argsCode, argCode )
			} else {
				ok = false
				reportError("Couldn't generate argument %d expression", index + 1)
			}
		}
		code = fmt.Sprintf("%s( %s )", funcCode, strings.Join(argsCode, ", "))
		resultType = funcType
	} else {
		reportError("Couldn't generate call expression")
	}
	return
}

func (c *CCompiler) generateCompositeLiteral(compLit ast.CompositeLit) (code string, resultType string, ok bool) {
	typeExpr := ""
	okType := false
	if compLit.Type != nil {
		typeExpr, okType = c.processTypeExpression(compLit.Type)
	}
	
	if strings.HasPrefix(typeExpr, "Go") && strings.HasSuffix(typeExpr, "Map")  {
		initVarName := c.createIdent("_map")
		initializer, okInit := c.generateMapInitializer(compLit, typeExpr, initVarName)
		if okInit {
			c.initializers = append( c.initializers, initializer )
			code = initVarName
		}
	} else {
		var initializers []string
		for _, expr := range compLit.Elts {
			codeExpr, typeExprValue, okExpr := c.generateExpression(expr)
			if okExpr {
				if !okType {
					okType = true
					typeExpr = typeExprValue
				}
				initializers = append( initializers, codeExpr )
			} else {
				reportError("Couldn't generate initializer")
			}
		}
		initializer := strings.Join(initializers, ",")
		code = fmt.Sprintf("{%s}", initializer)
	}
	resultType = typeExpr
	ok = okType
	return
}

func (c *CCompiler) generateMapInitializer(compLit ast.CompositeLit, maptype string, varname string) (code string, ok bool){
	code = fmt.Sprintf("%s %s;\n", maptype, varname)
	code += fmt.Sprintf("map_init(&%s);\n", varname)
	for _, expr := range compLit.Elts {//traverse key value items
		if keyValue, isKeyValue := (expr).(*ast.KeyValueExpr); isKeyValue {
			key, keyType, okKey := c.generateExpression(keyValue.Key)
			value, valueType, okValue := c.generateExpression(keyValue.Value)
			if okKey && okValue {
				tkey := getMapTypeKeyword(keyType)
				tval := getMapTypeKeyword(valueType)
				mapfuncset := fmt.Sprintf("Map%s%sSet", tkey, tval)
				code += fmt.Sprintf("%s(&%s, %s, %s);\n", mapfuncset, varname, key, value)
			} else {
				reportError("Couldn't generate with map literal")
			}
		} else {
			reportError("Non key-value item is map literal")
		}
	}
	ok = true
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