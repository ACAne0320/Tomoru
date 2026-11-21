package prefs

import (
	"encoding/json"
	"errors"
	"io/fs"

	"os"
	"runtime"
	"slices"
	"strings"
	"tomoru/internal/atomicfile"
	"tomoru/internal/lang"
)

type Font string

const (
	FontSystem  Font = "system"
	FontSerif   Font = "serif"
	FontRounded Font = "rounded"
)

var Fonts = []Font{FontSystem, FontSerif, FontRounded}

type Appearance string

const (
	AppearanceSystem Appearance = ""
	AppearanceLight  Appearance = "light"
	AppearanceDark   Appearance = "dark"
)

var Appearances = []Appearance{AppearanceSystem, AppearanceLight, AppearanceDark}

func (f Font) Builtin() bool { return slices.Contains(Fonts, f) }

const MaxShown = 10

const (
	MinTextOpacity  = 30
	MinOlderOpacity = 20
)

const DefaultShortcut = "Alt+Shift+T"

const DefaultImageShortcut = "Alt+Shift+S"

type Display int

const (
	DisplayBoth Display = iota
	DisplayTgt
	DisplaySrc
)

type Style int

const (
	StyleBox Style = iota
	StyleImmersive
)

type Align int

const (
	AlignLeft Align = iota
	AlignCenter
)

type Edge int

const (
	EdgeNone Edge = iota
	EdgeOutline
	EdgeShadow
)

const ScreenPrimary = ""

type Prefs struct {
	Onboarded        bool
	GuideStep        int
	Src, Tgt         string
	Source           string
	SourceName       string
	Display          Display
	TgtFirst         bool
	Shown            float64
	Style            Style
	Align            Align
	Edge             Edge
	EdgeColor        string
	Font             Font
	SrcSize, TgtSize float64
	SrcColor         string
	TgtColor         string
	TextOpacity      float64
	BoxOpacity       float64
	OlderOpacity     float64
	HideAfterSeconds float64 `json:"HideAfter"`
	KeepShown        bool
	Locked           bool
	AreaX, AreaW     float64
	Bottom           int
	Screen           string
	HideFromCapture  bool
	Draft            bool
	AutoStart        bool
	Shortcut         string
	ImageShortcut    string
	ImageTgt         string
	ImageW, ImageH   int
	ASR              string
	ASRAPIs          []Service
	MT               string
	MTAPIs           []Service
	Endpoints        []Endpoint
	ImageMT          string
	ModelsDir        string
	UILang           string
	Appearance       Appearance
	Scene            string
	History          bool
	NoProxy          bool
	ProxyURL         string
	Learn            string
}

type Endpoint struct {
	ID     string
	Name   string
	URL    string
	Model  string
	Format string
	Vision Vision `json:",omitempty"`
}

type Vision int

const (
	VisionUnknown Vision = iota
	VisionYes
	VisionNo
)

type Service struct {
	ID    string
	URL   string
	Model string
	Key   bool
}

type legacy struct {
	ASRCloud *struct {
		Host  string
		Model string
		Key   bool
	}
	Endpoints []struct{ Index bool }
}

func (p *Prefs) migrate(old legacy) {
	if a := old.ASRCloud; a != nil {
		if _, ok := p.API("qwen-cloud"); !ok && (a.Key || a.Host != "" || a.Model != "") {
			p.ASRAPIs = append(p.ASRAPIs, Service{ID: "qwen-cloud", URL: a.Host, Model: a.Model, Key: a.Key})
		}
	}
	for i := range p.Endpoints {
		e := &p.Endpoints[i]
		if e.Format != "" {
			continue
		}
		e.Format = "chat"
		if i < len(old.Endpoints) && old.Endpoints[i].Index {
			e.Format = "index"
		}
	}
}

func (p Prefs) HearsAll() bool { return p.Source == "" }

func (p Prefs) Translates() bool { return p.Display != DisplaySrc }

func (p *Prefs) APIs() []Service { return slices.Concat(p.ASRAPIs, p.MTAPIs) }

func (p *Prefs) API(id string) (*Service, bool) {
	for _, apis := range [][]Service{p.ASRAPIs, p.MTAPIs} {
		for i := range apis {
			if apis[i].ID == id {
				return &apis[i], true
			}
		}
	}
	return nil, false
}

const endpointPrefix = "custom:"

func EndpointEngine(id string) string { return endpointPrefix + id }

func EndpointOf(engine string) (id string, ok bool) { return strings.CutPrefix(engine, endpointPrefix) }

func IsEndpoint(engine string) bool { return strings.HasPrefix(engine, endpointPrefix) }

