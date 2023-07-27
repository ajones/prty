package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/google/go-github/v53/github"
)

type Footer struct{}

func (f *Footer) BuildView(viewWidth int, viewHeight int, statusMsg string, rateInfo *github.Rate, totalPRs int) string {
	doc := strings.Builder{}

	w := lipgloss.Width

	statusTag := tagStyle.Copy().Render("STATUS:")
	gsdTag := tagFusiaStyle.Copy().Render(fmt.Sprintf("PRS [%d]", totalPRs))
	logoTag := tagPurpleStyle.Copy().Align(lipgloss.Right).Render("🎉 PRTY 🎉")

	rateTag := ""
	if rateInfo != nil {
		untilReset := rateInfo.Reset.Sub(time.Now())
		rateMsg := fmt.Sprintf("Rem:%d Rst:%smin", rateInfo.Remaining, formatDurationToMin(untilReset))
		rateTag = tagBlueStyle.Copy().Align(lipgloss.Right).Render(rateMsg)
	}

	statusMesageTag := lipgloss.NewStyle().
		Width(viewWidth - w(statusTag) - w(gsdTag) - w(logoTag) - w(rateTag)).
		Render(statusMsg)

	// status | status message | ----- rateTag | pr count | logo

	bar := lipgloss.JoinHorizontal(lipgloss.Top,
		statusTag,
		statusMesageTag,
		rateTag,
		gsdTag,
		logoTag,
	)

	doc.WriteString(lipgloss.NewStyle().Width(viewWidth).Render(bar))

	return lipgloss.NewStyle().Background(darkerGrey).Height(viewHeight).Render(doc.String())
}
