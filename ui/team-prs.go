package ui

import (
	"sort"
	"time"

	"github.com/cznic/mathutil"
	"github.com/inburst/prty/datasource"
	"github.com/inburst/prty/stats"
	"github.com/inburst/prty/tracking"
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

func (p *TeamPrs) NeedsSort() bool {
	return p.needsSort
}

func (p *TeamPrs) OnSelect(cursor CursorPos, stats *stats.Stats) {
	pull := p.pulls[p.currentlySelectedPullIndex]

	now := time.Now()
	pull.ViewedAt = &now

	openbrowser(*pull.PR.HTMLURL)
	stats.OnViewedPR(pull)
	tracking.SendMetric("open.team")
}

func (p *TeamPrs) Clear() {
	p.pulls = []*datasource.PullRequest{}
	p.currentlySelectedPullIndex = 0
}

func (p *TeamPrs) OnCursorMove(moxedX int, movedY int) bool {
	if movedY != 0 {
		p.cursor.Y += movedY

		p.cursor.Y = mathutil.Clamp(p.cursor.Y, 0, max(len(p.pulls)-1, 0))
		p.currentlySelectedPullIndex = p.cursor.Y
		return true
	}
	return false
}

func (p *TeamPrs) GetSelectedIndex() int {
	return p.currentlySelectedPullIndex
}

func (p *TeamPrs) GetPulls() []*datasource.PullRequest {
	return p.pulls
}

func (p *TeamPrs) GetSelectedPull() *datasource.PullRequest {
	return p.pulls[p.currentlySelectedPullIndex]
}
