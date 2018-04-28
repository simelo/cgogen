package main

import (
	"strings"
)

func (f *Function) createSignature() string {
	signature := f.returnType + " " + f.name + "("
	var paramsCode []string
	for _, p := range f.parameters {
		paramsCode = append( paramsCode, p.ccode )
	}
	signature += strings.Join( paramsCode, ", " )
	signature += ")"
	return signature
}