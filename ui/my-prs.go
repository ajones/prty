package ui

import (
	"sort"
	"time"

	"github.com/cznic/mathutil"
	"github.com/inburst/prty/datasource"
	"github.com/inburst/prty/stats"
	"github.com/inburst/prty/tracking"
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

func (p *MyPrs) NeedsSort() bool {
	return p.needsSort
}

func (p *MyPrs) OnSelect(cursor CursorPos, stats *stats.Stats) {
	pull := p.pulls[p.currentlySelectedPullIndex]

	now := time.Now()
	pull.ViewedAt = &now

	openbrowser(*pull.PR.HTMLURL)
	stats.OnOpenPR(pull)
	tracking.SendMetric("open.my")
}

func (p *MyPrs) Clear() {
	p.pulls = []*datasource.PullRequest{}
	p.currentlySelectedPullIndex = 0
}

func (p *MyPrs) OnCursorMove(moxedX int, movedY int) bool {
	if movedY != 0 {
		p.cursor.Y += movedY

		p.cursor.Y = mathutil.Clamp(p.cursor.Y, 0, max(len(p.pulls)-1, 0))
		p.currentlySelectedPullIndex = p.cursor.Y
		return true
	}
	return false
}

func (p *MyPrs) GetSelectedIndex() int {
	return p.currentlySelectedPullIndex
}

func (p *MyPrs) GetPulls() []*datasource.PullRequest {
	return p.pulls
}

func (p *MyPrs) GetSelectedPull() *datasource.PullRequest {
	return p.pulls[p.currentlySelectedPullIndex]
}
