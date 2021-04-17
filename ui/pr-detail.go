package ui

import (
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/inburst/prty/datasource"
)

type PRDetail struct {
	PR *datasource.PullRequest
}

func (p *PRDetail) BuildView(w int, h int) string {
	doc := strings.Builder{}

	doc.WriteString(pullListStyle.Copy().Inherit(titleStyle).Width(w).Render(p.PR.PR.GetTitle()) + "\n")

	out, _ := glamour.Render(p.PR.PR.GetBody(), "dark")
	doc.WriteString(out + "\n")

	// TODO : add last comment and last commit to view

	return docStyle.Copy().MaxWidth(w).Render(doc.String())
}
