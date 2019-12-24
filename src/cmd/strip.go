package main

import (
	"flag"
	"fmt"
	"os"
	"io"
	"strings"
)

func main(){
	var path string
	flag.StringVar(&path, "i", "", "PATH to source file")
	flag.Parse()
	fmt.Println(path)
	dest := strings.Replace(path, "./", "", -1)
	dest = strings.Replace(dest, "/", ".", -1)
	copyFile(path, "../../test/" + dest)
}

func copyFile(source string, dest string){
	sf, err := os.Open(source)
	check(err)
	defer sf.Close()
	df, err := os.Create(dest)
	check(err)
	defer df.Close()
	io.Copy(df, sf)
	df.Sync()
}

