package settings

import (
	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/kit"
	"tomoru/locales"
)

// confirmEdit is a deletion asked about: the question, what goes with it, and the deletion.
type confirmEdit struct {
	open             bool
	question, detail string
	do               func(c *ui.Context)
}

// ask asks before do deletes something; do runs once Delete is chosen.
func (s *Settings) ask(question, detail string, do func(c *ui.Context)) {
	s.confirm = confirmEdit{open: true, question: question, detail: detail, do: do}
}

// confirmDialog is the dialog that asks.
func (s *Settings) confirmDialog(c *ui.Context) {
	d := &s.confirm
	if d.open && kit.Confirm(c, &d.open, d.question, d.detail, locales.Delete()) {
		d.do(c)
	}
}
