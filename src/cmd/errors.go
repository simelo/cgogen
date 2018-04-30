package main

import "fmt"

func reportError(msg string, a ...interface{}){
	fmt.Println(fmt.Sprintf(msg, a))
}