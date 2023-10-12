package ui

import (
	"sort"

	"github.com/inburst/prty/datasource"
)

type MyPrs struct {
	PRView
}

func (p *MyPrs) OnNewPullData(pr *datasource.PullRequest) {
	if pr.IAmAuthor && !pr.IsApproved && !pr.IsAbandoned {
		p.pulls = append(p.pulls, pr)
		p.needsSort = true
	}
}

func (p *MyPrs) OnSort() {
	sort.Sort(byImportance(p.pulls))
	p.needsSort = false
}
