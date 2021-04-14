package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type Footer struct{}

func (f *Footer) BuildView(viewWidth int, viewHeight int, statusMsg string) string {
	doc := strings.Builder{}

	w := lipgloss.Width

	statusKey := tagStyle.Copy().Render("STATUS:")
	encoding := tagSuccessStyle.Copy().Render("GSD")
	fishCake := tagSpecialStyle.Copy().Align(lipgloss.Right).Render("PRTY ⏱")
	statusVal := lipgloss.NewStyle().
		Width(viewWidth - w(statusKey) - w(encoding) - w(fishCake)).
		Render(statusMsg)

	bar := lipgloss.JoinHorizontal(lipgloss.Top,
		statusKey,
		statusVal,
		encoding,
		fishCake,
	)

	doc.WriteString(lipgloss.NewStyle().Width(viewWidth).Render(bar))

	return lipgloss.NewStyle().Height(viewHeight).Render(doc.String())
}
