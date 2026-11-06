package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The rules of docs/design-system.md that a test can keep.

// unstyled are the files that still draw with colors and sizes of their own, from before the design
// tokens: each leaves the list as its surface moves to kit's tokens, and none joins it.
var unstyled = []string{
	"internal/ui/overlay/handles.go",
	"internal/ui/overlay/overlay.go",
	"internal/ui/subtitle/subtitle.go",
}

// TestDesignTokens keeps colors and sizes in kit: the surfaces outside it name no color (ui.Hex
// and its kin with constant arguments) and give FontSize, Padding, Gap and Radius no number of
// their own but 0.
func TestDesignTokens(t *testing.T) {
	colors := []string{"Hex", "RGB", "RGBA", "Oklch", "HSL"}
	sizes := []string{"FontSize", "Padding", "Gap", "Radius"}
	found := map[string][]string{}
	err := filepath.WalkDir("internal/ui", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		path = filepath.ToSlash(path) // as the list has it, on Windows too
		if d.IsDir() && (path == "internal/ui/kit" || path == "internal/ui/assets") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			at := fset.Position(call.Pos())
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "ui" && slices.Contains(colors, sel.Sel.Name) && len(call.Args) > 0 && slices.IndexFunc(call.Args, func(a ast.Expr) bool { return !constant(a) }) < 0 {
				found[path] = append(found[path], fmt.Sprintf("%d: ui.%s", at.Line, sel.Sel.Name))
			}
			if slices.Contains(sizes, sel.Sel.Name) && slices.ContainsFunc(call.Args, number) {
				found[path] = append(found[path], fmt.Sprintf("%d: %s", at.Line, sel.Sel.Name))
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for path, at := range found {
		if !slices.Contains(unstyled, path) {
			t.Errorf("%s draws with its own colors or sizes; use kit's tokens: %s", path, strings.Join(at[:min(len(at), 5)], ", "))
		}
	}
	for _, path := range unstyled {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s is gone: take it off the list", path)
		} else if found[path] == nil {
			t.Errorf("%s draws with kit's tokens now: take it off the list", path)
		}
	}
}

// TestDesignScale keeps the scales of kit/tokens.go closed: no code in internal/ui, kit's included,
// makes a step of its own out of a token and a number (TextBody+1, RadiusCard+2, RadiusInner/2, a
// padding of Space(3)+2): a size the scale lacks is a token to name there. A token times a whole
// number, as both sides' padding, is a sum of steps; Space takes whole steps, as its type says.
func TestDesignScale(t *testing.T) {
	found := map[string][]string{}
	err := filepath.WalkDir("internal/ui", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		path = filepath.ToSlash(path)
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		// The binary expressions given a style as its size: there a Space with a number is a step
		// of its own too; elsewhere, Space(4)+16 is where something is.
		styled := map[ast.Expr]bool{}
		ast.Inspect(f, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && slices.Contains(styles, sel.Sel.Name) {
					for _, a := range call.Args {
						styled[a] = true
					}
				}
			}
			return true
		})
		ast.Inspect(f, func(n ast.Node) bool {
			b, ok := n.(*ast.BinaryExpr)
			if !ok {
				return true
			}
			for _, pair := range [][2]ast.Expr{{b.X, b.Y}, {b.Y, b.X}} {
				tok, lit := pair[0], pair[1]
				if !number(lit) || !step(tok) && !(styled[b] && space(tok)) {
					continue
				}
				switch b.Op {
				case token.ADD, token.SUB, token.QUO:
				case token.MUL:
					if l := lit.(*ast.BasicLit); l.Kind == token.INT {
						continue
					}
				default:
					continue
				}
				at := fset.Position(b.Pos())
				found[path] = append(found[path], fmt.Sprintf("%d: %s", at.Line, types(b)))
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for path, at := range found {
		t.Errorf("%s makes steps of its own out of kit's tokens; name the step in kit/tokens.go: %s", path, strings.Join(at[:min(len(at), 5)], ", "))
	}
}

// styles are the calls that give an element a size of the scales.
var styles = []string{"FontSize", "Padding", "Margin", "Gap", "GapX", "GapY", "Radius"}

// step tells whether e is a step of one of kit's named scales: TextBody, RadiusCard, IconSizeNav,
// WeightBold, CardGap, with kit. or without.
func step(e ast.Expr) bool {
	name := ""
	switch x := e.(type) {
	case *ast.Ident:
		name = x.Name
	case *ast.SelectorExpr:
		if pkg, ok := x.X.(*ast.Ident); ok && pkg.Name == "kit" {
			name = x.Sel.Name
		}
	}
	for _, prefix := range []string{"Text", "Radius", "IconSize", "Weight"} {
		if rest, ok := strings.CutPrefix(name, prefix); ok && rest != "" && rest[0] >= 'A' && rest[0] <= 'Z' {
			return true
		}
	}
	return name == "CardGap" || name == "SectionGap" || name == "GroupGap"
}

// space tells whether e is a call of Space, kit.Space or Space.
func space(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	switch f := call.Fun.(type) {
	case *ast.Ident:
		return f.Name == "Space"
	case *ast.SelectorExpr:
		return f.Sel.Name == "Space"
	}
	return false
}

// types is b as written, for the message.
func types(b *ast.BinaryExpr) string {
	var w strings.Builder
	printer.Fprint(&w, token.NewFileSet(), b)
	return w.String()
}

// constant tells whether e is a literal, signed or not.
func constant(e ast.Expr) bool {
	if u, ok := e.(*ast.UnaryExpr); ok {
		e = u.X
	}
	_, ok := e.(*ast.BasicLit)
	return ok
}

// number tells whether e is a number other than 0, written out.
func number(e ast.Expr) bool {
	if u, ok := e.(*ast.UnaryExpr); ok {
		e = u.X
	}
	lit, ok := e.(*ast.BasicLit)
	return ok && (lit.Kind == token.INT || lit.Kind == token.FLOAT) && strings.Trim(lit.Value, "0.") != ""
}

// TestDependencies keeps the modules the app requires itself to these: a new one is a decision,
// made by changing this list (docs/design-system.md, "Few dependencies"). go.mod is tidy: a module
// imported is required directly.
func TestDependencies(t *testing.T) {
	want := []string{
		"github.com/coder/websocket",   // cloud recognition's connection (internal/asr/qwencloud)
		"github.com/ebitengine/purego", // the overlay's Liquid Glass, from AppKit; MyGo requires it too
		"github.com/egoist/mygo",
		"golang.org/x/image", // scaling a picture down to translate it; MyGo requires it too
		"golang.org/x/sys",   // free disk space for the models; the memory and the taskbar's theme on Windows
		"golang.org/x/text",  // normalizing names to match them; MyGo requires it too
	}
	b, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	block := false
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case line == "require (":
			block = true
			continue
		case block && line == ")":
			block = false
			continue
		case strings.HasPrefix(line, "require "):
			line = strings.TrimPrefix(line, "require ")
		case !block:
			continue
		}
		if f := strings.Fields(line); len(f) >= 2 && !strings.Contains(line, "// indirect") {
			got = append(got, f[0])
		}
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("go.mod requires %q, want %q", got, want)
	}
}
