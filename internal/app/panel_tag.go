package app

import "github.com/egoist/mygo/ui"

// tagTone is a RunwayTagTone: the tint of a capsule tag.
type tagTone int

const (
	toneNeutral tagTone = iota
	toneGray
	toneBlue
	tonePurple
	toneOrange
	toneYellow
	toneGreen
	toneRed
	toneTeal
)

// tagLook is the resolved colors of a capsule tag.
type tagLook struct{ foreground, background, stroke ui.Color }

func toneTint(t *ui.Theme, tone tagTone) ui.Color {
	switch tone {
	case toneGray:
		return sys(t, sysGray)
	case toneBlue:
		return sys(t, sysBlue)
	case tonePurple:
		return sys(t, sysPurple)
	case toneOrange:
		return sys(t, sysOrange)
	case toneYellow:
		if t.Dark {
			return sys(t, sysYellow)
		}
		return sys(t, sysOrange)
	case toneGreen:
		return sys(t, sysGreen)
	case toneRed:
		return sys(t, sysRed)
	case toneTeal:
		return sys(t, sysTeal)
	}
	return inkSecondary(t)
}

func resolveTag(t *ui.Theme, tone tagTone) tagLook {
	if tone == toneNeutral {
		if t.Dark {
			return tagLook{inkSecondary(t), ui.RGBA(255, 255, 255, 0.14), ui.RGBA(255, 255, 255, 0.65*0.18)}
		}
		return tagLook{ui.RGBA(0, 0, 0, 0.85), ui.RGBA(0, 0, 0, 0.08), ui.RGBA(0, 0, 0, 0.95*0.1)}
	}
	tint := toneTint(t, tone)
	if !t.Dark {
		return tagLook{brandInk(t, tint), tint.Alpha(0.14), tint.Alpha(0.35)}
	}
	return tagLook{tint, tint.Alpha(0.22), tint.Alpha(0.42)}
}

// runwayTag is the RunwayTag capsule: 10pt semibold text, 7pt by 3pt padding.
func runwayTag(c *ui.Context, text string, tone tagTone) { runwayTagPadded(c, text, tone, 3, 7) }

// runwayTagPadded is runwayTag with its own padding.
func runwayTagPadded(c *ui.Context, text string, tone tagTone, vertical, horizontal float32) {
	look := resolveTag(c.Theme(), tone)
	width := float32(1)
	ui.Row(c).Shrink(0).Padding(vertical, horizontal).Radius(999).Background(look.background).Border(width, look.stroke).AlignItems(ui.Center).Label(text).Children(func() {
		ui.Text(c, text).FontSize(fsCaption).FontWeight(600).TextColor(look.foreground).SingleLine()
	})
}
