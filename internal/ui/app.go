package ui

import (
	"context"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/basecamp/amar/internal/docker"
	"github.com/basecamp/amar/internal/metrics"
)

type KeyMap struct {
	Accept  key.Binding
	Quit    key.Binding
	PrevApp key.Binding
	NextApp key.Binding
}

var DefaultKeyMap = KeyMap{
	Accept:  key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "accept")),
	Quit:    key.NewBinding(key.WithKeys("esc", "ctrl+c"), key.WithHelp("esc", "cancel")),
	PrevApp: key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "previous app")),
	NextApp: key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "next app")),
}

type Component interface {
	Init() tea.Cmd
	Update(tea.Msg) (Component, tea.Cmd)
	View() string
}

type NamespaceChangedMsg struct{}

type App struct {
	namespace      *docker.Namespace
	scraper        *metrics.MetricsScraper
	dockerScraper  *docker.Scraper
	currentIndex   int
	currentScreen  Component
	lastSize       tea.WindowSizeMsg
	eventChan      <-chan struct{}
	watchCtx       context.Context
	watchCancel    context.CancelFunc
}

func NewApp(ns *docker.Namespace) App {
	ctx, cancel := context.WithCancel(context.Background())
	eventChan := ns.EventWatcher().Watch(ctx)

	apps := ns.Applications()

	metricsPort := docker.DefaultMetricsPort
	if ns.Proxy().Settings != nil && ns.Proxy().Settings.MetricsPort != 0 {
		metricsPort = ns.Proxy().Settings.MetricsPort
	}

	scraper := metrics.NewMetricsScraper(metrics.ScraperSettings{
		Port:       metricsPort,
		BufferSize: ChartHistoryLength,
	})
	scraper.Start(ctx)

	dockerScraper := docker.NewScraper(ns, docker.ScraperSettings{
		BufferSize: ChartHistoryLength,
	})
	dockerScraper.Start(ctx)

	var screen Component
	if len(apps) > 0 {
		screen = NewDashboard(apps[0], scraper, dockerScraper)
	} else {
		screen = NewEmptyState()
	}

	return App{
		namespace:      ns,
		scraper:        scraper,
		dockerScraper:  dockerScraper,
		currentIndex:   0,
		currentScreen:  screen,
		eventChan:      eventChan,
		watchCtx:       ctx,
		watchCancel:    cancel,
	}
}

func (m App) Init() tea.Cmd {
	return tea.Batch(m.currentScreen.Init(), m.watchForChanges())
}

func (m App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.lastSize = msg
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, DefaultKeyMap.Quit):
			m.shutdown()
			return m, tea.Quit
		case key.Matches(msg, DefaultKeyMap.PrevApp):
			return m.switchApp(-1)
		case key.Matches(msg, DefaultKeyMap.NextApp):
			return m.switchApp(1)
		}
	case NamespaceChangedMsg:
		_ = m.namespace.Refresh(m.watchCtx)
		apps := m.namespace.Applications()
		if len(apps) > 0 && m.currentIndex < len(apps) {
			m.currentScreen = NewDashboard(apps[m.currentIndex], m.scraper, m.dockerScraper)
			m.currentScreen, _ = m.currentScreen.Update(m.lastSize)
		}
		return m, tea.Batch(m.currentScreen.Init(), m.watchForChanges())
	}

	var cmd tea.Cmd
	m.currentScreen, cmd = m.currentScreen.Update(msg)
	return m, cmd
}

func (m App) View() tea.View {
	view := tea.View{AltScreen: true}
	view.SetContent(m.currentScreen.View())
	return view
}

func Run(ns *docker.Namespace) error {
	app := NewApp(ns)
	_, err := tea.NewProgram(app).Run()
	return err
}

// Private

func (m App) shutdown() {
	m.watchCancel()
	m.scraper.Stop()
	m.dockerScraper.Stop()
}

func (m App) switchApp(delta int) (tea.Model, tea.Cmd) {
	apps := m.namespace.Applications()
	if len(apps) == 0 {
		return m, nil
	}

	newIndex := m.currentIndex + delta
	if newIndex < 0 {
		newIndex = len(apps) - 1
	} else if newIndex >= len(apps) {
		newIndex = 0
	}

	if newIndex == m.currentIndex {
		return m, nil
	}

	m.currentIndex = newIndex
	m.currentScreen = NewDashboard(apps[newIndex], m.scraper, m.dockerScraper)
	m.currentScreen, _ = m.currentScreen.Update(m.lastSize)
	return m, m.currentScreen.Init()
}

func (m App) watchForChanges() tea.Cmd {
	return func() tea.Msg {
		_, ok := <-m.eventChan
		if !ok {
			return nil
		}
		return NamespaceChangedMsg{}
	}
}
