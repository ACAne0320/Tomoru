package main

import (
	"bufio"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

var updateComments = flag.Bool("update-comments", false, "lower comments.txt to the comments left")

const commentBaseline = "comments.txt"

var commentSkip = []string{"spikes/", "third_party/"}

func TestComments(t *testing.T) {
	got, err := countRepoComments("..")
	if err != nil {
		t.Fatal(err)
	}
	if *updateComments {
		want, err := readCommentBaseline(commentBaseline)
		if err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		for file, n := range got {
			if want != nil && n > want[file] {
				t.Errorf("%s: %d comment lines, the baseline allows %d", file, n, want[file])
			}
		}
		if t.Failed() {
			return
		}
		if err := writeCommentBaseline(commentBaseline, got); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := readCommentBaseline(commentBaseline)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]bool{}
	for file := range got {
		files[file] = true
	}
	for file := range want {
		files[file] = true
	}
	for _, file := range slices.Sorted(maps.Keys(files)) {
		switch n, allowed := got[file], want[file]; {
		case n > allowed:
			t.Errorf("%s: %d comment lines, the baseline allows %d; say it in code, or record the decision in docs/adr", file, n, allowed)
		case n < allowed:
			t.Errorf("%s: %d comment lines, the baseline still allows %d; lower it: go test -count=1 comments_test.go -update-comments", file, n, allowed)
		}
	}
}

func countRepoComments(root string) (map[string]int, error) {
	out, err := exec.Command("git", "-C", root, "ls-files", "--cached", "--others", "--exclude-standard").Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	counts := map[string]int{}
	for _, file := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if slices.ContainsFunc(commentSkip, func(p string) bool { return strings.HasPrefix(file, p) }) {
			continue
		}
		count := commentCounter(file)
		if count == nil {
			continue
		}
		src, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		n, err := count(file, src)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		if n > 0 {
			counts[file] = n
		}
	}
	return counts, nil
}

func commentCounter(file string) func(string, []byte) (int, error) {
	switch path.Ext(file) {
	case ".go":
		return goCommentLines
	case ".swift", ".c", ".h", ".js", ".mjs":
		return func(_ string, src []byte) (int, error) { return cCommentLines(src, true), nil }
	case ".css":
		return func(_ string, src []byte) (int, error) { return cCommentLines(src, false), nil }
	case ".sh", ".py", ".yml", ".yaml":
		return func(_ string, src []byte) (int, error) { return hashCommentLines(src), nil }
	case ".html":
		return func(_ string, src []byte) (int, error) { return htmlCommentLines(src), nil }
	}
	return nil
}

func goDirective(text string) bool {
	return slices.ContainsFunc([]string{"//go:", "//export ", "//line ", "//nolint", "// +build"}, func(p string) bool {
		return strings.HasPrefix(text, p)
	})
}

func goCommentLines(file string, src []byte) (int, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, file, src, parser.ParseComments)
	if err != nil {
		return 0, err
	}
	if ast.IsGenerated(f) {
		return 0, nil
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
	lines := map[int]bool{}
	for _, group := range f.Comments {
		if cgo[group] {
			continue
		}
		for _, c := range group.List {
			if goDirective(c.Text) {
				continue
			}
			for l := fset.Position(c.Pos()).Line; l <= fset.Position(c.End()).Line; l++ {
				lines[l] = true
			}
		}
	}
	return len(lines), nil
}

func cCommentLines(src []byte, lineComments bool) int {
	lines := map[int]bool{}
	line := 1
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case c == '\n':
			line++
		case c == '"' && strings.HasPrefix(string(src[i:]), `"""`):
			end := strings.Index(string(src[i+3:]), `"""`)
			if end < 0 {
				return len(lines)
			}
			line += strings.Count(string(src[i:i+3+end]), "\n")
			i += 3 + end + 2
		case c == '"' || c == '\'' || c == '`':
			for i++; i < len(src) && src[i] != c; i++ {
				if src[i] == '\\' {
					i++
				} else if src[i] == '\n' {
					if c != '`' {
						break
					}
					line++
				}
			}
			if i < len(src) && src[i] == '\n' {
				line++
			}
		case c == '/' && i+1 < len(src) && src[i+1] == '/' && lineComments:
			start := i
			for i < len(src) && src[i] != '\n' {
				i++
			}
			if !(line == 1 && strings.HasPrefix(string(src[start:i]), "// swift-tools-version")) {
				lines[line] = true
			}
			i--
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			lines[line] = true
			for i += 2; i < len(src) && !(src[i] == '*' && i+1 < len(src) && src[i+1] == '/'); i++ {
				if src[i] == '\n' {
					line++
					lines[line] = true
				}
			}
			i++
		}
	}
	return len(lines)
}

func hashCommentLines(src []byte) int {
	n := 0
	scanner := bufio.NewScanner(strings.NewReader(string(src)))
	scanner.Buffer(nil, 1<<20)
	metadata := false
	for i := 1; scanner.Scan(); i++ {
		text := scanner.Text()
		if i == 1 && strings.HasPrefix(text, "#!") {
			continue
		}
		switch {
		case text == "# /// script":
			metadata = true
			continue
		case metadata:
			metadata = text != "# ///"
			continue
		}
		var quote byte
		for j := 0; j < len(text); j++ {
			c := text[j]
			switch {
			case quote != 0:
				if c == '\\' && quote == '"' {
					j++
				} else if c == quote {
					quote = 0
				}
			case c == '"' || c == '\'':
				quote = c
			case c == '#' && (j == 0 || text[j-1] == ' ' || text[j-1] == '\t'):
				if !strings.HasPrefix(text[j:], "# shellcheck") {
					n++
				}
				j = len(text)
			}
		}
	}
	return n
}

func htmlCommentLines(src []byte) int {
	n := 0
	for rest := string(src); ; {
		start := strings.Index(rest, "<!--")
		if start < 0 {
			return n
		}
		end := strings.Index(rest[start:], "-->")
		if end < 0 {
			end = len(rest) - start
		}
		n += strings.Count(rest[start:start+end], "\n") + 1
		rest = rest[start+end:]
	}
}

func readCommentBaseline(name string) (map[string]int, error) {
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		count, file, ok := strings.Cut(line, " ")
		n, err := strconv.Atoi(count)
		if !ok || err != nil {
			return nil, fmt.Errorf("%s: bad line %q", name, line)
		}
		counts[file] = n
	}
	return counts, nil
}

func writeCommentBaseline(name string, counts map[string]int) error {
	var b strings.Builder
	for _, file := range slices.Sorted(maps.Keys(counts)) {
		fmt.Fprintf(&b, "%d %s\n", counts[file], file)
	}
	return os.WriteFile(name, []byte(b.String()), 0o644)
}
