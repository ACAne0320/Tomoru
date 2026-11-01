package guide

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"tomoru/internal/ui/assets"
	"tomoru/internal/ui/kit"
	"tomoru/locales"
)

// permission asks for System Audio Recording, and shows the answer as soon as there is one. Screen
// Recording is not asked here: macOS has it turned on in System Settings and the app reopened,
// which would break the guide off; selecting a part of the screen is for the settings
// (docs/image-translation.md).
func (g *Guide) permission(c *ui.Context) {
	k := g.core
	granted := k.Access == "granted"
	// Answered, the system's prompt gives the front back to the app before. Tomoru starts as a
	// regular app for the guide so that it is that app (main.go); should another be, take it back.
	if k.Asking {
		g.asked = true
	} else if g.asked {
		g.asked = false
		g.nav.Focus()
	}
	// Check now when the guide opens at this step, and again every so often: the user may answer
	// the system's prompt after the request gave up waiting. (Turned on in System Settings, it
	// takes effect when the app is reopened.)
	if !k.Asking && !granted {
		if wait := 1500*time.Millisecond - time.Since(k.CheckedAt); wait > 0 {
			c.After(wait)
		} else {
			k.CheckAccess()
		}
	}
	g.part(c, 0, func() { title(c, locales.GuidePermissionTitle()) })
	g.part(c, 1, func() {
		ui.Column(c).Gap(kit.Space(2.5)).Margin(6, 0, 0, 0).Children(func() {
			permissionRow(c, assets.Sound, locales.SystemAudioOnly(), locales.SystemAudioOnlyDescription(), func() {
				mark(c, 22, granted, k.Asking || k.Checking && k.Access == "", k.Access == "denied")
			})
		})
	})
	// What to do, when the user has something to do: the row's mark says the rest.
	text := ""
	switch {
	case k.Asking:
		text = locales.AccessNoteAsking()
	case k.Access == "denied":
		text = locales.AccessNoteDenied()
	}
	if text == "" {
		return
	}
	g.part(c, 2, func() {
		note(c.Key(text), text).Transition(kit.Rise(0))
		if k.Access == "denied" && kit.Button(c, kit.Plain, kit.Small, assets.Reset, locales.ReopenApp()).AlignSelf(ui.Start).Clicked() {
			mygo.App.Relaunch()
		}
	})
}

// permissionRow is a permission: its icon, its name, what it is for, and at its end what trailing
// builds, its answer.
func permissionRow(c *ui.Context, icon *ui.SVG, name, what string, trailing func()) {
	pal := kit.Colors(c)
	ui.Row(c).Gap(kit.Space(3)).Padding(kit.Space(3), kit.Space(3.5)).Radius(kit.RadiusTile).Background(pal.Card).Border(1, pal.Line).AlignItems(ui.Center).Children(func() {
		const d = 34
		ui.Box(c).Size(d, d).Radius(kit.Pill(d)).Background(pal.LanternTint).Center().Shrink(0).Children(func() {
			ui.Icon(c, icon).FontSize(kit.IconSizeTile).TextColor(pal.LanternText)
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(kit.Space(0.5)).Children(func() {
			ui.Text(c, name).FontSize(kit.TextBody).FontWeight(kit.WeightBold).TextColor(pal.Ink)
			kit.Caption(c, what)
		})
		trailing()
	})
}

// permissionStage is System Settings' switch, turning on as the permission is given, and what the
// app does with what it hears and sees.
func (g *Guide) permissionStage(c *ui.Context) {
	k := g.core
	pal := kit.Colors(c)
	ui.Column(c).Fill().AlignItems(ui.Center).Justify(ui.Center).Gap(kit.Space(5)).Padding(kit.Space(6)).Children(func() {
		glass(c, ui.Column(c).Width(300).Padding(kit.Space(3.5), kit.Space(4)).Gap(kit.Space(3))).Children(func() {
			ui.Row(c).Gap(kit.Space(2)).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, assets.Sound).FontSize(kit.TextBody).TextColor(pal.Starlight.Alpha(.7))
				onStage(c, locales.SystemAudioOnly(), kit.TextCaption, .7).FontWeight(kit.WeightBold)
			})
			ui.Row(c).Gap(kit.Space(2.5)).AlignItems(ui.Center).Children(func() {
				ui.Image(c, assets.AppIcon).Size(26, 26).Margin(-2)
				onStage(c, "Tomoru", kit.TextBody, .95).Grow(1)
				toggle(c, k.Access == "granted")
			})
		})
		glass(c, ui.Column(c).Width(300).Padding(kit.Space(4)).Gap(kit.Space(3.5))).Children(func() {
			onStage(c, locales.PrivacyTitle(), kit.TextBody, .95).FontWeight(kit.WeightBold)
			for _, item := range []struct {
				icon         *ui.SVG
				what, detail string
			}{
				{assets.Sound, locales.PrivacySound(), locales.PrivacySoundDetail()},
				{assets.Picture, locales.PrivacyScreen(), locales.PrivacyScreenDetail()},
				{assets.Lock, locales.PrivacyHistory(), locales.PrivacyHistoryDetail()},
			} {
				ui.Row(c).Gap(kit.Space(2.5)).AlignItems(ui.Start).Children(func() {
					ui.Icon(c, item.icon).FontSize(kit.TextBody).TextColor(pal.Lantern).Margin(2, 0, 0, 0)
					ui.Column(c).Grow(1).Gap(kit.Space(0.5)).Children(func() {
						onStage(c, item.what, kit.TextSmall, .95).FontWeight(kit.WeightBold)
						onStage(c, item.detail, kit.TextCaption, .65).LineHeight(1.4)
					})
				})
			}
		})
	})
}

// toggle is a switch as System Settings draws it, which turns as on changes; only to show.
func toggle(c *ui.Context, on bool) {
	pal := kit.Colors(c)
	const w, h, knob = 34, 20, 16
	track := ui.Box(c).Size(w, h).Radius(kit.Pill(h)).Padding((h - knob) / 2)
	x, bg := float32(0), pal.Starlight.Alpha(.22)
	if on {
		x, bg = w-h, pal.Ok
	}
	track.Background(track.AnimateColor("bg", bg, ui.Animation{Duration: 220 * time.Millisecond})).Children(func() {
		k := ui.Box(c).Size(knob, knob).Radius(kit.Pill(knob)).Background(pal.Starlight).Shadow(0, 1, 3, 0, pal.Shade.Alpha(.3))
		k.Translate(k.AnimateTo("x", x, ui.Animation{Spring: ui.SpringOptions{Duration: 300 * time.Millisecond, Bounce: .2}}), 0)
	})
}
