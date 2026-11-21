package settings

import (
	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/kit"
	"tomoru/locales"
)

type confirmEdit struct {
	open             bool
	question, detail string
	do               func(c *ui.Context)
}

func (s *Settings) ask(question, detail string, do func(c *ui.Context)) {
	s.confirm = confirmEdit{open: true, question: question, detail: detail, do: do}
}

func (s *Settings) confirmDialog(c *ui.Context) {
	d := &s.confirm
	if d.open && kit.Confirm(c, &d.open, d.question, d.detail, locales.Delete()) {
		d.do(c)
	}
}
