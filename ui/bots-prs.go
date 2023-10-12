package ui

import (
	"sort"

	"github.com/inburst/prty/datasource"
)

type BotsPrs struct {
	PRView
}

func (p *BotsPrs) OnNewPullData(pr *datasource.PullRequest) {
	if pr.AuthorIsBot && !pr.IsApproved && !pr.IsAbandoned {
		p.pulls = append(p.pulls, pr)
		p.needsSort = true
	}
}

func (p *BotsPrs) OnSort() {
	sort.Sort(byImportance(p.pulls))
	p.needsSort = false
}
