package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/cznic/mathutil"
	"github.com/inburst/prty/datasource"
	"github.com/inburst/prty/stats"
	"github.com/inburst/prty/tracking"
	"github.com/lucasb-eyer/go-colorful"
)

// This is to prevent the UI from getting too slow when rendering a large number
// of PRs
// TODO: this should be driven by the size of the terminal not a hardcoded value
const MaxRenderedPRs = 50

type PRInteractionState string

const (
	PRInteractionStateNone     PRInteractionState = ""
	PRInteractionStateSelected PRInteractionState = "selected"
	PRInteractionStateViewed   PRInteractionState = "viewed"
	PRInteractionStateOpened   PRInteractionState = "opened"
)

type PRViewData interface {
	GetSelectedIndex() int
	NeedsSort() bool
	GetPulls() []*datasource.PullRequest
	GetSelectedPull() *datasource.PullRequest

	OnSort()
	Clear()
	OnSelect(cursor CursorPos, stats *stats.Stats)
	OnViewDetails(cursor CursorPos, stats *stats.Stats)
	OnCursorMove(moxedX int, movedY int, stats *stats.Stats) bool
	OnNewPullData(pr *datasource.PullRequest)
}

type PRView struct {
	pulls []*datasource.PullRequest

	needsSort                  bool
	currentlySelectedPullIndex int
	cursor                     CursorPos

	selectedPRInteractionState PRInteractionState
}

func (p *PRView) Clear() {
	p.pulls = []*datasource.PullRequest{}
	p.currentlySelectedPullIndex = 0
}

func (p *PRView) OnSelectedPRChange(from *datasource.PullRequest, to *datasource.PullRequest, lastInteractionState PRInteractionState, stats *stats.Stats) {
	if lastInteractionState == PRInteractionStateSelected || lastInteractionState == PRInteractionStateNone {
		// the previous PR was not interacted with
		stats.OnPassPR(from)
	}
	// for now all we are looking for here is situations where PRs were passed over
}

func (p *PRView) GetSelectedIndex() int {
	return p.currentlySelectedPullIndex
}

func (p *PRView) GetPulls() []*datasource.PullRequest {
	return p.pulls
}

func (p *PRView) GetSelectedPull() *datasource.PullRequest {
	return p.pulls[p.currentlySelectedPullIndex]
}
func (p *PRView) OnViewDetails(cursor CursorPos, stats *stats.Stats) {
	p.selectedPRInteractionState = PRInteractionStateViewed
	stats.OnViewPR(p.GetSelectedPull())
}

func (p *PRView) OnCursorMove(moxedX int, movedY int, stats *stats.Stats) bool {
	p.cursor.Y += movedY
	p.cursor.Y = mathutil.Clamp(p.cursor.Y, 0, max(len(p.pulls)-1, 0))

	if movedY == 0 {
		return false
	}

	// cursor moved
	if p.currentlySelectedPullIndex != p.cursor.Y {
		currentPR := p.pulls[p.currentlySelectedPullIndex]
		nextPR := p.pulls[p.cursor.Y]
		p.OnSelectedPRChange(currentPR, nextPR, p.selectedPRInteractionState, stats)
	}

	p.currentlySelectedPullIndex = p.cursor.Y
	p.selectedPRInteractionState = PRInteractionStateSelected
	return true
}

func (p *PRView) NeedsSort() bool {
	return p.needsSort
}

func (p *PRView) OnSelect(cursor CursorPos, stats *stats.Stats) {
	pull := p.pulls[p.currentlySelectedPullIndex]

	now := time.Now()
	pull.ViewedAt = &now

	openbrowser(*pull.GithubPR.HTMLURL)
	stats.OnOpenPR(pull)
	tracking.SendMetric("open.active")
	p.selectedPRInteractionState = PRInteractionStateOpened
}

func BuildHeader(viewWidth int, viewHeight int) string {
	w := lipgloss.Width
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
	renderedTitle := lipgloss.NewStyle().Padding(0, 2).Render(title.String())

	shortcuts := list.Copy().Width(40).Padding(0, 2).Render(
		lipgloss.JoinVertical(lipgloss.Center,
			listHeader("Keyboard Shortcuts"),
			lipgloss.JoinHorizontal(lipgloss.Top,
				lipgloss.JoinVertical(lipgloss.Left,
					listItem("[r]eload"),
					listItem("[s]ort"),
					listItem("[o|entr|sp] open"),
					listItem("[arrows] move"),
				),
				lipgloss.JoinVertical(lipgloss.Left,
					listItem("[d]escription"),
					listItem("[z] stats"),
					listItem("[esc] back"),
					listItem("[hjkl] vim move"),
				),
			),
		),
	)

	desc := lipgloss.NewStyle().Width(viewWidth-w(renderedTitle)-w(shortcuts)).Padding(0, 2).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			descStyle.Render("Intellegent PR Review Priority"),
			infoStyle.Render("Author: "+divider+url("https://github.com/ajones")),
		),
	)

	row := lipgloss.JoinHorizontal(lipgloss.Top, renderedTitle, desc, shortcuts)
	doc.WriteString("\n" + row + "\n")

	return lipgloss.NewStyle().
		MaxWidth(viewWidth).
		Height(viewHeight).
		MaxHeight(viewHeight).
		Render(doc.String()) + "\n"
}

