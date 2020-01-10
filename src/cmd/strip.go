package main

import (
	"fmt"
	"io"
	"os"
)

func copyFile(source string, dest string) {
	sf, err := os.Open(source)
	check(err)
	defer sf.Close()
	df, err := os.Create(dest)
	check(err)
	defer df.Close()
	io.Copy(df, sf)
	df.Sync()
}

