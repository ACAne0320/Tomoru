package core

import (
	"crypto/rand"
	"encoding/hex"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"tomoru/internal/asr"
	"tomoru/internal/learn"
	"tomoru/internal/scene"
	"tomoru/internal/translate"
)

// SceneService owns scene records, edits, undo operations, saved snapshots and the immutable
// active glossary. Core coordinates selection with preferences, history and live sessions.
type SceneService struct {
	Scenes      []scene.Scene
	savedScenes []scene.Scene
	activeID    string
	glossary    atomic.Pointer[scene.Glossary]
	about       atomic.Pointer[string] // the active scene's description
}

func (s *SceneService) load(path, selected string) (string, error) {
	s.Scenes = []scene.Scene{{}}
	var err error
	if path != "" {
		s.Scenes, err = scene.Load(path)
	}
	if s.sceneIndex(selected) < 0 {
		selected = ""
	}
	s.savedScenes = scene.Clone(s.Scenes)
	s.activate(selected)
	return selected, err
}

// SceneList returns detached records. Edits go through application actions.
func (s *SceneService) SceneList() []scene.Scene { return scene.Clone(s.Scenes) }

func (s *SceneService) sceneIndex(id string) int {
	return slices.IndexFunc(s.Scenes, func(sc scene.Scene) bool { return sc.ID == id })
}

func (s *SceneService) sceneByID(id string) (*scene.Scene, bool) {
	if i := s.sceneIndex(id); i >= 0 {
		return &s.Scenes[i], true
	}
	return nil, false
}

// SceneByID returns a detached draft; it never exposes a live glossary record.
func (s *SceneService) SceneByID(id string) (*scene.Scene, bool) {
	sc, ok := s.sceneByID(id)
	if !ok {
		return nil, false
	}
	return &scene.Clone([]scene.Scene{*sc})[0], true
}

func (s *SceneService) current(id string) *scene.Scene {
	if sc, ok := s.sceneByID(id); ok {
		return sc
	}
	return &s.Scenes[0]
}

func (s *SceneService) save(path string) error {
	if path == "" || reflect.DeepEqual(s.Scenes, s.savedScenes) {
		return nil
	}
	if err := scene.Save(path, s.Scenes); err != nil {
		return err
	}
	s.savedScenes = scene.Clone(s.Scenes)
	return nil
}

// selectScene remembers outgoing languages and restores the incoming scene's languages when
// valid. Otherwise it initializes that scene from the current translation pair.
func (s *SceneService) selectScene(from, to, src, tgt string) (string, string) {
	current, next := s.current(from), s.current(to)
	current.Src, current.Tgt = src, tgt
	if next.Src != "" && next.Tgt != "" && next.Src != next.Tgt {
		src, tgt = next.Src, next.Tgt
	} else {
		next.Src, next.Tgt = src, tgt
	}
	s.activate(to)
	return src, tgt
}

func (s *SceneService) setLanguages(id, src, tgt string) {
	sc := s.current(id)
	sc.Src, sc.Tgt = src, tgt
}

func (s *SceneService) addScene(name, src, tgt string) string {
	b := make([]byte, 4)
	for {
		rand.Read(b)
		id := hex.EncodeToString(b)
		if s.sceneIndex(id) >= 0 {
			continue
		}
		s.Scenes = append(s.Scenes, scene.Scene{ID: id, Name: strings.TrimSpace(name), Src: src, Tgt: tgt})
		return id
	}
}

// deleteScene captures a detached record so a restored scene never aliases an old draft.
func (s *SceneService) deleteScene(id string) (func() bool, bool) {
	i := s.sceneIndex(id)
	if i <= 0 {
		return func() bool { return false }, false
	}
	snapshot := scene.Clone(s.Scenes[i : i+1])[0]
	s.Scenes = slices.Delete(s.Scenes, i, i+1)
	if s.activeID == id {
		s.activate("")
	}
	return func() bool {
		if s.sceneIndex(id) >= 0 {
			return false
		}
		s.Scenes = slices.Insert(s.Scenes, min(i, len(s.Scenes)), scene.Clone([]scene.Scene{snapshot})[0])
		s.refresh()
		return true
	}, true
}

// updateScene validates identity, stores a detached edit and refreshes derived state before the
// application persists or notifies. A callback may retain its draft without owning live data.
func (s *SceneService) updateScene(id string, edit func(*scene.Scene)) (*scene.Scene, bool) {
	current, ok := s.sceneByID(id)
	if !ok {
		return nil, false
	}
	draft := scene.Clone([]scene.Scene{*current})[0]
	edit(&draft)
	draft.ID = id
	if reflect.DeepEqual(*current, draft) {
		return nil, false
	}
	*current = scene.Clone([]scene.Scene{draft})[0]
	s.refresh()
	return &draft, true
}

