package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strconv"
	"strings"
)

func main() {
	failed := false
	for _, file := range os.Args[1:] {
		if err := strip(file); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", file, err)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

func directive(text string) bool {
	return slices.ContainsFunc([]string{"//go:", "//export ", "//line ", "//nolint", "// +build"}, func(p string) bool {
		return strings.HasPrefix(text, p)
	})
}

func strip(file string) error {
	src, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, parser.ParseComments)
	if err != nil {
		return err
	}
	if ast.IsGenerated(f) {
		return nil
	}
	cgo := map[*ast.CommentGroup]bool{}
	for _, decl := range f.Decls {
		if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.IMPORT {
			for _, spec := range gen.Specs {
				if spec.(*ast.ImportSpec).Path.Value == strconv.Quote("C") {
					cgo[gen.Doc] = true
					cgo[spec.(*ast.ImportSpec).Doc] = true
				}
			}
		}
	}
	type span struct{ start, end int }
	var cuts []span
	for _, group := range f.Comments {
		if cgo[group] {
			continue
		}
		for _, c := range group.List {
			if directive(c.Text) {
				continue
			}
			start, end := fset.Position(c.Pos()).Offset, fset.Position(c.End()).Offset
			lineStart := bytes.LastIndexByte(src[:start], '\n') + 1
			lineEnd := len(src)
			if i := bytes.IndexByte(src[end:], '\n'); i >= 0 {
				lineEnd = end + i
			}
			if len(bytes.TrimSpace(src[lineStart:start])) == 0 && len(bytes.TrimSpace(src[end:lineEnd])) == 0 {
				cuts = append(cuts, span{lineStart, min(lineEnd+1, len(src))})
				continue
			}
			for start > lineStart && (src[start-1] == ' ' || src[start-1] == '\t') {
				start--
			}
			cuts = append(cuts, span{start, end})
		}
	}
	if len(cuts) == 0 {
		return nil
	}
	var out []byte
	last := 0
	for _, c := range cuts {
		if c.start < last {
			c.start = last
		}
		out = append(out, src[last:c.start]...)
		last = c.end
	}
	out = append(out, src[last:]...)
	formatted, err := format.Source(out)
	if err != nil {
		return fmt.Errorf("after stripping: %w", err)
	}
	return os.WriteFile(file, formatted, 0o644)
}
