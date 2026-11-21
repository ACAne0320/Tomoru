package main

import (
	"time"
)

type Case struct {
	ID, Lang, Scene, Video string
	Start, End             float64
	Ref                    string
	Tgt                    []string
}

func (c Case) HasRef() bool { return c.Ref != "todo" }

type Event struct {
	T       float64 `json:"t"`
	Pending *string `json:"pending,omitempty"`
	Draft   *string `json:"draft,omitempty"`
	Line    int     `json:"line,omitempty"`
	Src     string  `json:"src,omitempty"`
	Lang    string  `json:"lang,omitempty"`
	LDraft  string  `json:"ldraft,omitempty"`
	Tgt     *string `json:"tgt,omitempty"`
	Done    bool    `json:"done,omitempty"`
	Err     string  `json:"err,omitempty"`
	Voice   *bool   `json:"voice,omitempty"`
}

type Result struct {
	ID        string     `json:"id"`
	Lang      string     `json:"lang"`
	Scene     string     `json:"scene"`
	Tgt       string     `json:"tgt"`
	TgtRef    string     `json:"tgtRef"`
	Audio     float64    `json:"audio"`
	Events    []Event    `json:"events"`
	Err       string     `json:"err,omitempty"`
	Sentences []Sentence `json:"sentences,omitempty"`
}

type Sentence struct {
	Src string  `json:"src"`
	Tgt string  `json:"tgt"`
	Sec float64 `json:"sec"`
	Err string  `json:"err,omitempty"`
}

type Run struct {
	Name     string    `json:"name"`
	ASR      string    `json:"asr"`
	MT       string    `json:"mt"`
	Auto     bool      `json:"auto,omitempty"`
	Draft    bool      `json:"draft"`
	MTOnly   bool      `json:"mtOnly,omitempty"`
	Glossary string    `json:"glossary,omitempty"`
	About    string    `json:"about,omitempty"`
	Jobs     int       `json:"jobs"`
	Max      float64   `json:"max,omitempty"`
	Started  time.Time `json:"started"`
	Wall     float64   `json:"wall"`
	Audio    float64   `json:"audio"`
	CPUChild float64   `json:"cpuChild"`
	CPUSelf  float64   `json:"cpuSelf"`
	Cases    []string  `json:"cases"`
}
