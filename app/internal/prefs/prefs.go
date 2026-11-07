// Package prefs is the user's settings: what they are, their defaults, and the file that keeps
// them.
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

// Fonts are the subtitle fonts, by key; the UI names them and knows their families.
var Fonts = []string{"system", "serif", "rounded"}

// MaxShown is the most sentences shown at once. Past what the screen holds, the oldest are cut
// off at its top, faded as they are.
const MaxShown = 10

// DefaultShortcut is the global shortcut at first: Option-Shift-T.
const DefaultShortcut = "Alt+Shift+T"

// DefaultImageShortcut is the shortcut that translates a picture at first: Option-Shift-S.
const DefaultImageShortcut = "Alt+Shift+S"

// Prefs are the settings.
type Prefs struct {
	Onboarded        bool
	GuideStep        int     // the first-run guide's page, to come back to: System Settings quits the app to grant it permission
	Src, Tgt         string  // language codes, of lang.All
	Source           string  // the app to hear, by bundle ID; "" for all the system's sound
	SourceName       string  // its name, to show while it is not running
	Display          int     // 0: source and translation, 1: translation only
	TgtFirst         bool    // the translation above the source
	Shown            float64 // sentences shown at once, MaxShown at most
	Style            int     // 0: on a dark box (closed captions), 1: immersive, the text alone
	Align            int     // 0: left, so that words come in at a fixed place, 1: centered
	Edge             int     // around the text: 0 none, 1 outline, 2 drop shadow
	EdgeColor        string
	Font             string // a key of Fonts
	SrcSize, TgtSize float64
	SrcColor         string
	TgtColor         string
	HideAfter        float64 // seconds without speech before the subtitles go
	KeepShown        bool    // the subtitles stay while nobody speaks, rather than going after HideAfter
	Locked           bool
	// The subtitle area within the overlay, which spans the work area's width: its center and
	// width, as fractions of that width.
	AreaX, AreaW    float64
	Bottom          int // the subtitle block's bottom edge on screen, 0 until moved: it grows upward
	HideFromCapture bool
	Draft           bool   // translate sentences while they are spoken
	AutoStart       bool   // start translating at launch: launching it is asking for subtitles
	Shortcut        string // the global shortcut that starts and stops translating, as an accelerator
	ImageShortcut   string // the global shortcut that translates the picture on the clipboard
	ImageTgt        string // the language pictures are translated into, as chosen on their card; "" for Tgt
	ImageW, ImageH  int    // the size of the picture's card, as the user resized it; 0 for its own
	// ASR is the recognition engine: "apple", Apple's speech recognition, on macOS; a recognition
	// model's ID ("sensevoice-small"), used once downloaded; a cloud service's ID ("qwen-cloud",
	// "openai"), used as set up in ASRAPIs; or "" where the system has none and the
	// user chose none yet.
	ASR     string
	ASRAPIs []Service // the cloud recognition services the user set up
	// ASRCloud is Qwen's service as settings kept it before ASRAPIs: read, it moves into them.
	ASRCloud *ASRCloud `json:",omitempty"`
	// MT is the translation engine: "cloud", a local model's ID ("local-2b"), a cloud service's
	// ID, used as set up in MTAPIs, or "custom:" and the ID of one of Endpoints.
	MT        string
	MTAPIs    []Service  // the cloud translation services the user set up
	Endpoints []Endpoint // the translation APIs the user added
	// ImageMT is the engine that translates pictures, named as MT is, one that reads them: the
	// cloud, a local model with its vision projector, or an endpoint not found to read none. ""
	// until core sets it, from MT where that reads pictures.
	ImageMT string
	// ModelsDir is the folder the local models were moved to; "" for where they are at first
	// (models.Dir).
	ModelsDir string
	UILang    string // the language the app shows, as chosen in it; "" for the system's
	// Appearance is light or dark as chosen in the app: "light", "dark", or "" for the system's.
	Appearance string
	Scene      string // the translation scene in use, by ID; "" for the general one
	History    bool   // keep the subtitles, on this computer only
	// NoProxy reaches the cloud directly; else through ProxyURL, or the system's proxy when it is
	// "".
	NoProxy  bool
	ProxyURL string // "host:port", or a URL of http, https or socks5
	Learn    string // the endpoint that learns names from them, by ID, as last chosen
}

// Endpoint is a translation API the user added: a model behind an OpenAI-compatible chat API. Its
// API key is in the system's credential store, not here.
type Endpoint struct {
	ID    string
	Name  string
	URL   string // the base, as https://api.openai.com, without /v1
	Model string
	// Format is how it is prompted, as translate's formats are named: "chat", "index" (as
	// Index-Translate is).
	Format string
	// Index is Format as settings kept it before, true for "index": read, it moves into Format.
	Index bool `json:",omitempty"`
	// Vision is whether it reads pictures, as its last test found.
	Vision Vision `json:",omitempty"`
}

// Vision is whether an endpoint reads pictures: not known until a test sends it one.
type Vision int

const (
	VisionUnknown Vision = iota
	VisionYes
	VisionNo
)

