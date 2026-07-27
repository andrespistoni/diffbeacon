package ui

import "diffbeacon/internal/app"

const (
	minTwoPanelWidth       = 72
	minBodyHeight          = 6
	minSideBySideDiffWidth = 58
)

type layoutPlan struct {
	Width           int
	BodyHeight      int
	Compact         bool
	ListWidth       int
	ContentWidth    int
	EffectiveLayout app.Layout
	Reason          string
}

func planLayout(width, height, helpHeight int, requested app.Layout) layoutPlan {
	if width <= 0 {
		width = defaultWidth
	}
	if height <= 0 {
		height = defaultHeight
	}
	// BodyHeight is the panel's inner height. Borders, status, and help consume
	// exactly the remaining rows; one inner row is reserved for the panel title.
	plan := layoutPlan{Width: width, BodyHeight: max(2, height-helpHeight-3), EffectiveLayout: requested}
	plan.Compact = width < minTwoPanelWidth || plan.BodyHeight < minBodyHeight
	if plan.Compact {
		plan.ListWidth = max(1, width-2)
		plan.ContentWidth = plan.ListWidth
		plan.Reason = "Compact layout: terminal is too small for two panels; Enter/Tab switches files and content."
	} else {
		leftOuter := max(24, width/3)
		if leftOuter > width-30 {
			leftOuter = max(16, width/2)
		}
		plan.ListWidth = max(1, leftOuter-2)
		plan.ContentWidth = max(1, width-leftOuter-2)
	}
	if requested == app.LayoutSideBySide && plan.ContentWidth < minSideBySideDiffWidth {
		plan.EffectiveLayout = app.LayoutInline
		fallback := "Side-by-side unavailable at this width; showing synchronized inline viewport."
		if plan.Reason == "" {
			plan.Reason = fallback
		} else {
			plan.Reason += " " + fallback
		}
	}
	return plan
}
