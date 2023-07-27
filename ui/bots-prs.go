package ui

import (
	"sort"
	"time"

	"github.com/cznic/mathutil"
	"github.com/inburst/prty/datasource"
	"github.com/inburst/prty/stats"
	"github.com/inburst/prty/tracking"
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

func (p *BotsPrs) NeedsSort() bool {
	return p.needsSort
}

func (p *BotsPrs) OnSelect(cursor CursorPos, stats *stats.Stats) {
	pull := p.pulls[p.currentlySelectedPullIndex]

	now := time.Now()
	pull.ViewedAt = &now

	openbrowser(*pull.PR.HTMLURL)
	stats.OnOpenPR(pull)
	tracking.SendMetric("open.bots")
}

func (p *BotsPrs) Clear() {
	p.pulls = []*datasource.PullRequest{}
	p.currentlySelectedPullIndex = 0
}

func (p *BotsPrs) OnCursorMove(moxedX int, movedY int) bool {
	if movedY != 0 {
		p.cursor.Y += movedY

		p.cursor.Y = mathutil.Clamp(p.cursor.Y, 0, max(len(p.pulls)-1, 0))
		p.currentlySelectedPullIndex = p.cursor.Y
		return true
	}
	return false
}

func (p *BotsPrs) GetSelectedIndex() int {
	return p.currentlySelectedPullIndex
}

func (p *BotsPrs) GetPulls() []*datasource.PullRequest {
	return p.pulls
}

func (p *BotsPrs) GetSelectedPull() *datasource.PullRequest {
	return p.pulls[p.currentlySelectedPullIndex]
}
