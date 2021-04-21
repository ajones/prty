package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/google/go-github/v34/github"
)

type Footer struct{}

func (f *Footer) BuildView(viewWidth int, viewHeight int, statusMsg string, rateInfo *github.Rate) string {
	doc := strings.Builder{}

	w := lipgloss.Width

	statusTag := tagStyle.Copy().Render("STATUS:")
	gsdTag := tagSuccessStyle.Copy().Render("GSD")
	logoTag := tagSpecialStyle.Copy().Align(lipgloss.Right).Render("🎉 PRTY 🎉")

	rateMsg := ""
	if rateInfo != nil {
		untilReset := rateInfo.Reset.Sub(time.Now())
		rateMsg = fmt.Sprintf("Rem:%d Rst:%smin", rateInfo.Remaining, formatDurationToMin(untilReset))
	}
	rateTag := tagSpecialStyle.Copy().Align(lipgloss.Right).Render(rateMsg)

	statusMesageTag := lipgloss.NewStyle().
		Width(viewWidth - w(statusTag) - w(gsdTag) - w(logoTag) - w(rateTag)).
		Render(statusMsg)

	// status | status message | ----- rateTag | gsd | logo

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
