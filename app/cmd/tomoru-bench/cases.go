package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"tomoru/internal/scene"
)

func readCases(path string) ([]Case, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []Case
	header := true
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "#") || strings.TrimSpace(line) == "" {
			continue
		}
		if header {
			header = false
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) < 8 {
			return nil, fmt.Errorf("%s: a short line: %q", path, line)
		}
		c := Case{ID: f[0], Lang: f[1], Scene: f[2], Video: f[3], Start: seconds(f[4]), End: seconds(f[5]), Ref: f[6]}
		for _, t := range strings.Split(f[7], ",") {
			if t != "" && t != "-" {
				c.Tgt = append(c.Tgt, t)
			}
		}
		out = append(out, c)
	}
	return out, nil
}

func seconds(s string) float64 {
	var t float64
	for _, p := range strings.Split(s, ":") {
		v, _ := strconv.ParseFloat(p, 64)
		t = t*60 + v
	}
	return t
}

func round(x float64) float64 { return float64(int64(x*1000+.5)) / 1000 }

// readGlossary reads a case's glossary: none if it has no file.
func readGlossary(path string) (*scene.Glossary, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	var terms []scene.Term
	if err == nil {
		err = json.Unmarshal(b, &terms)
	}
	if err != nil {
		return nil, fmt.Errorf("glossary: %w", err)
	}
	return scene.NewGlossary(terms), nil
}

// readAbout reads the description of case id's scene, <id>.txt in dir: "" when there is none.
func readAbout(dir, id string) (string, error) {
	if dir == "" {
		return "", nil
	}
	b, err := os.ReadFile(filepath.Join(dir, id+".txt"))
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	return strings.TrimSpace(string(b)), err
}