func BuildPRView(p PRViewData, viewWidth int, viewHeight int, isRefreshing bool) string {
	doc := strings.Builder{}
	pullPosHeight := 1
	bodyHeight := viewHeight - pullPosHeight

	selectedIndex := p.GetSelectedIndex()
	pulls := p.GetPulls()

	msg := strings.Builder{}
	if p.NeedsSort() {
		msg.WriteString(lipgloss.NewStyle().Bold(true).Render("NEEDS (S)ORT  "))
	}
	if len(pulls) > 0 {
		msg.WriteString(fmt.Sprintf("%d of %d", selectedIndex+1, len(pulls)))
		doc.WriteString(pullPositionStyle.Copy().Width(viewWidth).Render(msg.String()) + "\n")
	} else {
		doc.WriteString(lipgloss.NewStyle().Width(viewWidth).Render("") + "\n")
	}

	prSection := strings.Builder{}
	if len(pulls) > 0 {
		viewablePulls := pulls[selectedIndex:mathutil.Min(selectedIndex+MaxRenderedPRs, len(pulls))]
		for i := range viewablePulls {
			pr := viewablePulls[i]

			if i == 0 {
				prSection.WriteString(
					pullListStyleSelected.Copy().Width(viewWidth).Render(fmt.Sprintf(">>> %s", pr.GithubPR.GetTitle())))
			} else {
				prSection.WriteString(
					pullListStyle.Copy().Width(viewWidth).Render(*pr.GithubPR.Title))
			}
			prSection.WriteString("\n")
			prSection.WriteString(BuildPRFooter(p, viewWidth, pr))
			prSection.WriteString("\n")
		}
	} else {
		if isRefreshing {
			prSection.WriteString(lipgloss.NewStyle().Width(viewWidth).Align(lipgloss.Center).Render("refreshing..."))
		} else {
			prSection.WriteString(lipgloss.NewStyle().Width(viewWidth).Align(lipgloss.Center).Render("nothing to show\nhere is a cat 🐈\n\n[r]eload"))
		}
	}

	doc.WriteString(docStyle.Copy().
		Height(bodyHeight).
		MaxHeight(bodyHeight).
		MaxWidth(viewWidth).
		Render(prSection.String()))

	return lipgloss.NewStyle().MaxWidth(viewWidth).Render(doc.String()) + "\n"
}

func BuildPRFooter(p PRViewData, viewWidth int, pr *datasource.PullRequest) string {
	foot := strings.Builder{}

	w := lipgloss.Width

	var statusTag string
	if pr.IsApproved {
		statusTag = prTagStyle.Copy().Width(20).Inherit(tagSuccessStyle).Render("APPROVED")
	} else if pr.IsAbandoned {
		statusTag = prTagStyle.Copy().Width(20).Inherit(tagStyle).Background(darkerGrey).Render("ABANDONED 💀")
	} else if pr.IsDraft {
		statusTag = prTagStyle.Copy().Width(20).Inherit(tagStyle).Render("DRAFT")
	} else if pr.HasChangesAfterLastComment {
		statusTag = prTagStyle.Copy().Width(20).Inherit(tagAlertStyle).Render("NEEDS REVIEW")
	} else {
		statusTag = prTagStyle.Copy().Width(20).Inherit(tagStyle).Render("OK")
	}

	beenViewedTag := ""
	if pr.ViewedAt != nil {
		beenViewedTag = prTagStyle.Copy().Inherit(tagSuccessStyle).Copy().Width(20).Render("VIEWED")
	}

	newChangesTag := ""
	if pr.HasNewChanges {
		newChangesTag = prTagStyle.Copy().Inherit(tagBlueStyle).Copy().Width(20).Render("NEW CHANGES")
	}

	// upper
	age := time.Since(pr.FirstCommitTime)
	commitsCountTag := prTagStyle.Copy().Width(20).Render(fmt.Sprintf("Commits: %d", pr.NumCommits))

	orgRepoStr := fmt.Sprintf("%s/%s", pr.OrgName, pr.RepoName)
	orgRepoTag := prTagStyle.Copy().Render(orgRepoStr)

	ageTag := prTagRightStyle.Copy().Render(fmt.Sprintf("Age %sh", formatDurationDayHour(age)))

	upperSpacerWidth := viewWidth - 2 - w(commitsCountTag) - w(orgRepoTag) - w(ageTag)
	upperSpacer := prTagSpacerStyle.Copy().Width(upperSpacerWidth).Render("")

	// lower
	wait := time.Now().Sub(pr.LastCommitTime)
	waitTag := prTagStyle.Copy().Width(20).Render(fmt.Sprintf("Wait %sh", formatDurationDayHour(wait)))

	authorTag := prTagRightStyle.Copy().Render(pr.Author)

	lowerSpacerWidth := viewWidth - 2 - w(statusTag) - w(waitTag) - w(beenViewedTag) - w(authorTag) - w(newChangesTag)
	lowerSpacer := prTagStyle.Copy().
		Width(lowerSpacerWidth).
		Render("")

	/*
		fields:
		- status
		- num commits
		- repo name
		- PR age
		- author
		- wait
		- viewed

		layout:
		num commits   | org/repo name                 ----  age
		status        | wait       | viewed | changes ---- author
	*/

	topBar := lipgloss.JoinHorizontal(lipgloss.Top,
		commitsCountTag,
		orgRepoTag,
		upperSpacer,
		ageTag,
	)

	bottomBar := lipgloss.JoinHorizontal(lipgloss.Top,
		statusTag,
		waitTag,
		beenViewedTag,
		newChangesTag,
		lowerSpacer,
		authorTag,
	)

	foot.WriteString(lipgloss.NewStyle().Render(topBar) + "\n")
	foot.WriteString(lipgloss.NewStyle().Render(bottomBar) + "\n")

	return foot.String()
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
