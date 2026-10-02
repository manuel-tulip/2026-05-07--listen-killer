package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ---------------------------------------------------------------------------
// View
// ---------------------------------------------------------------------------

func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	w := m.width - 2
	if w < 20 {
		w = 20
	}

	// Title bar
	title := m.styles.Title.Render("🎧 Listen Killer")
	if m.loading {
		title = m.styles.Title.Render("🎧 Listen Killer  ⏳")
	}
	marked := len(m.marked)
	countText := m.styles.TitleBar.Render(fmt.Sprintf(" %d listeners", len(m.listeners)))
	if marked > 0 {
		countText = m.styles.TitleBar.Render(fmt.Sprintf(" %d listeners  %d marked", len(m.listeners), marked))
	}
	titleBar := lipgloss.NewStyle().Width(w).Render(
		lipgloss.JoinHorizontal(lipgloss.Left, title, countText),
	)

	// Body
	var body string
	if m.mode == modeKill {
		body = m.renderKillOverlay()
	} else if m.showDetail {
		body = m.renderVerticalSplit()
	} else {
		body = m.renderTable()
	}

	footer := m.renderFooter()

	return lipgloss.JoinVertical(lipgloss.Left, titleBar, body, footer)
}

// ---------------------------------------------------------------------------
// Table only (no detail pane)
// ---------------------------------------------------------------------------

func (m Model) renderTable() string {
	if m.loading && len(m.listeners) == 0 {
		return m.styles.Loading.Render("\n  Scanning for TCP listeners...")
	}
	if len(m.listeners) == 0 {
		return m.styles.Loading.Render("\n  No TCP listeners found.")
	}
	return m.table.View()
}

// ---------------------------------------------------------------------------
// Vertical split: table on top, detail pane below
// ---------------------------------------------------------------------------

func (m Model) renderVerticalSplit() string {
	tableView := m.renderTable()
	detailView := m.renderDetailPane()
	separator := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(strings.Repeat("─", m.width-2))
	return lipgloss.JoinVertical(lipgloss.Left, tableView, separator, detailView)
}

// ---------------------------------------------------------------------------
// Detail pane (below the table — full width, compact layout)
// ---------------------------------------------------------------------------

func (m Model) renderDetailPane() string {
	info := m.selectedListener()
	if info == nil {
		return lipgloss.NewStyle().
			Height(10).
			Foreground(lipgloss.Color("243")).
			Render("  No selection")
	}

	addr := info.Address
	if addr == "" || addr == "::" {
		addr = "*"
	}

	cmdline := info.Cmdline
	if len(cmdline) > 80 {
		cmdline = cmdline[:77] + "..."
	}

	exe := info.Exe
	if len(exe) > 80 {
		exe = exe[:77] + "..."
	}

	cwd := info.Cwd
	if len(cwd) > 80 {
		cwd = cwd[:77] + "..."
	}

	url := listenerURL(info)

	label := m.styles.DetailLabel
	value := m.styles.DetailValue

	// Row 1: core identity
	line1 := lipgloss.JoinHorizontal(lipgloss.Left,
		label.Render("PID"), value.Render(fmt.Sprintf("%-8d", info.PID)),
		label.Render("Name"), value.Render(fmt.Sprintf("%-20s", info.Name)),
		label.Render("User"), value.Render(fmt.Sprintf("%-10s", info.Username)),
		label.Render("Uptime"), value.Render(info.Uptime),
	)

	// Row 2: network
	line2 := lipgloss.JoinHorizontal(lipgloss.Left,
		label.Render("Port"), value.Render(fmt.Sprintf("%-8d", info.Port)),
		label.Render("Address"), value.Render(fmt.Sprintf("%-18s", addr)),
		label.Render("CPU"), value.Render(fmt.Sprintf("%-8s", fmt.Sprintf("%.1f%%", info.CPUPercent))),
		label.Render("Memory"), value.Render(info.RSSHuman),
	)

	// Row 3: binary
	line3 := lipgloss.JoinHorizontal(lipgloss.Left,
		label.Render("Binary"), value.Render(exe),
	)

	// Row 4: working directory
	line4 := lipgloss.JoinHorizontal(lipgloss.Left,
		label.Render("Cwd"), value.Render(cwd),
	)

	// Row 5: cmdline
	line5 := lipgloss.JoinHorizontal(lipgloss.Left,
		label.Render("Cmd"), value.Render(cmdline),
	)

	// Row 6: URL + marks
	urlLine := lipgloss.JoinHorizontal(lipgloss.Left,
		label.Render("URL"), value.Render(url),
	)
	if m.marked[info.PID] {
		urlLine = lipgloss.JoinHorizontal(lipgloss.Left,
			urlLine,
			"  ",
			m.styles.StatusDanger.Render("● MARKED"),
		)
	}

	// Header with process title
	header := m.styles.DetailTitle.Render(fmt.Sprintf("%d — %s", info.PID, info.Name))

	lines := []string{header, line1, line2, line3, line4, line5, urlLine}
	// Surface enrichment degradations (cwd/exe denied or unsupported) instead
	// of letting fields silently appear empty.
	if len(info.Warnings) > 0 {
		lines = append(lines, label.Render("Warn")+" "+
			m.styles.StatusDanger.Render(strings.Join(info.Warnings, "  ")))
	}

	detail := lipgloss.NewStyle().
		Width(m.width-4).
		Padding(0, 1).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))

	return detail
}

// ---------------------------------------------------------------------------
// Kill dialog overlay
// ---------------------------------------------------------------------------

func (m Model) renderKillOverlay() string {
	title := m.styles.DialogTitle.Render("⚠ Kill Process")

	var target string
	if len(m.killPIDs) == 1 {
		target = fmt.Sprintf("PID %d — %s", m.killPIDs[0], m.killNames[0])
	} else {
		target = fmt.Sprintf("%d processes:", len(m.killPIDs))
		for i, pid := range m.killPIDs {
			name := m.killNames[i]
			target += fmt.Sprintf("\n  • PID %d — %s", pid, name)
		}
	}

	var signalLines []string
	for i, sig := range m.killSignals {
		prefix := "  "
		style := m.styles.ButtonInactive
		if i == m.killIdx {
			prefix = "▶ "
			style = m.styles.ButtonActive
		}
		desc := ""
		switch sig {
		case "TERM":
			desc = "(graceful)"
		case "KILL":
			desc = "(force)"
		case "INT":
			desc = "(interrupt)"
		}
		signalLines = append(signalLines, style.Render(prefix+sig+" "+desc))
	}

	signalsBlock := strings.Join(signalLines, "\n")
	footer := "\n\n[y] Confirm  •  [n] Cancel  •  [↑↓] Signal"

	dialog := m.styles.Dialog.Render(
		title + "\n\n" +
			m.styles.DialogText.Render(target) + "\n\n" +
			signalsBlock +
			footer,
	)

	w := m.width - 2
	h := m.height - 6
	if w < 20 {
		w = 20
	}
	if h < 10 {
		h = 10
	}

	return lipgloss.Place(w, h,
		lipgloss.Center, lipgloss.Center,
		dialog,
	)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (m Model) renderKV(label, value string) string {
	return m.styles.DetailLabel.Render(label) + " " + m.styles.DetailValue.Render(value)
}
