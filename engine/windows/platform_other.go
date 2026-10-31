//go:build !windows

package main

import (
	"errors"
	"image"
)

// Elsewhere than Windows, the engine hears files only: its tests run on any computer.

var errWindowsOnly = errors.New("only on Windows")

type unsupported struct{}

func (unsupported) Start(func([]int16)) error { return errWindowsOnly }

func systemSource(string) Source { return unsupported{} }

func listApps() {}

func speak(locale string, _ []string) {
	fail("no-voice", "no voice for %s: %v", locale, errWindowsOnly)
}

func selectArea(string) (*image.Rectangle, error) { return nil, errWindowsOnly }
