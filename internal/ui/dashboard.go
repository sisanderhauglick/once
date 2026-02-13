package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	zone "github.com/lrstanley/bubblezone/v2"

	"github.com/basecamp/once/internal/docker"
	"github.com/basecamp/once/internal/metrics"
)

const (
	PanelHeight = 8
	PanelGap    = 1
)

type dashboardKeyMap struct {
	Up        key.Binding
	Down      key.Binding
	Settings  key.Binding
	StartStop key.Binding
	NewApp    key.Binding
	Logs      key.Binding
	Quit      key.Binding
}

func (k dashboardKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Settings, k.Logs, k.NewApp, k.StartStop, k.Quit}
}

func (k dashboardKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{{k.Up, k.Down, k.Settings, k.Logs, k.NewApp, k.StartStop, k.Quit}}
}

var dashboardKeys = dashboardKeyMap{
	Up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Settings:  key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "settings")),
	StartStop: key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "start/stop")),
	NewApp:    key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new app")),
	Logs:      key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "logs")),
	Quit:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "quit")),
}

type Dashboard struct {
	namespace     *docker.Namespace
	scraper       *metrics.MetricsScraper
	dockerScraper *docker.Scraper
	apps          []*docker.Application
	panels        []DashboardPanel
	selectedIndex int
	width, height int
	viewport      viewport.Model
	toggling      bool
	togglingApp   string
	progress      ProgressBusy
	help          Help
	showingMenu   bool
	settingsMenu  SettingsMenu
}

type dashboardTickMsg struct{}

type startStopFinishedMsg struct {
	err error
}

func NewDashboard(ns *docker.Namespace, apps []*docker.Application, selectedIndex int,
	scraper *metrics.MetricsScraper, dockerScraper *docker.Scraper) Dashboard {

	vp := viewport.New()
	vp.MouseWheelEnabled = true
	vp.KeyMap.Up.SetEnabled(false)
	vp.KeyMap.Down.SetEnabled(false)
	vp.KeyMap.PageUp.SetEnabled(false)
	vp.KeyMap.PageDown.SetEnabled(false)
	vp.KeyMap.HalfPageUp.SetEnabled(false)
	vp.KeyMap.HalfPageDown.SetEnabled(false)
	vp.KeyMap.Left.SetEnabled(false)
	vp.KeyMap.Right.SetEnabled(false)

	d := Dashboard{
		namespace:     ns,
		scraper:       scraper,
		dockerScraper: dockerScraper,
		apps:          apps,
		selectedIndex: selectedIndex,
		viewport:      vp,
		help:          NewHelp(),
	}
	d.buildPanels()
	return d
}

func (m Dashboard) Init() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return dashboardTickMsg{} })
}

