package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/go-go-golems/glazed/pkg/cli"
	"github.com/go-go-golems/glazed/pkg/cmds"
	"github.com/go-go-golems/glazed/pkg/cmds/fields"
	"github.com/go-go-golems/glazed/pkg/cmds/schema"
	"github.com/go-go-golems/glazed/pkg/cmds/values"
	"github.com/go-go-golems/glazed/pkg/middlewares"
	"github.com/go-go-golems/glazed/pkg/settings"

	"github.com/wesen/listen-killer/pkg/listener"
	apptui "github.com/wesen/listen-killer/pkg/tui"
)

const defaultListMarkdownTemplate = `# Listening TCP Servers

{{- if not .rows }}
No TCP listeners found.
{{- end }}
{{- range $i, $p := .rows }}

## {{ add $i 1 }}.{{ if .name }} {{ .name }}{{ else }} Process{{ end }}{{ if .pid }} (PID {{ .pid }}){{ end }}
{{- if .port }}

- **Listen:** {{ if .address }}{{ .address }}:{{ end }}{{ .port }}{{ if .protocol }} ({{ .protocol }}){{ end }}
{{- end }}
{{- if .username }}
- **User:** {{ .username }}
{{- end }}
{{- if .uptime }}
- **Uptime:** {{ .uptime }}
{{- end }}
{{- if .rss_human }}
- **Memory:** {{ .rss_human }}
{{- end }}
{{- if .cpu_percent }}
- **CPU:** {{ .cpu_percent }}%
{{- end }}
{{- if .exe }}
- **Executable:** {{ .exe }}
{{- end }}
{{- if .cwd }}
- **Started from:** {{ .cwd }}
{{- end }}
{{- if .cmdline }}
- **Command:** {{ .cmdline }}
{{- end }}
{{- end }}
`

// ListCommand is a Glazed command that always emits structured listener data.
type ListCommand struct {
	*cmds.CommandDescription
}

// ListSettings maps CLI flags to a struct.
type ListSettings struct {
	PIDs      []string `glazed:"pid"`
	Ports     []string `glazed:"port"`
	Name      string   `glazed:"name"`
	User      string   `glazed:"user"`
	Path      string   `glazed:"path"`
	CPUSample string   `glazed:"cpu-sample"`
}

// NewListCommand creates the Glazed command with field definitions and sections.
func NewListCommand() (*ListCommand, error) {
	glazedSection, err := settings.NewGlazedSchema(
		settings.WithOutputSectionOptions(
			schema.WithDefaults(map[string]interface{}{
				"output":        "template",
				"template-file": defaultListMarkdownTemplate,
			}),
		),
	)
	if err != nil {
		return nil, err
	}

	cmdSection, err := cli.NewCommandSettingsSection()
	if err != nil {
		return nil, err
	}

	cmdDesc := cmds.NewCommandDescription(
		"list",
		cmds.WithShort("List TCP listening daemons"),
		cmds.WithLong(`
Show all TCP listening sockets on the system with process details.

Emits structured Glazed rows formatted as readable Markdown by default: one
section per listening process. Use --output json/yaml/csv/table for scripts,
CI diagnostics, and LLM-agent workflows.

For the interactive dashboard, run: listen-killer tui.

Examples:
  # Markdown output (default)
  listen-killer list

  # JSON for scripting / LLM-agent workflows
  listen-killer list --output json

  # Show selected fields only
  listen-killer list --fields pid,name,port,cwd,cmdline

  # Find one port or process family
  listen-killer list --port 3000 --output json
  listen-killer list --name node --fields pid,port,cwd,cmdline

  # Interactive TUI dashboard
  listen-killer tui
`),
		cmds.WithFlags(
			fields.New(
				"pid",
				fields.TypeStringList,
				fields.WithDefault([]string{}),
				fields.WithHelp("Only show listeners owned by these PIDs (repeatable or comma-separated)"),
			),
			fields.New(
				"port",
				fields.TypeStringList,
				fields.WithDefault([]string{}),
				fields.WithHelp("Only show these listening ports (repeatable or comma-separated)"),
			),
			fields.New(
				"name",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Only show listeners whose process name contains this string"),
			),
			fields.New(
				"user",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Only show listeners owned by this user substring"),
			),
			fields.New(
				"path",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Only show listeners whose executable, cwd, or cmdline contains this string"),
			),
			fields.New(
				"cpu-sample",
				fields.TypeString,
				fields.WithDefault(""),
				fields.WithHelp("Sample CPU over this duration (e.g. 500ms) so cpu_percent is populated; empty means no sample"),
			),
		),
		cmds.WithSections(glazedSection, cmdSection),
	)

	return &ListCommand{CommandDescription: cmdDesc}, nil
}

var _ cmds.GlazeCommand = &ListCommand{}

// RunIntoGlazeProcessor implements the GlazeCommand interface.
func (c *ListCommand) RunIntoGlazeProcessor(
	ctx context.Context,
	vals *values.Values,
	gp middlewares.Processor,
) error {
	settings := &ListSettings{}
	if err := vals.DecodeSectionInto(schema.DefaultSlug, settings); err != nil {
		return err
	}

	return runCLI(ctx, gp, settings)
}

// RunTUI launches the Bubbletea TUI.
func RunTUI() error {
	m := apptui.NewModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// runCLI scans listeners, applies script-friendly filters, and emits them
// through the Glazed processor.
func runCLI(ctx context.Context, gp middlewares.Processor, settings *ListSettings) error {
	scanner := listener.NewScanner()

	var listeners []listener.ListenerInfo
	var err error
	if settings.CPUSample != "" {
		interval, parseErr := time.ParseDuration(settings.CPUSample)
		if parseErr != nil {
			return fmt.Errorf("invalid --cpu-sample %q: %w", settings.CPUSample, parseErr)
		}
		listeners, err = scanner.ScanWithSample(interval)
	} else {
		listeners, err = scanner.Scan()
	}
	if err != nil {
		return err
	}

	pidFilter, err := parseInt32List(settings.PIDs, "pid")
	if err != nil {
		return err
	}
	portFilter, err := parsePortList(settings.Ports, "port")
	if err != nil {
		return err
	}

	for _, l := range listeners {
		if !matchesListFilters(l, pidFilter, portFilter, settings) {
			continue
		}
		if err := gp.AddRow(ctx, listenerRow(l)); err != nil {
			return err
		}
	}

	return nil
}

func matchesListFilters(l listener.ListenerInfo, pids map[int32]bool, ports map[uint32]bool, settings *ListSettings) bool {
	if len(pids) > 0 && !pids[l.PID] {
		return false
	}
	if len(ports) > 0 && !ports[l.Port] {
		return false
	}
	if settings.Name != "" && !containsFold(l.Name, settings.Name) {
		return false
	}
	if settings.User != "" && !containsFold(l.Username, settings.User) {
		return false
	}
	if settings.Path != "" && !containsFold(l.Exe, settings.Path) && !containsFold(l.Cwd, settings.Path) && !containsFold(l.Cmdline, settings.Path) {
		return false
	}
	return true
}
