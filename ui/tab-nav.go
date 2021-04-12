package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type TabNav struct{}

func (t *TabNav) BuildView(viewWidth int, viewHeight int, tabNames []string, selectedTabIndex int) string {
	doc := strings.Builder{}

	renderedTabs := []string{}
	for i := range tabNames {
		if i == selectedTabIndex {
			renderedTabs = append(renderedTabs, activeTab.Render(tabNames[i]))
		} else {
			renderedTabs = append(renderedTabs, tab.Render(tabNames[i]))
		}
	}

	// Tabs
	row := lipgloss.JoinHorizontal(
		lipgloss.Bottom,
		renderedTabs...,
	)

	gap := tabGap.Render(strings.Repeat(" ", max(0, viewWidth-lipgloss.Width(row)-2)))
	row = lipgloss.JoinHorizontal(lipgloss.Bottom, row, gap)

	doc.WriteString(lipgloss.NewStyle().
		Height(viewHeight).
		MaxHeight(viewHeight).
		Render(row) + "\n")

	return doc.String()
}