func (m Dashboard) Update(msg tea.Msg) (Component, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.progress = NewProgressBusy(m.width, Colors.Border)
		m.help.SetWidth(m.width)
		m.updateViewportSize()
		m.rebuildViewportContent()

		if m.showingMenu {
			m.settingsMenu, _ = m.settingsMenu.Update(msg)
		}

	case ComponentSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.progress = NewProgressBusy(m.width, Colors.Border)
		m.help.SetWidth(m.width)
		m.updateViewportSize()
		m.rebuildViewportContent()

	case tea.MouseClickMsg:
		if m.showingMenu {
			var cmd tea.Cmd
			m.settingsMenu, cmd = m.settingsMenu.Update(msg)
			return m, cmd
		}
		if cmd := m.help.Update(msg, dashboardKeys); cmd != nil {
			return m, cmd
		}
		if msg.Button == tea.MouseLeft {
			for i := range m.apps {
				if zi := zone.Get(panelZoneID(i)); zi != nil && zi.InBounds(msg) {
					m.selectedIndex = i
					m.rebuildViewportContent()
					m.scrollToSelection()
					return m, nil
				}
			}
		}

	case tea.MouseWheelMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)

	case tea.KeyMsg:
		if m.showingMenu {
			var cmd tea.Cmd
			m.settingsMenu, cmd = m.settingsMenu.Update(msg)
			return m, cmd
		}

		if key.Matches(msg, dashboardKeys.Quit) {
			return m, func() tea.Msg { return quitMsg{} }
		}
		if key.Matches(msg, dashboardKeys.Up) {
			if m.selectedIndex > 0 {
				m.selectedIndex--
				m.rebuildViewportContent()
				m.scrollToSelection()
			}
			return m, nil
		}
		if key.Matches(msg, dashboardKeys.Down) {
			if m.selectedIndex < len(m.apps)-1 {
				m.selectedIndex++
				m.rebuildViewportContent()
				m.scrollToSelection()
			}
			return m, nil
		}
		if key.Matches(msg, dashboardKeys.NewApp) {
			return m, func() tea.Msg { return navigateToInstallMsg{} }
		}
		if key.Matches(msg, dashboardKeys.Settings) && len(m.apps) > 0 {
			app := m.apps[m.selectedIndex]
			m.showingMenu = true
			m.settingsMenu = NewSettingsMenu(app)
			m.settingsMenu, _ = m.settingsMenu.Update(tea.WindowSizeMsg{Width: m.width, Height: m.height})
			return m, nil
		}
		if key.Matches(msg, dashboardKeys.StartStop) && len(m.apps) > 0 && !m.toggling {
			app := m.apps[m.selectedIndex]
			m.toggling = true
			m.togglingApp = app.Settings.Name
			m.progress = NewProgressBusy(m.width, Colors.Border)
			m.updateViewportSize()
			m.rebuildViewportContent()
			return m, tea.Batch(m.progress.Init(), m.runStartStop(app))
		}
		if key.Matches(msg, dashboardKeys.Logs) && len(m.apps) > 0 {
			return m, func() tea.Msg { return navigateToLogsMsg{app: m.apps[m.selectedIndex]} }
		}

	case SettingsMenuCloseMsg:
		m.showingMenu = false

	case SettingsMenuSelectMsg:
		m.showingMenu = false
		return m, func() tea.Msg {
			return navigateToSettingsSectionMsg(msg)
		}

	case startStopFinishedMsg:
		m.toggling = false
		m.togglingApp = ""
		m.updateViewportSize()
		m.rebuildViewportContent()

	case scrapeDoneMsg:
		m.rebuildViewportContent()

	case dashboardTickMsg:
		m.rebuildViewportContent()
		cmds = append(cmds, tea.Tick(time.Second, func(time.Time) tea.Msg { return dashboardTickMsg{} }))

	case progressBusyTickMsg:
		if m.toggling {
			var cmd tea.Cmd
			m.progress, cmd = m.progress.Update(msg)
			cmds = append(cmds, cmd)
		}

	case namespaceChangedMsg:
		previousName := ""
		if m.selectedIndex < len(m.apps) {
			previousName = m.apps[m.selectedIndex].Settings.Name
		}
		m.apps = m.namespace.Applications()
		m.buildPanels()
		m.selectedIndex = 0
		for i, app := range m.apps {
			if app.Settings.Name == previousName {
				m.selectedIndex = i
				break
			}
		}
		if m.selectedIndex >= len(m.apps) && len(m.apps) > 0 {
			m.selectedIndex = len(m.apps) - 1
		}
		m.rebuildViewportContent()
		m.scrollToSelection()
	}

	if m.showingMenu {
		var cmd tea.Cmd
		m.settingsMenu, cmd = m.settingsMenu.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m Dashboard) View() string {
	titleLine := m.renderTitle()

	helpView := m.help.View(dashboardKeys)
	helpLine := Styles.HelpLine(m.width, helpView)

	var content string
	if m.toggling {
		content = titleLine + "\n\n" + m.viewport.View() + "\n" + m.progress.View() + "\n" + helpLine
	} else {
		content = titleLine + "\n\n" + m.viewport.View() + "\n" + helpLine
	}

	if m.showingMenu {
		contentLayer := newZoneLayer(content)
		menuLayer := centeredZoneLayer(m.settingsMenu.View(), m.width, m.height)
		return renderPreservingZones(contentLayer, menuLayer)
	}

	return content
}

// Private

func (m Dashboard) runStartStop(app *docker.Application) tea.Cmd {
	return func() tea.Msg {
		var err error
		if app.Running {
			err = app.Stop(context.Background())
		} else {
			err = app.Start(context.Background())
		}
		return startStopFinishedMsg{err: err}
	}
}

func (m *Dashboard) updateViewportSize() {
	titleHeight := 2 // title line + blank line
	helpHeight := 1
	progressHeight := 0
	if m.toggling {
		progressHeight = 1
	}
	vpHeight := m.height - titleHeight - helpHeight - progressHeight
	if vpHeight < 0 {
		vpHeight = 0
	}
	m.viewport.SetHeight(vpHeight)
	m.viewport.SetWidth(m.width)
}

func (m *Dashboard) rebuildViewportContent() {
	var sections []string
	for i := range m.apps {
		if i > 0 {
			sections = append(sections, m.renderSeparator(i-1))
		}
		toggling := m.toggling && m.togglingApp == m.apps[i].Settings.Name
		panel := m.panels[i].View(i == m.selectedIndex, toggling, m.width)
		sections = append(sections, zone.Mark(panelZoneID(i), panel))
	}
	m.viewport.SetContent(strings.Join(sections, "\n"))
}

func (m *Dashboard) scrollToSelection() {
	panelTop := m.selectedIndex * (PanelHeight + PanelGap)
	panelBottom := panelTop + PanelHeight
	if panelTop < m.viewport.YOffset() {
		m.viewport.SetYOffset(panelTop)
	} else if panelBottom > m.viewport.YOffset()+m.viewport.Height() {
		m.viewport.SetYOffset(panelBottom - m.viewport.Height())
	}
}

func (m Dashboard) renderTitle() string {
	label := " ONCE "
	// 2 chars for end caps ╶ and ╴
	ruleWidth := m.width - 2
	if ruleWidth < len(label) {
		ruleWidth = len(label)
	}
	side := (ruleWidth - len(label)) / 2
	remainder := ruleWidth - len(label) - side*2
	line := "╶" + strings.Repeat("─", side) + label + strings.Repeat("─", side+remainder) + "╴"
	return lipgloss.NewStyle().Foreground(Colors.Border).Render(line)
}

func (m Dashboard) renderSeparator(_ int) string {
	return ""
}

func panelZoneID(index int) string {
	return fmt.Sprintf("dashboard_%d", index)
}

func (m *Dashboard) buildPanels() {
	m.panels = make([]DashboardPanel, len(m.apps))
	for i, app := range m.apps {
		m.panels[i] = NewDashboardPanel(app, m.scraper, m.dockerScraper)
	}
}
