package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/go-github/v34/github"
	"golang.org/x/term"

	"github.com/inburst/prty/config"
	"github.com/inburst/prty/datasource"
	"github.com/inburst/prty/logger"
	"github.com/inburst/prty/stats"
	"github.com/inburst/prty/ui"
	"github.com/inburst/prty/utils"
)

var (
	duration = time.Second * 10
	interval = time.Second
)

type tickMsg time.Time

type model struct {
	choices []string // items on the to-do list
	//cursor   int              // which to-do list item our cursor is pointing at
	selected map[int]struct{} // which to-do items are selected

	tabNames         []string // names of each tab
	selectedTabIndex int      // which tab is selected
	selectedRowIndex int      // which row is selected

	cursor ui.CursorPos

	nav        *ui.TabNav
	views      []ui.PRViewData
	footer     *ui.Footer
	detailView *ui.PRDetail
	statsView  *ui.Stats

	statusChan            chan string
	statusMessage         string
	remainingRequestsChan chan github.Rate
	currentRateInfo       *github.Rate

	ds           *datasource.Datasource
	prUpdateChan chan *datasource.PullRequest

	stats *stats.Stats
}

var initialModel = model{
	selectedTabIndex: 0,
	tabNames:         []string{"Needs Attention", "Team", "Active", "Bots"},

	nav: &ui.TabNav{},
	views: []ui.PRViewData{
		&ui.PriorityPRs{},
		&ui.TeamPrs{},
		&ui.ActivePRs{},
		&ui.BotsPrs{},
	},
	footer: &ui.Footer{},

	statusChan:            make(chan string),
	statusMessage:         "",
	remainingRequestsChan: make(chan github.Rate),

	prUpdateChan: make(chan *datasource.PullRequest),
}

func (m *model) Init() tea.Cmd {
	// these are pre-validated in checkConfiguration
	c, _ := config.LoadConfig()
	m.stats, _ = stats.LoadStats()
	datasource.InitSharedClient(c.GithubAccessToken)

	m.ds = datasource.New(c)
	m.ds.SetStatusChan(m.statusChan)
	m.ds.SetRemainingRequestsChan(m.remainingRequestsChan)
	m.ds.SetPRUpdateChan(m.prUpdateChan)

	go m.listenForStatusChanges()
	go m.listenForPRChanges()
	go m.listenForRemainingRequests()

	m.ds.LoadLocalCache()

	if c.RefreshOnStart {
		go m.ds.RefreshData()
		m.statusMessage = "init..."
	}

	return tick()
}

func (m *model) IsViewingSecondary() bool {
	return m.detailView != nil || m.statsView != nil
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		return m, tick()

	case tea.KeyMsg:
		switch msg.String() {

		case "ctrl+c", "q":
			return m, tea.Quit

		case "r":
			if m.IsViewingSecondary() {
				break
			}
			m.refreshData()
			for _, v := range m.views {
				v.Clear()
			}

		case "s":
			if m.IsViewingSecondary() {
				break
			}
			v := m.views[m.cursor.X]
			v.OnSort()

		case "z":
			if m.IsViewingSecondary() {
				break
			}
			m.statsView = &ui.Stats{
				UserStats: m.stats,
			}

		case "up", "k":
			if m.IsViewingSecondary() {
				break
			}
			v := m.views[m.cursor.X]
			v.OnCursorMove(0, -1)

		case "down", "j":
			if m.IsViewingSecondary() {
				break
			}
			v := m.views[m.cursor.X]
			v.OnCursorMove(0, 1)

		case "d":
			if m.IsViewingSecondary() {
				break
			}
			v := m.views[m.cursor.X]
			p := v.GetSelectedPull()
			m.detailView = &ui.PRDetail{
				PR: p,
			}

		case "esc":
			m.detailView = nil
			m.statsView = nil

		case "left", "h":
			if m.IsViewingSecondary() {
				break
			}
			m.cursor.Y = 0
			m.cursor.X--
			if m.cursor.X < 0 {
				m.cursor.X = 0
			}

		case "right", "l":
			if m.IsViewingSecondary() {
				break
			}
			m.cursor.Y = 0
			m.cursor.X++
			if m.cursor.X >= len(m.views) {
				m.cursor.X = len(m.views) - 1
			}

		// The "enter" key and the spacebar (a literal space) toggle
		// the selected state for the item that the cursor is pointing at.
		case "o", "enter", " ":
			m.sendSelectToActiveTab()
		}
	}

	// Return the updated model to the Bubble Tea runtime for processing.
	// Note that we're not returning a command.
	return m, nil
}

