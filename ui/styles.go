package ui

import "github.com/charmbracelet/lipgloss"

var (
	// General.

	subtle    = lipgloss.AdaptiveColor{Light: "#D9DCCF", Dark: "#383838"}
	highlight = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	special   = lipgloss.AdaptiveColor{Light: "#43BF6D", Dark: "#73F59F"}

	pink       = lipgloss.Color("#FFACFC")
	fusia      = lipgloss.Color("#F148FB")
	blue       = lipgloss.Color("#7122FA")
	purple     = lipgloss.Color("#560A86")
	green      = lipgloss.Color("#80ED99")
	red        = lipgloss.Color("#FF5F87")
	white      = lipgloss.Color("#FAFAFA")
	black      = lipgloss.Color("#000000")
	lightGrey  = lipgloss.Color("#383838")
	grey       = lipgloss.Color("#303030")
	darkGrey   = lipgloss.Color("#282828")
	darkerGrey = lipgloss.Color("#111111")

	divider = lipgloss.NewStyle().
		SetString("•").
		Padding(0, 1).
		Foreground(subtle).
		String()

	url = lipgloss.NewStyle().Foreground(special).Render

	// Tabs.

	activeTabBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      " ",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "┘",
		BottomRight: "└",
	}

	tabBorder = lipgloss.Border{
		Top:         "─",
		Bottom:      "─",
		Left:        "│",
		Right:       "│",
		TopLeft:     "╭",
		TopRight:    "╮",
		BottomLeft:  "┴",
		BottomRight: "┴",
	}

	tab = lipgloss.NewStyle().
		Border(tabBorder, true).
		BorderForeground(highlight).
		Padding(0, 1)

	activeTab = tab.Copy().Border(activeTabBorder, true)

	tabGap = tab.Copy().
		BorderTop(false).
		BorderLeft(false).
		BorderRight(false)

	// Title.

	titleStyle = lipgloss.NewStyle().
			MarginLeft(1).
			MarginRight(5).
			Padding(0, 1).
			Italic(true)
		//Foreground(lipgloss.Color("#FFF7DB"))

	descStyle = lipgloss.NewStyle().MarginTop(1)

	infoStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderTop(true).
			BorderForeground(subtle)

	// List.

	list = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, false).
		BorderForeground(subtle).
		MarginRight(2).
		Height(8).
		Width(3 + 1)

	listHeader = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(subtle).
			MarginRight(2).
			Render

	listItem = lipgloss.NewStyle().Padding(0, 1).Render

	checkMark = lipgloss.NewStyle().SetString("✓").
			Foreground(special).
			PaddingRight(1).
			String()

	listDone = func(s string) string {
		return checkMark + lipgloss.NewStyle().
			Strikethrough(true).
			Foreground(lipgloss.AdaptiveColor{Light: "#969B86", Dark: "#696969"}).
			Render(s)
	}

	// Pull Request
	prStyle = lipgloss.NewStyle().
		Background(lightGrey)

	prTagStyle = prStyle.Copy().
			Background(darkGrey).
			Width(20).
			Padding(0, 1)

	prTagLeftStyle = prTagStyle.Copy().
			Inherit(prTagStyle).
			Padding(0, 1)

	prTagRightStyle = prTagStyle.Copy().
			Inherit(prTagStyle).
			Padding(0, 1).
			Align(lipgloss.Right)

	pullPositionStyle = lipgloss.NewStyle().
				Align(lipgloss.Right).
				Padding(0, 1)

	pullListStyle = lipgloss.NewStyle().
			Align(lipgloss.Left).
			Background(lightGrey).
			Padding(1, 1, 1, 1)

	pullListStyleSelected = pullListStyle.Copy().
				Padding(1, 1, 1, 1).
				Foreground(white).
				Background(purple)

	historyStyle = lipgloss.NewStyle().
			Align(lipgloss.Left).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(highlight).
			Margin(1, 3, 0, 0).
			Padding(1, 2).
			Height(19).
			Width(30)

	// Tag
	tagStyle = lipgloss.NewStyle().
			Padding(0, 1).
			Align(lipgloss.Center)

	tagAlertStyle = tagStyle.Copy().
			Foreground(white).
			Background(red)

	tagSuccessStyle = tagStyle.Copy().
			Foreground(lightGrey).
			Background(green)

	tagSpecialStyle = tagStyle.Copy().
			Foreground(white).
			Background(purple)

	// Page.

	docStyle = lipgloss.NewStyle().Padding(1, 2, 1, 2)
)
