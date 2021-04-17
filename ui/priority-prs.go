package ui

import (
	"sort"
	"time"

	"github.com/cznic/mathutil"
	"github.com/inburst/prty/datasource"
	"github.com/inburst/prty/stats"
)

/*
Got data
- PR author
- requested reviewers / Are there people tagged in the PR


- number of comments and comment authors
- Time since last comment
- has there been changes since last comment
- have I made comments on this PR
- total age of PR
*/

type PriorityPRs struct {
	PRView
}

func (p *PriorityPRs) OnNewPullData(pr *datasource.PullRequest) {
	if !pr.IsAbandoned && !pr.IsDraft && !pr.IsApproved && !pr.AuthorIsBot && pr.HasChangesAfterLastComment {
		p.pulls = append(p.pulls, pr)
		p.needsSort = true
	}
}

func (p *PriorityPRs) OnSort() {
	sort.Sort(byImportance(p.pulls))
	p.needsSort = false
}

func (p *PriorityPRs) NeedsSort() bool {
	return p.needsSort
}

func (p *PriorityPRs) OnSelect(cursor CursorPos, stats *stats.Stats) {
	pull := p.pulls[p.currentlySelectedPullIndex]

	now := time.Now()
	pull.ViewedAt = &now

	openbrowser(*pull.PR.HTMLURL)
	stats.OnViewedPR(pull)
}

func (p *PriorityPRs) Clear() {
	p.pulls = []*datasource.PullRequest{}
	p.currentlySelectedPullIndex = 0
}

func (p *PriorityPRs) OnCursorMove(moxedX int, movedY int) bool {
	if movedY != 0 {
		p.cursor.Y += movedY

		p.cursor.Y = mathutil.Clamp(p.cursor.Y, 0, max(len(p.pulls)-1, 0))
		p.currentlySelectedPullIndex = p.cursor.Y
		return true
	}
	return false
}

func (p *PriorityPRs) GetSelectedIndex() int {
	return p.currentlySelectedPullIndex
}

func (p *PriorityPRs) GetPulls() []*datasource.PullRequest {
	return p.pulls
}

func (p *PriorityPRs) GetSelectedPull() *datasource.PullRequest {
	return p.pulls[p.currentlySelectedPullIndex]
}
