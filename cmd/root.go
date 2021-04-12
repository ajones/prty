package cmd

import (
	"fmt"
	"os"
	"strings"
	"time"

	//"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/joho/godotenv"
	"golang.org/x/term"

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
	prView *ui.PriorityPRs

	statusChan    chan string
	statusMessage string

	ds *datasource.Datasource
}

var initialModel = model{
	// Our to-do list is just a grocery list
	choices: []string{"Buy carrots", "Buy celery", "Buy kohlrabi"},

	// A map which indicates which choices are selected. We're using
	// the  map like a mathematical set. The keys refer to the indexes
	// of the `choices` slice, above.
	selected: make(map[int]struct{}),

	selectedTabIndex: 0,
	tabNames:         []string{"Needs Attention", "Team", "Open", "All"},

	nav:    &ui.TabNav{},
	prView: &ui.PriorityPRs{},

	statusChan:    make(chan string),
	statusMessage: "",

	ds: datasource.New(),
}

func (m *model) Init() tea.Cmd {
	m.ds.SetStatusChan(m.statusChan)

	m.statusMessage = "init..."

	go m.ds.RefreshData()
	go m.listenForStatusChanges(m.statusChan)

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

		// The "up" and "k" keys move the cursor up
		case "up", "k":
			m.prView.OnCursorMove(0, -1)

		// The "down" and "j" keys move the cursor down
		case "down", "j":
			m.prView.OnCursorMove(0, 1)

		case "left":
			m.cursor.Y = 0
			m.cursor.X--

		case "right":
			m.cursor.Y = 0
			m.cursor.X++

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
	switch m.cursor.X {
	case 0:
		//pulls, _ := datasource.GetAllPulls("hellodigit", "digit-libs")
		m.prView.OnSelect(m.cursor)
	}
}

func (m *model) View() string {
	// todo moe to async update handler
	pulls, _ := datasource.GetAllPulls()
	m.prView.OnPullsUpdate(pulls)

	width, height, _ := term.GetSize(int(os.Stdout.Fd()))

	navHeight := 3
	footerHeight := 1
	bodyHeight := height - navHeight - footerHeight

	renderedPage := strings.Builder{}

	renderedPage.WriteString(m.nav.BuildView(width, navHeight, m.tabNames, m.cursor.X))

	switch m.cursor.X {
	case 0:
		renderedPage.WriteString(m.prView.BuildView(width, bodyHeight))
	default:
		renderedPage.WriteString(lipgloss.NewStyle().Height(bodyHeight).Render("\n"))
	}

	footer := ui.Footer{}
	renderedPage.WriteString(footer.BuildView(width, footerHeight, m.statusMessage))
	return renderedPage.String()
}

func (m *model) listenForStatusChanges(c chan string) {
	for {
		m.statusMessage = <-c
	}
}

func Execute() {
	err := godotenv.Load()
	if err != nil {
		fmt.Println("Error loading .env file")
	}

	//datasource.GithubGet()
	/*
		abandonAgeDays := os.Getenv("ABANDONED_AGE_DAYS")
		if days, err := strconv.Atoi(abandonAgeDays); err == nil {
			fmt.Printf("%d\n", days)
			then := time.Now().Add(time.Duration(30) * time.Hour * time.Duration(24))
			fmt.Printf("then %s\n", then)

			abd := time.Now().After(pr.LastCommitTime.Add(time.Duration(days) * time.Hour * time.Duration(24)))
		}
	*/

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
