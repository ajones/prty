package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/cznic/mathutil"
	"github.com/inburst/prty/datasource"
	"github.com/lucasb-eyer/go-colorful"
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
	pulls []*datasource.PullRequest

	needsSort                  bool
	currentlySelectedPullIndex int
	cursor                     CursorPos

	statusChan chan<- string
}

type byImportance []*datasource.PullRequest

func (s byImportance) Len() int {
	return len(s)
}
func (s byImportance) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s byImportance) Less(i, j int) bool {
	return s[i].Importance > s[j].Importance
}

func (p *PriorityPRs) OnNewPullData(pr *datasource.PullRequest) {
	p.pulls = append(p.pulls, pr)
	p.needsSort = true
}

func (p *PriorityPRs) OnSort() {
	sort.Sort(byImportance(p.pulls))
	p.needsSort = false
}

func (p *PriorityPRs) OnSelect(cursor CursorPos) {
	pull := p.pulls[p.currentlySelectedPullIndex]

	now := time.Now()
	pull.ViewedAt = &now

	openbrowser(*pull.PR.HTMLURL)
}

func (p *PriorityPRs) Clear() {
	p.pulls = []*datasource.PullRequest{}
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

func (p *PriorityPRs) BuildTabHeader(viewWidth int, viewHeight int) string {
	doc := strings.Builder{}
	var (
		colors = colorGrid(1, 5)
		title  strings.Builder
	)
	for i, v := range colors {
		const offset = 2
		c := lipgloss.Color(v[0])
		fmt.Fprint(&title, titleStyle.Copy().MarginLeft(i*offset).Background(c).SetString("PRTY 🎉"))
		if i < len(colors)-1 {
			title.WriteRune('\n')
		}
	}

	desc := lipgloss.JoinVertical(lipgloss.Left,
		descStyle.Render("Intellegent PR priority"),
		infoStyle.Render("Author: "+divider+url("https://github.com/ajones")),
	)

	row := lipgloss.JoinHorizontal(lipgloss.Top, title.String(), desc)
	doc.WriteString("\n" + row + "\n\n")

	return lipgloss.NewStyle().
		MaxWidth(viewWidth).
		Height(viewHeight).
		MaxHeight(viewHeight).
		Render(doc.String()) + "\n"
}

func (p *PriorityPRs) BuildPRDetailBody(viewWidth int, pr *datasource.PullRequest) string {
	prStatus := strings.Builder{}

	//prStatus.WriteString(
	//	pullListStyle.Copy().Width(viewWidth).Render(fmt.Sprintf("Author: %s", pr.Author)))
	/*
			prStatus.WriteString(
			pullListStyle.Copy().Width(viewWidth).Render(fmt.Sprintf("Repo: %s", pr.RepoName)) + "\n")
		prStatus.WriteString(
			pullListStyle.Copy().Width(viewWidth).Render(fmt.Sprintf("Labels: %s", strings.Join(pr.Labels, " "))) + "\n")
		prStatus.WriteString(
			pullListStyle.Copy().Width(viewWidth).Render(fmt.Sprintf("Requested: %s", strings.Join(pr.RequestedReviewers, " "))) + "\n")
		prStatus.WriteString(
			pullListStyle.Copy().Width(viewWidth).Render(fmt.Sprintf("num commits: %d", pr.NumCommits)) + "\n")

		prStatus.WriteString(
			pullListStyle.Copy().Width(viewWidth).Render(*pr.PR.CommentsURL) + "\n")
	*/

	return prStatus.String() + "\n"
}

func (p *PriorityPRs) BuildPRFooter(viewWidth int, pr *datasource.PullRequest) string {
	foot := strings.Builder{}

	w := lipgloss.Width

	var statusKey string
	if pr.IsApproved {
		statusKey = statusApprovedStyle.Render("APPROVED")
	} else if pr.IsAbandoned {
		statusKey = statusStyle.Render("ABANDONED 💀")
	} else if pr.IsDraft {
		statusKey = statusStyle.Render("DRAFT")
	} else if pr.HasChangesAfterLastComment {
		statusKey = statusAlertStyle.Render("NEEDS REVIEW")
	} else {
		statusKey = statusStyle.Render("OK")
	}

	viewedIcon := ""
	if pr.ViewedAt != nil {
		viewedIcon = " ✅ "
	}

	orgTag := orgStatusTagStyle.Render(fmt.Sprintf("%s/%s", pr.OrgName, pr.RepoName))
	age := time.Now().Sub(pr.LastCommitTime)
	totalWait := encodingStyle.Render(fmt.Sprintf("Wait %sh", formatDurationDayHour(age)))
	author := fishCakeStyle.Render(pr.Author)

	// TODO : properly calculate the width after padding applied
	commitsCount := statusTextStyle.Copy().Render(fmt.Sprintf("Commits: %d", pr.NumCommits))
	//Width(viewWidth - 2 - w(viewedIcon) - w(statusKey) - w(totalWait) - w(author) - w(orgTag)).

	viewStatus := statusTextStyle.Copy().
		Width(viewWidth - 2 - w(statusKey) - w(commitsCount) - w(orgTag) - w(totalWait) - w(author)).
		Render(viewedIcon)

	bar := lipgloss.JoinHorizontal(lipgloss.Top,
		statusKey,
		commitsCount,
		viewStatus,
		orgTag,
		totalWait,
		author,
	)

	foot.WriteString(statusBarStyle.Render(bar) + "\n")

	return foot.String()
}

func (p *PriorityPRs) BuildView(viewWidth int, viewHeight int) string {
	doc := strings.Builder{}
	headerHeight := 7
	pullPosHeight := 1
	bodyHeight := viewHeight - headerHeight - pullPosHeight

	doc.WriteString(p.BuildTabHeader(viewWidth, headerHeight))

	msg := strings.Builder{}
	if p.needsSort {
		msg.WriteString("NEEDS (S)ORT  ")
	}
	msg.WriteString(fmt.Sprintf("%d of %d", p.currentlySelectedPullIndex+1, len(p.pulls)))
	doc.WriteString(pullPositionStyle.Copy().Width(viewWidth).Render(msg.String()) + "\n")

	prSection := strings.Builder{}
	viewablePulls := p.pulls[p.currentlySelectedPullIndex:len(p.pulls)]
	for i := range viewablePulls {
		pr := viewablePulls[i]

		if i == 0 {
			prSection.WriteString(
				pullListStyleSelected.Copy().Width(viewWidth).Render(fmt.Sprintf(">>> %s", *pr.PR.Title)))
		} else {
			prSection.WriteString(
				pullListStyle.Copy().Width(viewWidth).Render(*pr.PR.Title))
		}

		prSection.WriteString(p.BuildPRDetailBody(viewWidth, pr))
		prSection.WriteString(p.BuildPRFooter(viewWidth, pr))
		prSection.WriteString("\n")
	}

	doc.WriteString(docStyle.Copy().
		Height(bodyHeight).
		MaxHeight(bodyHeight).
		MaxWidth(viewWidth).
		Render(prSection.String()))

	return lipgloss.NewStyle().MaxWidth(viewWidth).Render(doc.String()) + "\n"
}

func colorGrid(xSteps, ySteps int) [][]string {
	x0y0, _ := colorful.Hex("#F25D94")
	x1y0, _ := colorful.Hex("#EDFF82")
	x0y1, _ := colorful.Hex("#643AFF")
	x1y1, _ := colorful.Hex("#14F9D5")

	x0 := make([]colorful.Color, ySteps)
	for i := range x0 {
		x0[i] = x0y0.BlendLuv(x0y1, float64(i)/float64(ySteps))
	}

	x1 := make([]colorful.Color, ySteps)
	for i := range x1 {
		x1[i] = x1y0.BlendLuv(x1y1, float64(i)/float64(ySteps))
	}

	grid := make([][]string, ySteps)
	for x := 0; x < ySteps; x++ {
		y0 := x0[x]
		grid[x] = make([]string, xSteps)
		for y := 0; y < xSteps; y++ {
			grid[x][y] = y0.BlendLuv(x1[x], float64(y)/float64(xSteps)).Hex()
		}
	}

	return grid
}
