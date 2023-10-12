package ui

import (
	"sort"

	"github.com/inburst/prty/datasource"
)

type TeamPrs struct {
	PRView
}

func (p *TeamPrs) OnNewPullData(pr *datasource.PullRequest) {
	if (pr.AuthorIsTeammate || pr.IAmAuthor) && !pr.IsApproved && !pr.IsAbandoned {
		p.pulls = append(p.pulls, pr)
		p.needsSort = true
	}
}

func (p *TeamPrs) OnSort() {
	sort.Sort(byImportance(p.pulls))
	p.needsSort = false
}
