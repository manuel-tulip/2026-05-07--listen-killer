package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/wesen/listen-killer/pkg/listener"
)

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetWidth(m.width - 2)
		detailLines := 0
		if m.showDetail {
			detailLines = 12
		}
		m.table.SetHeight(m.height - 5 - detailLines)
		if !m.ready {
			m.ready = true
		}
		return m, nil

	case tea.KeyMsg:
		key := msg.String()

		if key == "ctrl+c" {
			return m, tea.Quit
		}

		if key == "?" {
			m.help.ShowAll = !m.help.ShowAll
			return m, nil
		}

		switch m.mode {
		case modeTable:
			var cmd tea.Cmd
			m, handled, cmd := m.handleTableKey(key)
			if handled {
				return m, cmd
			}
			var tCmd tea.Cmd
			m.table, tCmd = m.table.Update(msg)
			return m, tCmd

		case modeKill:
			return m.updateKill(key)
		}

		return m, nil

	case ListenersLoadedMsg:
		m.loading = false
		if msg.Err != nil {
			m.statusMsg = fmt.Sprintf("Error: %v", msg.Err)
		} else {
			m.listeners = msg.Listeners
			// Prune marks for PIDs that no longer exist
		alive:
			for pid := range m.marked {
				for _, l := range m.listeners {
					if l.PID == pid {
						continue alive
					}
				}
				delete(m.marked, pid)
			}
			m.table.SetRows(m.listenersToRows())
			n := len(m.listeners)
			marked := len(m.marked)
			if marked > 0 {
				m.statusMsg = fmt.Sprintf("%d listeners, %d marked", n, marked)
			} else {
				m.statusMsg = fmt.Sprintf("%d listeners", n)
			}
		}
		return m, nil

	case KillResultMsg:
		m.mode = modeTable
		if msg.Err != nil {
			m.statusMsg = fmt.Sprintf("PID %d: %v", msg.PID, msg.Err)
		} else {
			delete(m.marked, msg.PID)
			m.statusMsg = fmt.Sprintf("Sent %s → PID %d", msg.Signal, msg.PID)
			return m, m.loadListeners()
		}
		return m, nil

	case BrowserOpenedMsg:
		if msg.Err != nil {
			m.statusMsg = fmt.Sprintf("Browser: %v", msg.Err)
		} else {
			m.statusMsg = fmt.Sprintf("Opened %s", msg.URL)
		}
		return m, nil

	case RefreshTickMsg:
		if m.autoRefresh && m.mode == modeTable {
			return m, tea.Batch(m.loadListeners(), autoRefreshTick(m.refreshSeconds))
		}
		return m, nil
	}

	var tCmd tea.Cmd
	m.table, tCmd = m.table.Update(msg)
	return m, tCmd
}

// ---------------------------------------------------------------------------
// Table mode
// ---------------------------------------------------------------------------

