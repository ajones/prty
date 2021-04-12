package ui

import (
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/google/go-github/github"
)

type PRDetail struct{}

func (p *PRDetail) BuildView(w int, h int, pull *github.PullRequest) string {
	//physicalWidth, _, _ := term.GetSize(int(os.Stdout.Fd()))
	doc := strings.Builder{}

	doc.WriteString(pullListStyle.Copy().Inherit(titleStyle).Width(w).Render(*pull.Title) + "\n")

	out, _ := glamour.Render(*pull.Body, "dark")
	doc.WriteString(out + "\n")

	return docStyle.Copy().MaxWidth(w).Render(doc.String())
}
