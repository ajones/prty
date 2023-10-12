package ui

import (
	"sort"

	"github.com/inburst/prty/datasource"
)

type AllPrs struct {
	PRView
}

func (p *AllPrs) OnNewPullData(pr *datasource.PullRequest) {
	// if !pr.IsApproved {
	p.pulls = append(p.pulls, pr)
	p.needsSort = true
	// }
}

func (p *AllPrs) OnSort() {
	sort.Sort(byImportance(p.pulls))
	p.needsSort = false

	p.selectedPRInteractionState = PRInteractionStateSelected
}
