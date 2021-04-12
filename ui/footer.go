package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Footer struct{}

func (f *Footer) BuildView(viewWidth int, viewHeight int, statusMsg string) string {
	doc := strings.Builder{}

	w := lipgloss.Width

	statusKey := statusStyle.Render("STATUS")
	encoding := encodingStyle.Render("GSD")
	fishCake := fishCakeStyle.Render("PRTY ⏱")
	statusVal := statusTextStyle.Copy().
		Width(viewWidth - w(statusKey) - w(encoding) - w(fishCake)).
		Render(statusMsg)

	bar := lipgloss.JoinHorizontal(lipgloss.Top,
		statusKey,
		statusVal,
		encoding,
		fishCake,
	)

	doc.WriteString(statusBarStyle.Width(viewWidth).Render(bar))

	return lipgloss.NewStyle().Height(viewHeight).Render(doc.String())
}
