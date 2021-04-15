package ui

import (
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/inburst/prty/datasource"
)

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

func replaceLinks(str string) string {
	var re = regexp.MustCompile(`\[.*\](.*)`)
	return re.ReplaceAllString(str, `$1.$2`)
}

func stringWrap(str string, width int) string {
	if len(str) < width {
		return str
	}
	newStr := ""

	strLen := len(str)
	for i := 0; i < strLen; i += width {
		if i+width < strLen {
			newStr += str[i:(i+width)] + "\n"
		} else {
			newStr += str[i:strLen] + "\n"
		}
	}

	return newStr
}

func stripNewLines(str string) string {
	return strings.ReplaceAll(str, "\n", "")
}

func openbrowser(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Fatal(err)
	}

}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Minute)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	return fmt.Sprintf("%02d:%02d", h, m)
}

func formatDurationToMin(d time.Duration) string {
	d = d.Round(time.Minute) / time.Minute
	return fmt.Sprintf("%02d", d)
}

func formatDurationDayHour(d time.Duration) string {
	d = d.Round(time.Minute)

	days := d / (time.Hour * 24)
	hours := (d - (days * (time.Hour * 24))) / time.Hour
	return fmt.Sprintf("%dd:%02d", days, hours)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
