package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	//"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/go-github/v34/github"
	"github.com/joho/godotenv"
	"golang.org/x/term"

	"github.com/inburst/prty/config"
	"github.com/inburst/prty/datasource"
	"github.com/inburst/prty/ui"
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

	nav    *ui.TabNav
	views  []ui.PRViewData
	footer *ui.Footer

	statusChan            chan string
	statusMessage         string
	remainingRequestsChan chan github.Rate
	currentRateInfo       *github.Rate

	ds           *datasource.Datasource
	prUpdateChan chan *datasource.PullRequest
}

var initialModel = model{
	// Our to-do list is just a grocery list
	choices: []string{"Buy carrots", "Buy celery", "Buy kohlrabi"},

	// A map which indicates which choices are selected. We're using
	// the  map like a mathematical set. The keys refer to the indexes
	// of the `choices` slice, above.
	selected: make(map[int]struct{}),

	selectedTabIndex: 0,
	tabNames:         []string{"Needs Attention", "Team", "Active", "All"},

	nav: &ui.TabNav{},
	views: []ui.PRViewData{
		&ui.PriorityPRs{},
		&ui.TeamPrs{},
		&ui.ActivePRs{},
	},
	footer: &ui.Footer{},

	statusChan:            make(chan string),
	statusMessage:         "",
	remainingRequestsChan: make(chan github.Rate),

	prUpdateChan: make(chan *datasource.PullRequest),
}

func (m *model) Init() tea.Cmd {
	c, err := config.LoadConfig()
	if err != nil {
		println(fmt.Sprintf("%s", err))
		os.Exit(1)
		return tick()
	}

	m.ds = datasource.New(c)
	m.ds.SetStatusChan(m.statusChan)
	m.ds.SetRemainingRequestsChan(m.remainingRequestsChan)
	m.ds.SetPRUpdateChan(m.prUpdateChan)

	go m.listenForStatusChanges()
	go m.listenForPRChanges()
	go m.listenForRemainingRequests()
	go m.ds.RefreshData()

	m.ds.LoadLocalCache()

	m.statusMessage = "init..."
	return tick()
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {

	switch msg := msg.(type) {
	case tickMsg:
		return m, tick()

	// Is it a key press?
	case tea.KeyMsg:

		// Cool, what was the actual key pressed?
		switch msg.String() {

		// These keys should exit the program.
		case "ctrl+c", "q":
			return m, tea.Quit

		case "r":
			m.refreshData()
			v := m.views[m.cursor.X]
			v.Clear()

		case "s":
			v := m.views[m.cursor.X]
			println(fmt.Sprintf("%d %v", m.cursor.X, v))
			v.OnSort()

		// The "up" and "k" keys move the cursor up
		case "up", "k":
			v := m.views[m.cursor.X]
			v.OnCursorMove(0, -1)

		// The "down" and "j" keys move the cursor down
		case "down", "j":
			v := m.views[m.cursor.X]
			v.OnCursorMove(0, 1)

		case "left":
			m.cursor.Y = 0
			m.cursor.X--
			if m.cursor.X < 0 {
				m.cursor.X = 0
			}

		case "right":
			m.cursor.Y = 0
			m.cursor.X++
			if m.cursor.X >= len(m.views) {
				m.cursor.X = len(m.views) - 1
			}

		// The "enter" key and the spacebar (a literal space) toggle
		// the selected state for the item that the cursor is pointing at.
		case "enter", " ":
			m.sendSelectToActiveTab()
		}
	}

	// Return the updated model to the Bubble Tea runtime for processing.
	// Note that we're not returning a command.
	return m, nil
}

func (m *model) sendSelectToActiveTab() {
	v := m.views[m.cursor.X]
	v.OnSelect(m.cursor)
}

func (m *model) refreshData() {
	go m.ds.RefreshData()
}

func (m *model) View() string {
	width, height, _ := term.GetSize(int(os.Stdout.Fd()))

	navHeight := 3
	footerHeight := 1
	bodyHeight := height - navHeight - footerHeight

	renderedPage := strings.Builder{}

	// Tab Nav
	renderedPage.WriteString(m.nav.BuildView(width, navHeight, m.tabNames, m.cursor.X))

	// Body View
	v := m.views[m.cursor.X]
	renderedPage.WriteString(ui.BuildPRView(v, width, bodyHeight))

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

func Execute() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
	}

	p := tea.NewProgram(&initialModel)
	// Use the full size of the terminal in its "alternate screen buffer"
	p.EnterAltScreen()
	defer p.ExitAltScreen()

	if err := p.Start(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

func tick() tea.Cmd {
	return tea.Tick(time.Duration(interval), func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
