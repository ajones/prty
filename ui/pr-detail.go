package ui

import (
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/inburst/prty/datasource"
)

type PRDetail struct {
	PR *datasource.PullRequest
}

func (p *PRDetail) BuildView(viewWidth int, viewHeight int) string {
	doc := strings.Builder{}
	h := lipgloss.Height

	title := pullListStyle.Copy().Inherit(titleStyle).Width(viewWidth).Render(p.PR.PR.GetTitle()) + "\n"
	doc.WriteString(title)

	out, _ := glamour.Render(p.PR.PR.GetBody(), "dark")
	doc.WriteString(lipgloss.NewStyle().MaxHeight(viewHeight-h(title)).Height(viewHeight-h(title)).Render(out) + "\n")

	// TODO : add last comment and last commit to view

	return lipgloss.NewStyle().MaxWidth(viewWidth).Render(doc.String()) + "\n"
}
