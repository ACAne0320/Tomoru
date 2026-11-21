package main

import (
	"time"
)

type Case struct {
	ID, Lang, Scene, Video string
	Start, End             float64
	Ref                    RefTrack
	Tgt                    []string
}

type RefTrack string

const RefTodo RefTrack = "todo"

func (c Case) HasRef() bool { return c.Ref != RefTodo }

type Event struct {
	T         float64 `json:"t"`
	Pending   *string `json:"pending,omitempty"`
	Draft     *string `json:"draft,omitempty"`
	Line      int     `json:"line,omitempty"`
	Src       string  `json:"src,omitempty"`
	Lang      string  `json:"lang,omitempty"`
	KeptDraft string  `json:"ldraft,omitempty"`
	Tgt       *string `json:"tgt,omitempty"`
	Done      bool    `json:"done,omitempty"`
	Err       string  `json:"err,omitempty"`
	Voice     *bool   `json:"voice,omitempty"`
}

func (e Event) OfSentence() bool { return e.Line != 0 }

type Result struct {
	ID        string     `json:"id"`
	Lang      string     `json:"lang"`
	Scene     string     `json:"scene"`
	Tgt       string     `json:"tgt"`
	TgtRef    string     `json:"tgtRef"`
	AudioSec  float64    `json:"audio"`
	Events    []Event    `json:"events"`
	Err       string     `json:"err,omitempty"`
	Sentences []Sentence `json:"sentences,omitempty"`
}

type Sentence struct {
	Src          string  `json:"src"`
	Tgt          string  `json:"tgt"`
	TranslateSec float64 `json:"sec"`
	Err          string  `json:"err,omitempty"`
}

type Run struct {
	Name        string    `json:"name"`
	ASR         string    `json:"asr"`
	MT          string    `json:"mt"`
	Auto        bool      `json:"auto,omitempty"`
	Draft       bool      `json:"draft"`
	MTOnly      bool      `json:"mtOnly,omitempty"`
	Glossary    string    `json:"glossary,omitempty"`
	About       string    `json:"about,omitempty"`
	Jobs        int       `json:"jobs"`
	Max         float64   `json:"max,omitempty"`
	Started     time.Time `json:"started"`
	WallSec     float64   `json:"wall"`
	AudioSec    float64   `json:"audio"`
	CPUChildSec float64   `json:"cpuChild"`
	CPUSelfSec  float64   `json:"cpuSelf"`
	Cases       []string  `json:"cases"`
}
