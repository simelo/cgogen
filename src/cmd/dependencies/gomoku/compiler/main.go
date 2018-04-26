// Copyright 2017 Leandro A. F. Pereira. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package compiler

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/tools/go/loader"
)

type Compiler struct {
	gen *cppGen

	conf    loader.Config
	program *loader.Program

	outDir string

	symFilter symbolFilter
	
	idents int
	
	forwardDeclarations []string
	anonymouseTypes []string
}

type symbolFilter struct {
	generated map[*types.Scope]map[string]struct{}
}

func newSymbolFilter() symbolFilter {
	return symbolFilter{generated: make(map[*types.Scope]map[string]struct{})}
}

func (sf *symbolFilter) once(scope *types.Scope, symbol string) bool {
	if s, ok := sf.generated[scope]; ok {
		if _, ok = s[symbol]; ok {
			return false
		}
	} else {
		sf.generated[scope] = make(map[string]struct{})
	}

	sf.generated[scope][symbol] = struct{}{}
	return true
}

func NewCompiler(args []string, outDir string) (*Compiler, error) {
	comp := Compiler{
		outDir: outDir,
		conf: loader.Config{
			TypeChecker: types.Config{
				IgnoreFuncBodies: true,
				Importer:         importer.Default(),
			},
			AllowErrors: false,
		},
		symFilter: newSymbolFilter(),
	}

	_, err := comp.conf.FromArgs(args[1:], false)
	if err != nil {
		return nil, errors.Wrapf(err, "could not create program loader: %s", err)
	}

	comp.program, err = comp.conf.Load()
	if err != nil {
		return nil, errors.Wrapf(err, "could not load program: %s", err)
	}

	return &comp, nil
}

func (c *Compiler) genFile(name string, pkg *loader.PackageInfo, ast *ast.File, out func(*cppGen) error) error {
	f, err := os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	
	auxName := fmt.Sprintf("%s_aux.h", ast.Name.Name)
	
	cGen := cppGen{
		fset:                    c.program.Fset,
		ast:                     ast,
		pkg:                     pkg.Pkg,
		inf:                     pkg.Info,
		output:                  f,
		fileName:				 name,	
		auxName:				 auxName,
		symFilter:               &c.symFilter,
		typeAssertFuncGenerated: make(map[string]struct{}),
	}
	
	cGen.idents = c.idents
	result := out(&cGen)
	c.idents = cGen.idents
	c.forwardDeclarations = append(c.forwardDeclarations, cGen.forwardDeclarations...)
	c.anonymouseTypes = append(c.anonymouseTypes, cGen.anonymouseTypes...)
	return result
}

func createAuxFile(name string, forwards []string, anomTypes []string) error{
	var faux *os.File
	faux = nil
	var err error
	faux, err = os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	for _, f := range forwards {
		fmt.Fprintf(faux, "struct %s;\n", f)
	}
	for _, at := range anomTypes {
		fmt.Fprintf(faux, at)
	}
	defer faux.Close()
	return nil
	
}

func (c *Compiler) genPackage(pkg *loader.PackageInfo) error {
	genImpl := func(gen *cppGen) error { return gen.GenerateImpl() }
	genHdr := func(gen *cppGen) error { return gen.GenerateHdr() }

	
	for _, ast := range pkg.Files {
		name := fmt.Sprintf("%s.cpp", ast.Name.Name)
		err := c.genFile(filepath.Join(c.outDir, name), pkg, ast, genImpl)
		if err != nil {
			return err
		}

		name = fmt.Sprintf("%s.h", ast.Name.Name)
		err = c.genFile(filepath.Join(c.outDir, name), pkg, ast, genHdr)
		if err != nil {
			return err
		}
		name = fmt.Sprintf("%s_aux.h", ast.Name.Name)
		createAuxFile(filepath.Join(c.outDir, name), c.forwardDeclarations, c.anonymouseTypes)
	}

	return nil
}

func getAuxFileName(name string) string{
	if strings.HasSuffix(name, ".h"){
		return name[:len(name)-2] + "_aux.h"
	} else if strings.HasSuffix(name, ".cpp"){
		return name[:len(name)-4] + "_aux.h"
	} else {
		return name + "_aux.h"
	}
}

func clearDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer dir.Close()

	entries, err := dir.Readdirnames(-1)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if err = os.RemoveAll(filepath.Join(path, entry)); err != nil {
			return err
		}
	}

	return nil
}

func (c *Compiler) Compile() error {
	if err := clearDirectory(c.outDir); err != nil {
		return err
	}
	os.Remove(c.outDir)
	if err := os.Mkdir(c.outDir, 0755); err != nil {
		return err
	}

	for _, pkg := range c.program.AllPackages {
		if pkg.Pkg.Name() == "runtime" {
			continue
		}

		if !pkg.Pkg.Complete() {
			return fmt.Errorf("package %s is not complete", pkg.Pkg.Name())
		}

		if err := c.genPackage(pkg); err != nil {
			return fmt.Errorf("could not generate code for package %s: %s", pkg.Pkg.Name(), err)
		}
	}

	return nil
}