func (p *Prefs) Endpoint(id string) (*Endpoint, bool) {
	for i := range p.Endpoints {
		if p.Endpoints[i].ID == id {
			return &p.Endpoints[i], true
		}
	}
	return nil, false
}

func (p Prefs) Clone() Prefs {
	p.Endpoints = slices.Clone(p.Endpoints)
	p.ASRAPIs, p.MTAPIs = slices.Clone(p.ASRAPIs), slices.Clone(p.MTAPIs)
	return p
}

func Default(locale string) Prefs {
	tgt := "en"
	if l := lang.Of(locale); l != "" {
		tgt = l
	}
	src := "en"
	if tgt == "en" {
		src = "ja"
	}
	p := Prefs{Src: src, Tgt: tgt, Shown: 2, AreaX: .5, AreaW: .62, Draft: true, AutoStart: true, Shortcut: DefaultShortcut, ImageShortcut: DefaultImageShortcut,
		ASR: SystemASR(), MT: "cloud"}
	p.ResetStyle()
	return p
}

func SystemASR() string {
	if runtime.GOOS == "darwin" {
		return "apple"
	}
	return ""
}

func (p *Prefs) SetStyle(s Style) {
	p.Style = s
	if s == StyleImmersive && p.Edge == EdgeNone {
		p.Edge = EdgeOutline
	}
}

func (p *Prefs) ResetStyle() {
	p.Style, p.Edge, p.EdgeColor, p.TgtFirst = StyleImmersive, EdgeOutline, "#1e2259", false
	p.Font, p.SrcSize, p.TgtSize = FontSystem, 20, 28
	p.SrcColor, p.TgtColor, p.HideAfterSeconds, p.KeepShown = "#ffffff", "#7fdbff", 5, false
	p.TextOpacity, p.BoxOpacity, p.OlderOpacity = 100, 62, 54
}

func Load(path, locale string) (Prefs, error) {
	p := Default(locale)
	var err error
	if b, rerr := os.ReadFile(path); rerr == nil {
		var old legacy
		if err = json.Unmarshal(b, &p); err == nil {
			json.Unmarshal(b, &old)
			p.migrate(old)
		}
	} else if !errors.Is(rerr, fs.ErrNotExist) {
		err = rerr
	}
	p.fix(Default(locale))
	return p, err
}

func (p *Prefs) Normalize(d Prefs) { p.fix(d) }

func (p *Prefs) fix(d Prefs) {
	p.Shown = min(max(p.Shown, 1), MaxShown)
	p.TextOpacity = min(max(p.TextOpacity, MinTextOpacity), 100)
	p.BoxOpacity = min(max(p.BoxOpacity, 0), 100)
	p.OlderOpacity = min(max(p.OlderOpacity, MinOlderOpacity), 100)
	if p.Display < DisplayBoth || p.Display > DisplaySrc {
		p.Display = d.Display
	}
	if !lang.Known(p.Src) && p.Src != lang.Auto || !lang.Known(p.Tgt) || p.Src == p.Tgt {
		p.Src, p.Tgt = d.Src, d.Tgt
	}
	if !lang.Known(p.ImageTgt) || p.ImageTgt == p.Tgt {
		p.ImageTgt = ""
	}
	if strings.TrimSpace(string(p.Font)) == "" {
		p.Font = FontSystem
	}
	for _, c := range []struct {
		v *string
		d string
	}{{&p.SrcColor, d.SrcColor}, {&p.TgtColor, d.TgtColor}, {&p.EdgeColor, d.EdgeColor}} {
		if !isColor(*c.v) {
			*c.v = c.d
		}
	}
	if !p.HearsAll() && p.SourceName == "" {
		p.SourceName = p.Source
	}
	if p.MT == "" {
		p.MT = d.MT
	}
	if p.ASR == "" {
		p.ASR = d.ASR
	}
	if p.Shortcut == "" {
		p.Shortcut = d.Shortcut
	}
	p.ProxyURL = strings.TrimSpace(p.ProxyURL)
	if p.ImageShortcut == "" {
		p.ImageShortcut = d.ImageShortcut
	}
	if p.ImageShortcut == p.Shortcut {
		p.ImageShortcut = DefaultShortcut
	}
}

func isColor(s string) bool {
	h := strings.TrimPrefix(strings.TrimSpace(s), "#")
	if n := len(h); n != 3 && n != 4 && n != 6 && n != 8 {
		return false
	}
	return strings.Trim(h, "0123456789abcdefABCDEF") == ""
}

func Save(path string, p Prefs) error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.Write(path, b, 0o644)
}
