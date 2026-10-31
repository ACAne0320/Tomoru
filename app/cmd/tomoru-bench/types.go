package main

import (
	"time"
)

// Case is a line of cases.tsv.
type Case struct {
	ID, Lang, Scene, Video string
	Start, End             float64
	Ref                    string   // the reference's subtitle track, "todo" or "manual"
	Tgt                    []string // the translations' tracks, as "zh-CN"
}

// HasRef tells whether the case has a reference to score recognition against.
func (c Case) HasRef() bool { return c.Ref != "todo" }

// Event is a change of the subtitles, at T seconds of the recording.
type Event struct {
	T       float64 `json:"t"`
	Pending *string `json:"pending,omitempty"` // the text heard and not committed yet
	Draft   *string `json:"draft,omitempty"`   // the draft translation of the pending text
	Line    int     `json:"line,omitempty"`    // a line's ID, new or changed
	Src     string  `json:"src,omitempty"`     // a new line's text
	LDraft  string  `json:"ldraft,omitempty"`  // the draft it keeps until its translation is done
	Tgt     *string `json:"tgt,omitempty"`     // its translation so far
	Done    bool    `json:"done,omitempty"`    // its translation is complete
	Err     string  `json:"err,omitempty"`     // why it has none
	Voice   *bool   `json:"voice,omitempty"`   // someone speaks, by voice detection
}

// Result is a case's run.
type Result struct {
	ID     string  `json:"id"`
	Lang   string  `json:"lang"`
	Scene  string  `json:"scene"`
	Tgt    string  `json:"tgt"`    // the language translated into
	TgtRef string  `json:"tgtRef"` // its reference's track, "" for none
	Audio  float64 `json:"audio"`  // seconds of the recording
	Events []Event `json:"events"`
	Err    string  `json:"err,omitempty"`
	// MT-only runs: the references' sentences and their translations.
	Sentences []Sentence `json:"sentences,omitempty"`
}

// Sentence is a sentence of a reference, translated.
type Sentence struct {
	Src string  `json:"src"`
	Tgt string  `json:"tgt"`
	Sec float64 `json:"sec"` // how long its translation took
	Err string  `json:"err,omitempty"`
}

// Run is a run's configuration and what it cost.
type Run struct {
	Name     string    `json:"name"`
	ASR      string    `json:"asr"`
	MT       string    `json:"mt"`
	Draft    bool      `json:"draft"`
	MTOnly   bool      `json:"mtOnly,omitempty"`
	Glossary string    `json:"glossary,omitempty"`
	About    string    `json:"about,omitempty"`
	Jobs     int       `json:"jobs"`
	Max      float64   `json:"max,omitempty"`
	Started  time.Time `json:"started"`
	Wall     float64   `json:"wall"`     // seconds the run took
	Audio    float64   `json:"audio"`    // seconds of audio it heard
	CPUChild float64   `json:"cpuChild"` // CPU seconds of the engines, tomoru-asr and llama-server
	CPUSelf  float64   `json:"cpuSelf"`  // of this process: segmenting, the cloud's sessions
	Cases    []string  `json:"cases"`
}
