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

	title := pullListStyle.Copy().Inherit(titleStyle).Width(viewWidth).Render(p.PR.PR.GetTitle()) + "\n"
	doc.WriteString(title)

	r, _ := glamour.NewTermRenderer(
		// detect background color and pick either the default dark or light theme
		glamour.WithStandardStyle("dark"),
		// wrap output at specific width
		//glamour.WithWordWrap(viewWidth-20),
	)
	out, _ := r.Render(p.PR.PR.GetBody())

	entries := []importanceEntry{}
	for k, v := range p.PR.ImportanceLookup {
		entries = append(entries, importanceEntry{
			name:  k,
			value: v,
		})
	}
	sort.Sort(importanceEntryByImportance(entries))
	keys := []string{}
	values := []string{}
	for i := range entries {
		keys = append(keys, detailSideBarStyle.Copy().Render(entries[i].name))
		values = append(values, detailSideBarStyle.Copy().Render(fmt.Sprintf("%0.2f", entries[i].value)))
	}

	importanceList := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.JoinVertical(lipgloss.Left,
			keys...,
		),
		lipgloss.JoinVertical(lipgloss.Right,
			values...,
		),
	)

	sideBarWidth := 25
	view := lipgloss.NewStyle().Copy().Render(
		lipgloss.JoinHorizontal(lipgloss.Top,
			lipgloss.NewStyle().Width(viewWidth-sideBarWidth).MaxWidth(viewWidth-sideBarWidth).Height(viewHeight-h(title)).Render(out),
			lipgloss.NewStyle().Width(sideBarWidth).MaxWidth(sideBarWidth).Height(viewHeight-h(title)).Render(importanceList)))

	doc.WriteString(lipgloss.NewStyle().MaxHeight(viewHeight-h(title)).Height(viewHeight-h(title)).Render(view) + "\n")

	// TODO : add last comment and last commit to view

	return lipgloss.NewStyle().MaxWidth(viewWidth).Render(doc.String()) + "\n"
}
