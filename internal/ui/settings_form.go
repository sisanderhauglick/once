package ui

import (
	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/basecamp/amar/internal/docker"
)

type settingsFormField int

const (
	settingsFieldImage settingsFormField = iota
	settingsFieldHostname
	settingsFieldTLS
	settingsFieldSaveButton
	settingsFieldCancelButton
	settingsFieldCount
)

type SettingsFormSubmitMsg struct {
	Settings docker.ApplicationSettings
}

type SettingsFormCancelMsg struct{}

type SettingsForm struct {
	width, height int
	focused       settingsFormField
	settings      docker.ApplicationSettings
	imageInput    textinput.Model
	hostnameInput textinput.Model
}

func NewSettingsForm(settings docker.ApplicationSettings) SettingsForm {
	image := textinput.New()
	image.Placeholder = "user/repo:tag"
	image.Prompt = ""
	image.CharLimit = 256
	image.SetValue(settings.Image)
	image.Focus()

	hostname := textinput.New()
	hostname.Placeholder = "app.example.com"
	hostname.Prompt = ""
	hostname.CharLimit = 256
	hostname.SetValue(settings.Host)

	return SettingsForm{
		focused:       settingsFieldImage,
		settings:      settings,
		imageInput:    image,
		hostnameInput: hostname,
	}
}

func (m SettingsForm) Init() tea.Cmd {
	return nil
}

func (m SettingsForm) Update(msg tea.Msg) (SettingsForm, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		inputWidth := min(m.width-4, 60)
		m.imageInput.SetWidth(inputWidth)
		m.hostnameInput.SetWidth(inputWidth)

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
			return m.focusNext()
		case key.Matches(msg, key.NewBinding(key.WithKeys("shift+tab"))):
			return m.focusPrev()
		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			return m.handleEnter()
		case key.Matches(msg, key.NewBinding(key.WithKeys("space"))) && m.focused == settingsFieldTLS:
			if !docker.IsLocalhost(m.settings.Host) {
				m.settings.DisableTLS = !m.settings.DisableTLS
			}
			return m, nil
		}
	}

	switch m.focused {
	case settingsFieldImage:
		var cmd tea.Cmd
		m.imageInput, cmd = m.imageInput.Update(msg)
		m.settings.Image = m.imageInput.Value()
		cmds = append(cmds, cmd)
	case settingsFieldHostname:
		var cmd tea.Cmd
		m.hostnameInput, cmd = m.hostnameInput.Update(msg)
		m.settings.Host = m.hostnameInput.Value()
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m SettingsForm) View() string {
	imageLabel := Styles.Label.Render("Image")
	imageField := Styles.Focus(Styles.Input, m.focused == settingsFieldImage).
		Render(m.imageInput.View())

	hostnameLabel := Styles.Label.Render("Hostname")
	hostnameField := Styles.Focus(Styles.Input, m.focused == settingsFieldHostname).
		Render(m.hostnameInput.View())

	tlsLabel := Styles.Label.Render("TLS")
	var tlsText string
	if docker.IsLocalhost(m.settings.Host) {
		tlsText = "Not available for localhost"
	} else if m.settings.TLSEnabled() {
		tlsText = "[x] Enabled"
	} else {
		tlsText = "[ ] Enabled"
	}
	tlsField := Styles.Focus(Styles.Input, m.focused == settingsFieldTLS).
		Render(tlsText)

	saveButton := Styles.Focus(Styles.ButtonPrimary, m.focused == settingsFieldSaveButton).
		Render("Save")
	cancelButton := Styles.Focus(Styles.Button, m.focused == settingsFieldCancelButton).
		Render("Cancel")

	buttons := lipgloss.JoinHorizontal(lipgloss.Center, saveButton, cancelButton)

	form := lipgloss.JoinVertical(lipgloss.Left,
		imageLabel,
		imageField,
		hostnameLabel,
		hostnameField,
		tlsLabel,
		tlsField,
		"",
		buttons,
	)

	return form
}

// Private

func (m SettingsForm) focusNext() (SettingsForm, tea.Cmd) {
	m.blurCurrent()
	m.focused = (m.focused + 1) % settingsFieldCount
	return m.focusCurrent()
}

func (m SettingsForm) focusPrev() (SettingsForm, tea.Cmd) {
	m.blurCurrent()
	m.focused = (m.focused - 1 + settingsFieldCount) % settingsFieldCount
	return m.focusCurrent()
}

func (m *SettingsForm) blurCurrent() {
	switch m.focused {
	case settingsFieldImage:
		m.imageInput.Blur()
	case settingsFieldHostname:
		m.hostnameInput.Blur()
	}
}

func (m SettingsForm) focusCurrent() (SettingsForm, tea.Cmd) {
	var cmd tea.Cmd
	switch m.focused {
	case settingsFieldImage:
		cmd = m.imageInput.Focus()
	case settingsFieldHostname:
		cmd = m.hostnameInput.Focus()
	}
	return m, cmd
}

func (m SettingsForm) handleEnter() (SettingsForm, tea.Cmd) {
	switch m.focused {
	case settingsFieldImage, settingsFieldHostname, settingsFieldTLS:
		return m.focusNext()
	case settingsFieldSaveButton:
		return m.submitForm()
	case settingsFieldCancelButton:
		return m, func() tea.Msg { return SettingsFormCancelMsg{} }
	}
	return m, nil
}

func (m SettingsForm) submitForm() (SettingsForm, tea.Cmd) {
	return m, func() tea.Msg {
		return SettingsFormSubmitMsg{Settings: m.settings}
	}
}
