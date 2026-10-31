package core

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"tomoru/internal/learn"
	"tomoru/internal/scene"
)

func initializedScenes(t *testing.T) *SceneService {
	t.Helper()
	s := &SceneService{}
	if _, err := s.load("", ""); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSceneServiceDetachedEditsAndImmediateGlossary(t *testing.T) {
	s := initializedScenes(t)
	id := s.addScene("game", "ja", "zh")
	s.activate(id)
	input := scene.Term{Src: "クラウド", Tgt: "克劳德", Heard: []string{"クラウト"}}
	s.addTerm(id, input)
	input.Heard[0] = "changed outside service"
	draft, _ := s.SceneByID(id)
	draft.Terms[0].Tgt = "detached mutation"
	draft.Terms[0].Heard[0] = "detached mutation"
	list := s.SceneList()
	list[1].Terms[0].Tgt = "list mutation"
	if src, terms := s.applyTerms("クラウト"); src != "クラウド" || len(terms) != 1 || terms[0].Tgt != "克劳德" {
		t.Fatalf("live glossary aliased a draft: %q %+v", src, terms)
	}
	var retained *scene.Scene
	s.updateScene(id, func(edit *scene.Scene) {
		retained = edit
		edit.ID = "not allowed"
		edit.Terms[0].Tgt = "new translation"
	})
	retained.Terms[0].Tgt = "after callback"
	stored, _ := s.SceneByID(id)
	if stored.ID != id || stored.Terms[0].Tgt != "new translation" {
		t.Errorf("stored edit aliases retained draft: %+v", stored)
	}
	if _, terms := s.applyTerms("クラウト"); terms[0].Tgt != "new translation" {
		t.Errorf("edit did not immediately publish glossary: %+v", terms)
	}
}

func TestSceneServiceFailedSaveCanRetryUnchangedData(t *testing.T) {
	s := initializedScenes(t)
	id := s.addScene("game", "ja", "zh")
	s.addTerm(id, scene.Term{Src: "name", Tgt: "translation", Heard: []string{"heard"}})
	path := filepath.Join(t.TempDir(), "scenes.json")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	before := scene.Clone(s.savedScenes)
	if err := s.save(path); err == nil {
		t.Fatal("expected rename onto directory to fail")
	}
	if !reflect.DeepEqual(s.savedScenes, before) {
		t.Fatal("failed save was marked persisted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := s.save(path); err != nil {
		t.Fatalf("unchanged dirty data was not retried: %v", err)
	}
	loaded, err := scene.Load(path)
	stored, _ := json.Marshal(loaded)
	expected, _ := json.Marshal(s.SceneList())
	if err != nil || !bytes.Equal(stored, expected) {
		t.Fatalf("retry did not save current records: %+v %v", loaded, err)
	}
	s.Scenes[1].Terms[0].Heard[0] = "later edit"
	if s.savedScenes[1].Terms[0].Heard[0] != "heard" {
		t.Error("saved snapshot shares nested slices with mutable state")
	}
}

func TestSceneServiceUndoDoesNotOverwriteNewTerm(t *testing.T) {
	s := initializedScenes(t)
	s.addTerm("", scene.Term{Src: "name", Tgt: "old"})
	undo, ok := s.deleteTerm("", 0)
	if !ok {
		t.Fatal("term not deleted")
	}
	s.addTerm("", scene.Term{Src: "name", Tgt: "new"})
	if undo() {
		t.Fatal("undo replaced a term the user recreated")
	}
	if _, terms := s.applyTerms("name"); len(terms) != 1 || terms[0].Tgt != "new" {
		t.Errorf("undo overwrote explicit edit: %+v", terms)
	}
}

func TestSceneServiceLearnedTermsAndUndo(t *testing.T) {
	s := initializedScenes(t)
	s.addTerm("", scene.Term{Src: "manual", Tgt: "keep"})
	n, undo := s.takeLearned("", []learn.Suggestion{{Src: "manual", Tgt: "overwrite"}, {Src: "learned", Tgt: "accepted", Heard: []string{"heard name"}}, {Src: "rejected", Tgt: "no"}}, []bool{true, true, false})
	sc, _ := s.SceneByID("")
	if n != 1 || len(sc.Terms) != 2 || sc.Terms[0].Tgt != "keep" || len(sc.Rejected) != 1 || sc.Rejected[0] != "rejected" {
		t.Fatalf("learned changes: %d %+v", n, sc)
	}
	if !undo() {
		t.Fatal("undo failed")
	}
	sc, _ = s.SceneByID("")
	if len(sc.Terms) != 1 || sc.Terms[0].Tgt != "keep" || len(sc.Rejected) != 0 {
		t.Fatalf("undo did not restore glossary/rejections: %+v", sc)
	}
}

func TestSceneServiceLoadPropagatesReadErrors(t *testing.T) {
	s := &SceneService{}
	selected, err := s.load(t.TempDir(), "gone")
	if err == nil {
		t.Fatal("directory read error hidden as missing settings")
	}
	if selected != "" || len(s.Scenes) != 1 {
		t.Fatalf("invalid fallback scene: %q %+v", selected, s.Scenes)
	}
	if text, terms := s.applyTerms("original"); text != "original" || len(terms) != 0 {
		t.Errorf("fallback glossary: %q %+v", text, terms)
	}
}