func (s *SceneService) addTerm(id string, t scene.Term) bool {
	sc, ok := s.sceneByID(id)
	if !ok {
		return false
	}
	t.Src, t.Tgt = strings.TrimSpace(t.Src), strings.TrimSpace(t.Tgt)
	if t.Src == "" || t.Tgt == "" {
		return false
	}
	var heard []string
	for _, h := range t.Heard {
		if h = strings.TrimSpace(h); h != "" && h != t.Src && !slices.Contains(heard, h) {
			heard = append(heard, h)
		}
	}
	t.Heard = heard
	if t.Added.IsZero() {
		t.Added = time.Now()
	}
	if i := slices.IndexFunc(sc.Terms, func(other scene.Term) bool { return other.Src == t.Src }); i >= 0 {
		for _, h := range sc.Terms[i].Heard {
			if !slices.Contains(t.Heard, h) {
				t.Heard = append(t.Heard, h)
			}
		}
		sc.Terms[i] = t
	} else {
		sc.Terms = append(sc.Terms, t)
	}
	s.refresh()
	return true
}

func (s *SceneService) deleteTerm(id string, i int) (func() bool, bool) {
	sc, ok := s.sceneByID(id)
	if !ok || i < 0 || i >= len(sc.Terms) {
		return func() bool { return false }, false
	}
	t := cloneTerms(sc.Terms[i : i+1])[0]
	sc.Terms = slices.Delete(sc.Terms, i, i+1)
	s.refresh()
	return func() bool {
		sc, ok := s.sceneByID(id)
		if !ok || slices.ContainsFunc(sc.Terms, func(other scene.Term) bool { return other.Src == t.Src }) {
			return false
		}
		sc.Terms = slices.Insert(sc.Terms, min(i, len(sc.Terms)), cloneTerms([]scene.Term{t})[0])
		s.refresh()
		return true
	}, true
}

func (s *SceneService) takeLearned(id string, suggestions []learn.Suggestion, taken []bool) (int, func() bool) {
	sc, ok := s.sceneByID(id)
	if !ok {
		return 0, func() bool { return false }
	}
	terms, rejected := cloneTerms(sc.Terms), slices.Clone(sc.Rejected)
	n := 0
	for i, sg := range suggestions {
		if i < len(taken) && taken[i] {
			if slices.ContainsFunc(sc.Terms, func(t scene.Term) bool { return !t.Auto && scene.Same(t.Src, sg.Src) }) {
				continue
			}
			if s.addTerm(id, scene.Term{Src: sg.Src, Tgt: sg.Tgt, Heard: sg.Heard, Auto: true}) {
				n++
			}
			continue
		}
		no := sg.Heard
		if len(no) == 0 {
			no = []string{sg.Src}
		}
		for _, h := range no {
			if !slices.ContainsFunc(sc.Rejected, func(r string) bool { return scene.Same(r, h) }) {
				sc.Rejected = append(sc.Rejected, h)
			}
		}
	}
	s.refresh()
	return n, func() bool {
		if sc, ok := s.sceneByID(id); ok {
			sc.Terms, sc.Rejected = cloneTerms(terms), slices.Clone(rejected)
			s.refresh()
			return true
		}
		return false
	}
}

func (s *SceneService) activate(id string) { s.activeID = id; s.refresh() }
func (s *SceneService) refresh() {
	sc := s.current(s.activeID)
	about := strings.TrimSpace(sc.About)
	s.glossary.Store(scene.NewGlossary(sc.Terms))
	s.about.Store(&about)
}

// sceneAbout is the active scene's description, which goes with each sentence translated. Like
// applyTerms, it is safe for session workers.
func (s *SceneService) sceneAbout() string {
	if about := s.about.Load(); about != nil {
		return *about
	}
	return ""
}

// hints are what recognition is told to expect, from the active scene: its names and description.
func (s *SceneService) hints() asr.Scene {
	return asr.Scene{Names: s.glossary.Load().Names(), About: s.sceneAbout()}
}

// applyTerms is safe for session workers: each edit publishes a complete immutable glossary.
func (s *SceneService) applyTerms(text string) (string, []translate.Term) {
	if glossary := s.glossary.Load(); glossary != nil {
		return glossary.Apply(text)
	}
	return text, nil
}

func cloneTerms(terms []scene.Term) []scene.Term {
	out := slices.Clone(terms)
	for i := range out {
		out[i].Heard = slices.Clone(out[i].Heard)
	}
	return out
}
