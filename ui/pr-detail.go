package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/inburst/prty/datasource"
)

type PRDetail struct {
	PR *datasource.PullRequest
}

type importanceEntry struct {
	name  string
	value float64
}

type importanceEntryByImportance []importanceEntry

func (a importanceEntryByImportance) Len() int      { return len(a) }
func (a importanceEntryByImportance) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a importanceEntryByImportance) Less(i, j int) bool {
	if a[i].value != a[j].value {
		return a[i].value > a[j].value
	}
	return strings.Compare(a[i].name, a[j].name) > 0
}

func (p *PRDetail) BuildView(viewWidth int, viewHeight int) string {
	doc := strings.Builder{}
	h := lipgloss.Height

	// Begin Title Bar
	additionsStr := fmt.Sprintf("+%d", p.PR.Additions)
	deletionsStr := fmt.Sprintf("-%d", p.PR.Deletions)
	additionsWidth := len(additionsStr)
	deletionsWidth := len(deletionsStr)
	codeDeltaTotalWidth := additionsWidth + deletionsWidth + 4 // +4 for the padding between blocks

	prTitleBlock := prTitleStyle.Copy().Inherit(titleStyle).Width(viewWidth - codeDeltaTotalWidth).Render(p.PR.PR.GetTitle())
	additionsBlock := prAdditionsAndDeletionsStyle.Copy().Foreground(lipgloss.Color("#00FF00")).Width(additionsWidth).Render(additionsStr)
	deletionsBlock := prAdditionsAndDeletionsStyle.Copy().Foreground(lipgloss.Color("#FF0000")).Width(deletionsWidth).Render(deletionsStr)

	titleBar := lipgloss.JoinHorizontal(lipgloss.Top,
		prTitleBlock,
		additionsBlock,
		deletionsBlock,
	)
	doc.WriteString(titleBar + "\n")
	// End Title Bar

	// Begin PR Markdown Body
	r, _ := glamour.NewTermRenderer(
		// detect background color and pick either the default dark or light theme
		glamour.WithStandardStyle("dark"),
		// wrap output at specific width
		//glamour.WithWordWrap(viewWidth-20),
	)
	markdownBody, _ := r.Render(p.PR.PR.GetBody())
	// End PR Markdown Body

	// Begin Importance Display
	entries := []importanceEntry{}
	for k, v := range p.PR.ImportanceLookup {
		entries = append(entries, importanceEntry{
			name:  k,
			value: v,
		})
	}

	sort.Sort(importanceEntryByImportance(entries))
	keys := []string{
		detailSideBarStyle.Copy().Render("Importance"),
		detailSideBarStyle.Copy().Render(""), // blank line
		detailSideBarStyle.Copy().Render("Breakdown:"),
	}
	values := []string{
		detailSideBarStyle.Copy().Render(fmt.Sprintf("%.0f", p.PR.Importance)),
		detailSideBarStyle.Copy().Render(""), // blank line
		detailSideBarStyle.Copy().Render(""), // blank line
	}
	for i := range entries {
		keys = append(keys, detailSideBarStyle.Copy().Render(entries[i].name))
		values = append(values, detailSideBarStyle.Copy().Render(fmt.Sprintf("%.0f", entries[i].value)))
	}

	importanceList := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.JoinVertical(lipgloss.Left,
			keys...,
		),
		lipgloss.JoinVertical(lipgloss.Right,
			values...,
		),
	)
	// End Importance Display

	importanceBreakdownWidth := 25

	markDownBodyBlock := lipgloss.NewStyle().
		Width(viewWidth-importanceBreakdownWidth).
		MaxWidth(viewWidth-importanceBreakdownWidth).
		Height(viewHeight-h(titleBar)).
		Render(markdownBody) + "\n"

	importanceBodyBlock := lipgloss.NewStyle().
		Padding(1, 1, 1, 1).
		Width(importanceBreakdownWidth).
		MaxWidth(importanceBreakdownWidth).
		Height(viewHeight-h(titleBar)).
		Render(importanceList) + "\n"

	bodyBlock := lipgloss.NewStyle().Copy().Render(
		lipgloss.JoinHorizontal(lipgloss.Top,
			markDownBodyBlock,
			importanceBodyBlock,
		))

	renderedBody := lipgloss.NewStyle().
		MaxHeight(viewHeight - h(titleBar)).
		Height(viewHeight - h(titleBar)).
		Render(bodyBlock)

	doc.WriteString(renderedBody)

	return lipgloss.NewStyle().MaxWidth(viewWidth).Render(doc.String()) + "\n"
}
