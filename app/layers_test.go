package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// Every internal package family is classified here. New families fail closed, while
// subpackages inherit the same restrictions. UI surfaces may share kit/shared/subtitle,
// but only shell may import another surface. Test adapters never enter production.
var packageDependencies = map[string][]string{
	"applog": {}, "capture": {"resource"}, "resource": {}, "samples": {}, "models": {"resource"},
	"prefs": {"atomicfile", "lang"}, "proxy": {}, "history": {}, "usage": {}, "atomicfile": {}, "lang": {},
	"translate": {"lang"}, "segment": {"lang"}, "scene": {"translate", "atomicfile"},
	"engine": {"resource"}, "asr": {"engine", "lang", "models"}, "learn": {"lang", "scene"},
	"live":          {"asr", "engine", "lang", "segment", "translate", "usage"},
	"provider":      {"translate"},
	"core":          {"asr", "engine", "history", "lang", "learn", "live", "models", "prefs", "provider", "samples", "scene", "translate", "usage"},
	"core/coretest": {"core", "engine", "live", "models", "prefs"},
	"ui/assets":     {},
	"ui/kit":        {"ui/assets"},
	"ui/shared":     {"core", "asr", "engine", "lang", "models", "prefs", "scene", "ui/assets", "ui/kit"},
	"ui/subtitle":   {"lang", "live", "prefs", "samples", "ui/kit"},
	"ui/guide":      {"core", "engine", "live", "models", "prefs", "ui/assets", "ui/kit", "ui/shared", "ui/subtitle"},
	"ui/imagecard":  {"core", "translate", "ui/assets", "ui/kit", "ui/shared"},
	"ui/overlay":    {"core", "prefs", "ui/assets", "ui/kit", "ui/shared", "ui/subtitle"},
	"ui/panel":      {"core", "prefs", "ui/assets", "ui/kit", "ui/shared"},
	"ui/settings":   {"asr", "core", "history", "models", "prefs", "proxy", "scene", "translate", "usage", "ui/assets", "ui/kit", "ui/shared", "ui/subtitle"},
	"ui/shell":      {"core", "prefs", "ui/assets", "ui/guide", "ui/imagecard", "ui/kit", "ui/overlay", "ui/panel", "ui/settings", "ui/shared"},
}

// Test fixtures may construct the data consumed by a package, or use the desktop test
// adapter. These permissions apply only to _test.go files, never production imports.
var testDependencies = map[string][]string{
	"segment":      {"engine"},
	"live":         {"models", "resource"},
	"ui/guide":     {"core/coretest", "samples"},
	"ui/imagecard": {"core/coretest"},
	"ui/overlay":   {"core/coretest", "engine", "live"},
	"ui/panel":     {"core/coretest", "live"},
	"ui/settings":  {"core/coretest", "learn", "live"},
	"ui/shell":     {"core/coretest"},
}

func inPackage(path, prefix string) bool {
	return path == prefix || strings.HasPrefix(path, prefix+"/")
}

func packageFamily(path string) string {
	name, internal := strings.CutPrefix(path, "tomoru/internal/")
	if !internal {
		return ""
	}
	longest := ""
	for family := range packageDependencies {
		if inPackage(name, family) && len(family) > len(longest) {
			longest = family
		}
	}
	return longest
}

func mayImport(pkg, imp string, test bool) bool {
	family := packageFamily(pkg)
	if family == "" {
		return false
	}
	ui := strings.HasPrefix(family, "ui/")
	desktop := ui || family == "core/coretest"
	// MyGo's netproxy is a library, not the desktop: proxy reads the system's settings with it.
	if imp == "github.com/egoist/mygo/netproxy" && family == "proxy" {
		return true
	}
	if inPackage(imp, "github.com/egoist/mygo") || inPackage(imp, "tomoru/locales") {
		return desktop
	}
	if !inPackage(imp, "tomoru") {
		return true
	}
	dependency := packageFamily(imp)
	if dependency == "" {
		return false
	}
	if dependency == "core/coretest" && !test {
		return false
	}
	return dependency == family || slices.Contains(packageDependencies[family], dependency) ||
		test && slices.Contains(testDependencies[family], dependency)
}

func TestLayers(t *testing.T) {
	// Parse all platforms, not just packages chosen by this machine's build tags.
	err := filepath.WalkDir("internal", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		pkg := "tomoru/" + filepath.ToSlash(filepath.Dir(path))
		if packageFamily(pkg) == "" {
			t.Errorf("unclassified package %s: add its dependency rules", pkg)
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			imp, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			if !mayImport(pkg, imp, strings.HasSuffix(path, "_test.go")) {
				t.Errorf("%s imports forbidden dependency %s", path, imp)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestLayerRules(t *testing.T) {
	for _, tc := range []struct {
		pkg, imp   string
		test, want bool
	}{
		{"ui/kit", "core", false, false},
		{"ui/kit/nested", "scene", true, false},
		{"ui/shared", "core", false, true},
		{"ui/shared", "ui/imagecard", false, false},
		{"ui/panel", "ui/imagecard", false, false},
		{"ui/guide", "ui/settings/nested", false, false},
		{"ui/shell", "ui/imagecard/nested", false, true},
		{"ui/new-surface", "ui/kit", false, false},
		{"ui/settings", "unclassified", false, false},
		{"ui/settings", "core/coretest", false, false},
		{"ui/settings", "core/coretest", true, true},
		{"core", "ui/kit", false, false},
		{"engine", "live", false, false},
		{"live", "provider", false, false},
		{"provider", "engine", false, false},
		{"provider", "translate", false, true},
		{"asr", "engine", false, true},
		{"asr/qwencloud", "lang", false, true},
		{"asr", "live", false, false},
		{"live", "asr", false, true},
		{"resource", "core", false, false},
	} {
		if got := mayImport("tomoru/internal/"+tc.pkg, "tomoru/internal/"+tc.imp, tc.test); got != tc.want {
			t.Errorf("%s -> %s (test=%v): got %v, want %v", tc.pkg, tc.imp, tc.test, got, tc.want)
		}
	}
	for _, family := range []string{"engine", "asr", "live", "resource", "provider", "core"} {
		for _, framework := range []string{"github.com/egoist/mygo", "github.com/egoist/mygo/ui", "github.com/egoist/mygo/ui/shader", "tomoru/locales"} {
			if mayImport("tomoru/internal/"+family, framework, false) {
				t.Errorf("%s may import desktop dependency %s", family, framework)
			}
		}
	}
	if !mayImport("tomoru/internal/core/coretest", "github.com/egoist/mygo", false) {
		t.Error("desktop test adapter must be allowed to schedule UI updates")
	}
}

// UI bindings use detached snapshots. Calling a save from a view or reaching through
// the facade's compatibility fields would reintroduce redraw-dependent persistence.
func TestUIStateAccess(t *testing.T) {
	err := filepath.WalkDir("internal/ui", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			field, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if field.Sel.Name == "SavePrefs" {
				t.Errorf("%s saves while rendering; use application actions", path)
			}
			if field.Sel.Name == "Prefs" || field.Sel.Name == "Scenes" {
				if name, ok := field.X.(*ast.Ident); ok && name.Name == "prefs" {
					return true
				}
				t.Errorf("%s accesses mutable %s; use a detached snapshot and application actions", path, field.Sel.Name)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
