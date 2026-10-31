package core

import (
	"reflect"

	"tomoru/internal/prefs"
)

// PreferencesService owns settings snapshots and their persistence acknowledgement.
// Application code changes settings through Core.UpdatePrefs or a feature action.
type PreferencesService struct {
	Prefs prefs.Prefs
	saved prefs.Prefs
}

// Preferences returns a detached snapshot suitable for a view's editing draft.
func (s *PreferencesService) Preferences() prefs.Prefs { return s.Prefs.Clone() }

func (s *PreferencesService) save(path string) error {
	if path == "" || reflect.DeepEqual(s.Prefs, s.saved) {
		return nil
	}
	if err := prefs.Save(path, s.Prefs); err != nil {
		return err
	}
	s.saved = s.Prefs.Clone()
	return nil
}

// UpdatePrefs applies an explicit settings edit and its effects. UI bindings must
// use detached drafts, so a redraw alone never changes or saves application state.
func (c *Core) UpdatePrefs(edit func(*prefs.Prefs)) {
	before := c.Preferences()
	next := before.Clone()
	edit(&next)
	if reflect.DeepEqual(before, next) {
		return
	}
	next.Normalize(before)
	// Scene selection has its own language and history boundary.
	sceneID := next.Scene
	next.Scene = before.Scene
	c.Prefs = next.Clone()
	if c.fitLangs() {
		next.Src = c.Prefs.Src
	}
	if next.Src != before.Src || next.Tgt != before.Tgt {
		c.keepLangs()
	}
	if !next.History && before.History {
		c.closeHistory()
	}
	if next.Src != before.Src || next.Tgt != before.Tgt || next.Source != before.Source || next.Draft != before.Draft || next.ASR != before.ASR || next.MT != before.MT {
		c.RestartIfRunning()
	}
	if sceneID != before.Scene {
		c.SetScene(sceneID)
	}
	c.settingsChanged()
	if next.History && !before.History {
		c.HistoryService.backfill(c.State.Lines)
		c.HistoryService.drain()
	}
}

// ApplyPreferences merges only fields edited in a view draft. Actions executed
// while building that view cannot be overwritten by its older snapshot.
func (c *Core) ApplyPreferences(before, after prefs.Prefs) {
	if reflect.DeepEqual(before, after) {
		return
	}
	c.UpdatePrefs(func(p *prefs.Prefs) {
		old, changed, dst := reflect.ValueOf(before), reflect.ValueOf(after), reflect.ValueOf(p).Elem()
		for i := 0; i < old.NumField(); i++ {
			if !reflect.DeepEqual(old.Field(i).Interface(), changed.Field(i).Interface()) {
				dst.Field(i).Set(changed.Field(i))
			}
		}
	})
}
