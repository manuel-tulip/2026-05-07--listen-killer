package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/wesen/listen-killer/pkg/listener"
)

// ---------------------------------------------------------------------------
// Custom messages
// ---------------------------------------------------------------------------

type ListenersLoadedMsg struct {
	Listeners []listener.ListenerInfo
	Err       error
}

type KillResultMsg struct {
	PID    int32
	Signal string
	Err    error
}

type RefreshTickMsg time.Time

type BrowserOpenedMsg struct {
	URL string
	Err error
}

// ---------------------------------------------------------------------------
// View modes
// ---------------------------------------------------------------------------

type viewMode int

const (
	modeTable viewMode = iota
	modeKill
)

// ---------------------------------------------------------------------------
// Model
// ---------------------------------------------------------------------------

type Model struct {
	listeners []listener.ListenerInfo

	table  table.Model
	help   help.Model
	keys   KeyMap
	styles Styles

	width   int
	height  int
	ready   bool
	loading bool
	mode    viewMode

	// scanner retains CPU samples across refreshes so cpu_percent is a real
	// rate from the second scan onward.
	scanner *listener.Scanner

	// Multi-mark: set of marked PIDs
	marked map[int32]bool

	// Detail pane: toggled with 'd', shows the row under the cursor
	showDetail bool

	// Kill dialog state
	killPIDs    []int32
	killNames   []string
	killSignal  string
	killSignals []string
	killIdx     int

	// Auto-refresh
	autoRefresh    bool
	refreshSeconds int

	// Status message
	statusMsg string
}

// NewModel creates a new Model in loading state.
func NewModel() Model {
	columns := []table.Column{
		{Title: "●", Width: 1},
		{Title: "PID", Width: 7},
		{Title: "Process", Width: 16},
		{Title: "User", Width: 8},
		{Title: "Port", Width: 6},
		{Title: "Address", Width: 14},
		{Title: "Uptime", Width: 8},
		{Title: "CPU%", Width: 6},
		{Title: "Memory", Width: 9},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(20),
		table.WithRows([]table.Row{}),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(true)
	t.SetStyles(s)

	return Model{
		table:          t,
		help:           help.New(),
		keys:           NewKeyMap(),
		styles:         DefaultStyles(),
		loading:        true,
		mode:           modeTable,
		scanner:        listener.NewScanner(),
		marked:         make(map[int32]bool),
		killSignal:     "TERM",
		killSignals:    []string{"TERM", "KILL", "INT"},
		killIdx:        0,
		autoRefresh:    false,
		refreshSeconds: 3,
		showDetail:     true,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadListeners(), tea.EnterAltScreen)
}

// ---------------------------------------------------------------------------
// Row conversion — includes mark column (●)
// ---------------------------------------------------------------------------

func (m Model) listenersToRows() []table.Row {
	rows := make([]table.Row, len(m.listeners))
	for i, l := range m.listeners {
		addr := l.Address
		if addr == "" || addr == "::" {
			addr = "*"
		}
		cpu := ""
		if l.CPUSampled {
			cpu = fmt.Sprintf("%.1f", l.CPUPercent)
		}
		mark := " "
		if m.marked[l.PID] {
			mark = "●"
		}
		rows[i] = table.Row{
			mark,
			fmt.Sprintf("%d", l.PID),
			l.Name,
			l.Username,
			fmt.Sprintf("%d", l.Port),
			addr,
			l.Uptime,
			cpu,
			l.RSSHuman,
		}
	}
	return rows
}

// ---------------------------------------------------------------------------
// Get ListenerInfo for the currently selected table row
// ---------------------------------------------------------------------------

func (m Model) selectedListener() *listener.ListenerInfo {
	row := m.table.SelectedRow()
	if len(row) < 2 {
		return nil
	}
	var pid int32
	fmt.Sscanf(row[1], "%d", &pid)
	for _, l := range m.listeners {
		if l.PID == pid {
			info := l
			return &info
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Build a browser URL from a ListenerInfo
// ---------------------------------------------------------------------------

func listenerURL(l *listener.ListenerInfo) string {
	host := l.Address
	if host == "" || host == "*" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	// IPv6 hosts must be bracketed in a URL; a bare "http://::1:5199" is
	// invalid. Wildcards were already rewritten to IPv4 above.
	if strings.Contains(host, ":") {
		return fmt.Sprintf("http://[%s]:%d", host, l.Port)
	}
	return fmt.Sprintf("http://%s:%d", host, l.Port)
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

func (m Model) loadListeners() tea.Cmd {
	return func() tea.Msg {
		listeners, err := m.scanner.Scan()
		return ListenersLoadedMsg{Listeners: listeners, Err: err}
	}
}

func (m Model) killOne(pid int32, signal string) tea.Cmd {
	return func() tea.Msg {
		err := listener.KillProcess(pid, signal)
		return KillResultMsg{PID: pid, Signal: signal, Err: err}
	}
}

func killMarked(pids []int32, signal string) tea.Cmd {
	cmds := make([]tea.Cmd, len(pids))
	for i, pid := range pids {
		p := pid // capture
		cmds[i] = func() tea.Msg {
			err := listener.KillProcess(p, signal)
			return KillResultMsg{PID: p, Signal: signal, Err: err}
		}
	}
	return tea.Batch(cmds...)
}

func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "linux":
			cmd = exec.Command("xdg-open", url)
		case "darwin":
			cmd = exec.Command("open", url)
		default:
			return BrowserOpenedMsg{URL: url, Err: fmt.Errorf("unsupported OS: %s", runtime.GOOS)}
		}
		err := cmd.Start()
		if err != nil {
			return BrowserOpenedMsg{URL: url, Err: err}
		}
		return BrowserOpenedMsg{URL: url, Err: nil}
	}
}

func autoRefreshTick(seconds int) tea.Cmd {
	return tea.Tick(time.Duration(seconds)*time.Second, func(t time.Time) tea.Msg {
		return RefreshTickMsg(t)
	})
}
