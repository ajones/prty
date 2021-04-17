package ui

import (
	"sort"
	"time"

	"github.com/cznic/mathutil"
	"github.com/inburst/prty/datasource"
	"github.com/inburst/prty/stats"
)

type ActivePRs struct {
	PRView
}

func (p *ActivePRs) OnNewPullData(pr *datasource.PullRequest) {
	activeWindow := 60 // things updated with 48 hours are considered active add a bit to spread over the weekend

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

func (p *ActivePRs) NeedsSort() bool {
	return p.needsSort
}

func (p *ActivePRs) OnSelect(cursor CursorPos, stats *stats.Stats) {
	pull := p.pulls[p.currentlySelectedPullIndex]

	now := time.Now()
	pull.ViewedAt = &now

	openbrowser(*pull.PR.HTMLURL)
	stats.OnViewedPR(pull)
}

func (p *ActivePRs) Clear() {
	p.pulls = []*datasource.PullRequest{}
}

func (p *ActivePRs) OnCursorMove(moxedX int, movedY int) bool {
	if movedY != 0 {
		p.cursor.Y += movedY

		p.cursor.Y = mathutil.Clamp(p.cursor.Y, 0, max(len(p.pulls)-1, 0))
		p.currentlySelectedPullIndex = p.cursor.Y
		return true
	}
	return false
}

func (p *ActivePRs) GetSelectedIndex() int {
	return p.currentlySelectedPullIndex
}

func (p *ActivePRs) GetPulls() []*datasource.PullRequest {
	return p.pulls
}

func (p *ActivePRs) GetSelectedPull() *datasource.PullRequest {
	return p.pulls[p.currentlySelectedPullIndex]
}
