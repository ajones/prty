package ui

import (
	"sort"
	"time"

	"github.com/cznic/mathutil"
	"github.com/inburst/prty/datasource"
	"github.com/inburst/prty/stats"
	"github.com/inburst/prty/tracking"
)

type AllPrs struct {
	PRView
}

func (p *AllPrs) OnNewPullData(pr *datasource.PullRequest) {
	if !pr.IsApproved {
		p.pulls = append(p.pulls, pr)
		p.needsSort = true
	}
}

func (p *AllPrs) OnSort() {
	sort.Sort(byImportance(p.pulls))
	p.needsSort = false
}

func (p *AllPrs) NeedsSort() bool {
	return p.needsSort
}

func (p *AllPrs) OnSelect(cursor CursorPos, stats *stats.Stats) {
	pull := p.pulls[p.currentlySelectedPullIndex]

	now := time.Now()
	pull.ViewedAt = &now

	openbrowser(*pull.PR.HTMLURL)
	stats.OnOpenPR(pull)
	tracking.SendMetric("open.all")
}

func (p *AllPrs) Clear() {
	p.pulls = []*datasource.PullRequest{}
	p.currentlySelectedPullIndex = 0
}

func (p *AllPrs) OnCursorMove(moxedX int, movedY int) bool {
	if movedY != 0 {
		p.cursor.Y += movedY

		p.cursor.Y = mathutil.Clamp(p.cursor.Y, 0, max(len(p.pulls)-1, 0))
		p.currentlySelectedPullIndex = p.cursor.Y
		return true
	}
	return false
}

func (p *AllPrs) GetSelectedIndex() int {
	return p.currentlySelectedPullIndex
}

func (p *AllPrs) GetPulls() []*datasource.PullRequest {
	return p.pulls
}

func (p *AllPrs) GetSelectedPull() *datasource.PullRequest {
	return p.pulls[p.currentlySelectedPullIndex]
}
