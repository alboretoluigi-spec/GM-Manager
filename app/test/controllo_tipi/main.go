package main

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
)

var root = os.Args[1]
var fset = token.NewFileSet()
var cache = map[string]*types.Package{}
var std = importer.ForCompiler(fset, "source", nil)
var target = os.Args[2]

type imp struct{}

func (imp) Import(p string) (*types.Package, error) {
	if strings.HasPrefix(p, "gasterbless/") {
		return check(filepath.Join(root, strings.TrimPrefix(p, "gasterbless/")), p, false)
	}
	if !strings.Contains(strings.Split(p, "/")[0], ".") {
		return std.Import(p)
	}
	return nil, fmt.Errorf("esterno %s", p)
}

func check(dir, path string, tests bool) (*types.Package, error) {
	if pk, ok := cache[path]; ok && !tests {
		return pk, nil
	}
	ents, _ := os.ReadDir(dir)
	var files []*ast.File
	for _, e := range ents {
		n := e.Name()
		if !strings.HasSuffix(n, ".go") || (strings.HasSuffix(n, "_test.go") && !tests) {
			continue
		}
		if strings.HasSuffix(n, "_windows.go") || strings.Contains(n, "_windows_") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(dir, n), nil, parser.ParseComments)
		if err != nil {
			fmt.Println("PARSE", err)
			continue
		}
		if strings.HasSuffix(n, "_test.go") && strings.HasSuffix(f.Name.Name, "_test") {
			continue
		}
		// salta file con vincoli di build windows
		skip := false
		for _, c := range f.Comments {
			for _, l := range c.List {
				if strings.HasPrefix(l.Text, "//go:build") && strings.Contains(l.Text, "windows") && !strings.Contains(l.Text, "!windows") {
					skip = true
				}
			}
		}
		if !skip {
			files = append(files, f)
		}
	}
	conf := types.Config{Importer: imp{}, Error: func(err error) {
		if path == target {
			fmt.Println(err)
		}
	}}
	pk, _ := conf.Check(path, fset, files, nil)
	if !tests {
		cache[path] = pk
	}
	return pk, nil
}

func main() {
	check(filepath.Join(root, strings.TrimPrefix(target, "gasterbless/")), target, len(os.Args) > 3)
}