func (m Model) handleTableKey(key string) (Model, bool, tea.Cmd) {
	switch key {

	case "q":
		return m, true, tea.Quit

	// --- Mark / unmark current row ---
	case " ":
		info := m.selectedListener()
		if info == nil {
			return m, true, nil
		}
		if m.marked[info.PID] {
			delete(m.marked, info.PID)
		} else {
			m.marked[info.PID] = true
		}
		// Refresh the table to show the ● marker
		m.table.SetRows(m.listenersToRows())
		// Move cursor down one so user can rapid-mark
		var tCmd tea.Cmd
		m.table, tCmd = m.table.Update(tea.KeyMsg{Type: tea.KeyDown})
		return m, true, tCmd

	// --- Clear all marks ---
	case "M":
		m.marked = make(map[int32]bool)
		m.table.SetRows(m.listenersToRows())
		m.statusMsg = "Marks cleared"
		return m, true, nil

	// --- Kill: marked PIDs or single PID ---
	case "K":
		// If marks exist, kill all marked. Otherwise kill selected.
		if len(m.marked) > 0 {
			pids := markedPIDs(m.marked)
			names := make([]string, len(pids))
			for i, pid := range pids {
				names[i] = pidName(m.listeners, pid)
			}
			m.killPIDs = pids
			m.killNames = names
		} else {
			info := m.selectedListener()
			if info == nil {
				m.statusMsg = "No process selected"
				return m, true, nil
			}
			m.killPIDs = []int32{info.PID}
			m.killNames = []string{info.Name}
		}
		m.killSignal = m.killSignals[0]
		m.killIdx = 0
		m.mode = modeKill
		return m, true, nil

	// --- Toggle detail pane ---
	case "d":
		m.showDetail = !m.showDetail
		if m.showDetail {
			m.table.SetHeight(m.height - 5 - 12)
		} else {
			m.table.SetHeight(m.height - 5)
		}
		m.statusMsg = "Detail pane: "
		if m.showDetail {
			m.statusMsg += "on"
		} else {
			m.statusMsg += "off"
		}
		return m, true, nil

	// --- Open browser ---
	case "o":
		info := m.selectedListener()
		if info == nil {
			return m, true, nil
		}
		url := listenerURL(info)
		m.statusMsg = fmt.Sprintf("Opening %s ...", url)
		return m, true, openBrowserCmd(url)

	// --- Refresh ---
	case "r":
		m.loading = true
		m.statusMsg = "Refreshing..."
		return m, true, m.loadListeners()

	// --- Auto-refresh ---
	case "a":
		m.autoRefresh = !m.autoRefresh
		if m.autoRefresh {
			m.statusMsg = fmt.Sprintf("Auto-refresh: %ds", m.refreshSeconds)
			return m, true, tea.Batch(autoRefreshTick(m.refreshSeconds))
		}
		m.statusMsg = "Auto-refresh off"
		return m, true, nil

	case "/":
		m.statusMsg = "Filter: coming soon"
		return m, true, nil
	}

	return m, false, nil
}

// ---------------------------------------------------------------------------
// Kill confirmation
// ---------------------------------------------------------------------------

func (m Model) updateKill(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.mode = modeTable
		return m, nil

	case "y", "Y", "enter":
		m.mode = modeTable
		signal := m.killSignal
		if len(m.killPIDs) == 1 {
			m.statusMsg = fmt.Sprintf("Killing PID %d with %s...", m.killPIDs[0], signal)
			return m, m.killOne(m.killPIDs[0], signal)
		}
		m.statusMsg = fmt.Sprintf("Killing %d processes with %s...", len(m.killPIDs), signal)
		// Clear marks for the PIDs we're about to kill
		for _, pid := range m.killPIDs {
			delete(m.marked, pid)
		}
		return m, killMarked(m.killPIDs, signal)

	case "n", "N":
		m.mode = modeTable
		m.statusMsg = "Kill cancelled"
		return m, nil

	case "up":
		if m.killIdx > 0 {
			m.killIdx--
			m.killSignal = m.killSignals[m.killIdx]
		}
		return m, nil

	case "down":
		if m.killIdx < len(m.killSignals)-1 {
			m.killIdx++
			m.killSignal = m.killSignals[m.killIdx]
		}
		return m, nil
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func markedPIDs(m map[int32]bool) []int32 {
	pids := make([]int32, 0, len(m))
	for pid := range m {
		pids = append(pids, pid)
	}
	sort.Slice(pids, func(i, j int) bool { return pids[i] < pids[j] })
	return pids
}

func pidName(listeners []listener.ListenerInfo, pid int32) string {
	for _, l := range listeners {
		if l.PID == pid {
			return l.Name
		}
	}
	return fmt.Sprintf("PID-%d", pid)
}

// ---------------------------------------------------------------------------
// Footer
// ---------------------------------------------------------------------------

func (m Model) renderFooter() string {
	status := m.styles.StatusOK.Render(m.statusMsg)
	if strings.HasPrefix(m.statusMsg, "Error") || strings.HasPrefix(m.statusMsg, "Failed") || strings.HasPrefix(m.statusMsg, "PID") {
		status = m.styles.StatusDanger.Render(m.statusMsg)
	}

	helpView := m.help.View(m.keys)

	avail := m.width - lipgloss.Width(status) - lipgloss.Width(helpView) - 4
	if avail < 0 {
		avail = 0
	}
	spacer := lipgloss.NewStyle().Width(avail).Render("")
	footer := lipgloss.JoinHorizontal(lipgloss.Left, status, spacer, helpView)

	fw := m.width - 2
	if fw < 10 {
		fw = 10
	}
	return m.styles.Footer.Width(fw).Render(footer)
}