func tick() tea.Cmd {
	return tea.Tick(time.Duration(interval), func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m *model) sendSelectToActiveTab() {
	v := m.views[m.cursor.X]
	v.OnSelect(m.cursor, m.stats)
	m.ds.SaveToFile()
}

func (m *model) refreshData() {
	go m.ds.RefreshData()
}

func (m *model) View() string {
	width, height, _ := term.GetSize(int(os.Stdout.Fd()))

	heroHeight := 7
	navHeight := 3
	footerHeight := 1
	bodyHeight := height - navHeight - footerHeight - heroHeight
	renderedPage := strings.Builder{}

	// Header
	renderedPage.WriteString(ui.BuildHeader(width, heroHeight))
	// Tab Nav
	renderedPage.WriteString(m.nav.BuildView(width, navHeight, m.tabNames, m.cursor.X))
	// Body View
	if m.detailView != nil {
		renderedPage.WriteString(m.detailView.BuildView(width, bodyHeight))
	} else if m.statsView != nil {
		renderedPage.WriteString(m.statsView.BuildView(width, bodyHeight))
	} else {
		v := m.views[m.cursor.X]
		renderedPage.WriteString(ui.BuildPRView(v, width, bodyHeight))
	}
	// Footer
	renderedPage.WriteString(m.footer.BuildView(width, footerHeight, m.statusMessage, m.currentRateInfo))

	return renderedPage.String()
}

func (m *model) listenForStatusChanges() {
	for {
		m.statusMessage = <-m.statusChan
	}
}

func (m *model) listenForRemainingRequests() {
	for {
		rate := <-m.remainingRequestsChan
		m.currentRateInfo = &rate
	}
}

func (m *model) listenForPRChanges() {
	for {
		newPR := <-m.prUpdateChan
		for _, v := range m.views {
			v.OnNewPullData(newPR)
		}
	}
}

func startUI() {
	p := tea.NewProgram(&initialModel)
	// Use the full size of the terminal in its "alternate screen buffer"
	p.EnterAltScreen()
	defer p.ExitAltScreen()

	if err := p.Start(); err != nil {
		fmt.Printf("Error starting UI : %s", err)
		os.Exit(1)
	}
}

func handleArguments() {
	args := os.Args
	// -v
	if utils.Contains(args, "-v") {
		fmt.Printf("version: %s\n", config.PRTYVersion)
		os.Exit(0)
	}
}

func checkConfiguration() {
	err := logger.InitializeLogger()
	if err != nil {
		fmt.Printf("Error initilizting logger %s\n%s", err)
		os.Exit(1)
	}

	logPath, _ := config.GetLogFilePath()
	moreInformationMessage := fmt.Sprintf("Debug information available at %s\nPlease review the README if you encounter any issues https://github.com/ajones/prty\n", logPath)

	_, err = config.LoadConfig()
	if err != nil {
		fmt.Printf("%s\n%s", err, moreInformationMessage)
		os.Exit(1)
	}

	_, err = stats.LoadStats()
	if err != nil {
		fmt.Printf("%s\n%s", err, moreInformationMessage)
		os.Exit(1)
	}

}

func Execute() {
	checkConfiguration()
	handleArguments()
	startUI()
}
