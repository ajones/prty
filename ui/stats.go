package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/inburst/prty/stats"
)

type Stats struct {
	UserStats *stats.Stats
}

func (p *Stats) BuildView(viewWidth int, viewHeight int) string {
	doc := strings.Builder{}

	statItem := lipgloss.NewStyle().Padding(0, 1).Width(16)

	statsList := list.Copy().Width(viewWidth).Height(viewHeight).Align(lipgloss.Center).Padding(0, 2).Render(
		lipgloss.JoinVertical(lipgloss.Center,
			statItem.Render("\n"),
			listHeader("📈 PRTY Stats 📈"),
			lipgloss.JoinHorizontal(lipgloss.Top,
				lipgloss.JoinVertical(lipgloss.Right,
					statItem.Render("Lifetime Opens"),
					statItem.Render("\n"),
				),
				lipgloss.JoinVertical(lipgloss.Right,
					statItem.Render(fmt.Sprintf("%d", p.UserStats.LifetimePROpens)),
					statItem.Render("\n"),
				),
			),
		),
	)

	doc.WriteString(statsList)

	return lipgloss.NewStyle().MaxWidth(viewWidth).Render(doc.String()) + "\n"
}