// Service is a cloud service the user set up, of recognition or of translation. Its API key is in
// the system's credential store, not here.
type Service struct {
	ID    string // the service, as asr.Specs or core names it: "qwen-cloud", "openai"
	URL   string // its address, "" for the service's
	Model string // "" for the service's default
	Key   bool   // an API key is kept: the service can be used
}

// ASRCloud is Qwen's service as settings kept it before ASRAPIs.
type ASRCloud struct {
	Host  string
	Model string
	Key   bool
}

// APIs are the cloud services the user set up, of both kinds.
func (p *Prefs) APIs() []Service { return slices.Concat(p.ASRAPIs, p.MTAPIs) }

// API finds a cloud service the user set up, of either kind, by ID.
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

// CustomPrefix starts MT for an endpoint.
const CustomPrefix = "custom:"

// Endpoint finds an endpoint by ID.
func (p *Prefs) Endpoint(id string) (*Endpoint, bool) {
	for i := range p.Endpoints {
		if p.Endpoints[i].ID == id {
			return &p.Endpoints[i], true
		}
	}
	return nil, false
}

// Clone is a copy of p that shares nothing with it.
func (p Prefs) Clone() Prefs {
	p.Endpoints = slices.Clone(p.Endpoints)
	p.ASRAPIs, p.MTAPIs = slices.Clone(p.ASRAPIs), slices.Clone(p.MTAPIs)
	p.ASRCloud = nil
	return p
}

// Default are the settings of a first launch for a reader of locale, the language the app shows:
// translating into it from English, or into English from Japanese for a reader of English, or of
// a language Tomoru does not translate into.
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
		ASR: DefaultASR(), MT: "cloud"}
	p.ResetStyle()
	return p
}

// SystemASR is the recognition engine the system has, "" for none.
func SystemASR() string {
	if runtime.GOOS == "darwin" {
		return "apple"
	}
	return ""
}

// DefaultASR is the recognition engine to use at first: the system's, else none, for the user to
// choose a model to download or the cloud's.
func DefaultASR() string { return SystemASR() }

// ResetStyle sets the subtitles' look back to the defaults: the text alone, outlined, in the "ice"
// scheme.
func (p *Prefs) ResetStyle() {
	p.Style, p.Edge, p.EdgeColor, p.TgtFirst = 1, 1, "#1e2259", false
	p.Font, p.SrcSize, p.TgtSize = Fonts[0], 20, 28
	p.SrcColor, p.TgtColor, p.HideAfter, p.KeepShown = "#ffffff", "#7fdbff", 5, false
}

// Load reads the settings at path over the defaults for locale. A missing file gives the
// defaults; values a person may have edited out of range are set right.
func Load(path, locale string) (Prefs, error) {
	p := Default(locale)
	var err error
	if b, rerr := os.ReadFile(path); rerr == nil {
		err = json.Unmarshal(b, &p)
	} else if !errors.Is(rerr, fs.ErrNotExist) {
		err = rerr
	}
	p.fix(Default(locale))
	return p, err
}

// Normalize validates an edited snapshot against its previous valid values.
func (p *Prefs) Normalize(d Prefs) { p.fix(d) }

func (p *Prefs) fix(d Prefs) {
	p.Shown = min(max(p.Shown, 1), MaxShown)
	if !lang.Known(p.Src) || !lang.Known(p.Tgt) || p.Src == p.Tgt {
		p.Src, p.Tgt = d.Src, d.Tgt
	}
	if !lang.Known(p.ImageTgt) || p.ImageTgt == p.Tgt {
		p.ImageTgt = ""
	}
	if !slices.Contains(Fonts, p.Font) {
		p.Font = Fonts[0]
	}
	if p.Source != "" && p.SourceName == "" {
		p.SourceName = p.Source
	}
	if p.MT == "" {
		p.MT = d.MT
	}
	if p.ASR == "" {
		p.ASR = d.ASR
	}
	if a := p.ASRCloud; a != nil {
		// Its key is kept under the same name: the service's ID.
		if _, ok := p.API("qwen-cloud"); !ok && (a.Key || a.Host != "" || a.Model != "") {
			p.ASRAPIs = append(p.ASRAPIs, Service{ID: "qwen-cloud", URL: a.Host, Model: a.Model, Key: a.Key})
		}
		p.ASRCloud = nil
	}
	for i := range p.Endpoints {
		e := &p.Endpoints[i]
		if e.Format == "" {
			e.Format = "chat"
			if e.Index {
				e.Format = "index"
			}
		}
		e.Index = false
	}
	if p.Shortcut == "" {
		p.Shortcut = d.Shortcut
	}
	p.ProxyURL = strings.TrimSpace(p.ProxyURL)
	if p.ImageShortcut == "" {
		p.ImageShortcut = d.ImageShortcut
	}
	if p.ImageShortcut == p.Shortcut { // starting and stopping took ⌥⇧S before pictures came
		p.ImageShortcut = DefaultShortcut
	}
}

// Save writes p to path.
func Save(path string, p Prefs) error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return atomicfile.Write(path, b, 0o644)
}
