package ui

import (
	"sort"
	"time"

	"github.com/inburst/prty/datasource"
)

type ActivePRs struct {
	PRView
}

func (p *ActivePRs) OnNewPullData(pr *datasource.PullRequest) {
	activeWindow := (24 * 2) + 12 // things updated with 48 hours are considered active add a bit to spread over the weekend

	mostRecentActivityTime := pr.LastCommitTime
	if pr.LastCommentTime.After(pr.LastCommitTime) {
		mostRecentActivityTime = pr.LastCommentTime
	}
	hoursSinceActivity := int(time.Now().Sub(mostRecentActivityTime) / time.Hour)

	if !pr.IsApproved && hoursSinceActivity < activeWindow {
		p.pulls = append(p.pulls, pr)
		p.needsSort = true
	}
}

func (p *ActivePRs) OnSort() {
	sort.Sort(byImportance(p.pulls))
	p.needsSort = false
}
