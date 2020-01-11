package main

import "fmt"

func reportError(msg string, a ...interface{}) {
	if len(a) > 0 {
		fmt.Println(fmt.Sprintf(string(msg), a))
	} else {
		fmt.Println(msg)
	}
}
