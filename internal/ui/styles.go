package ui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/compat"
)

type colors struct {
	Primary         color.Color
	Secondary       color.Color
	Background      color.Color
	Text            color.Color
	TextDark        color.Color
	Focused         color.Color
	Border          color.Color
	Success         color.Color
	Warning         color.Color
	Error           color.Color
	Info            color.Color
	Muted           color.Color
	PanelBg         color.Color
}

var Colors = colors{
	Primary:         lipgloss.Color("#7AA2F7"),
	Secondary:       lipgloss.Color("#9f3"),
	Background:      lipgloss.Color("#000000"),
	Text:            lipgloss.Color("#FFFFFF"),
	TextDark:        lipgloss.Color("#000000"),
	Focused:         lipgloss.Color("#FFA500"),
	Border:          lipgloss.Color("#6272a4"),
	Success:         lipgloss.Color("#50fa7b"),
	Warning:         lipgloss.Color("#f1fa8c"),
	Error:           lipgloss.Color("#ff5555"),
	Info:            lipgloss.Color("#8be9fd"),
	Muted:           lipgloss.Color("#bd93f9"),
	PanelBg: compat.AdaptiveColor{
		Light: lipgloss.Color("#e8e8e8"),
		Dark:  lipgloss.Color("#1a1b26"),
	},
}

var ChartColors = struct {
	CPU      lipgloss.Style
	Memory   lipgloss.Style
	Requests lipgloss.Style
	Errors   lipgloss.Style
}{
	CPU:      lipgloss.NewStyle().Foreground(Colors.Info),
	Memory:   lipgloss.NewStyle().Foreground(Colors.Muted),
	Requests: lipgloss.NewStyle().Foreground(Colors.Success),
	Errors:   lipgloss.NewStyle().Foreground(Colors.Error),
}

type styles struct {
	Title         lipgloss.Style
	Label         lipgloss.Style
	Input         lipgloss.Style
	Button        lipgloss.Style
	ButtonPrimary lipgloss.Style
}

var Styles = styles{
	Title: lipgloss.NewStyle().
		Foreground(Colors.Primary).
		Bold(true),
	Label: lipgloss.NewStyle().
		Bold(true),
	Input: lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Colors.Border).
		Padding(0, 1).
		MarginBottom(1),
	Button: lipgloss.NewStyle().
		Padding(0, 2).
		MarginRight(1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Colors.Border),
	ButtonPrimary: lipgloss.NewStyle().
		Padding(0, 2).
		MarginRight(1).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Colors.Primary),
}

func (s styles) Focus(base lipgloss.Style, focused bool) lipgloss.Style {
	if focused {
		return base.BorderForeground(Colors.Focused)
	}
	return base
}

func (s styles) TitleRule(width int, crumbs ...string) string {
	label := " " + strings.Join(append([]string{"ONCE"}, crumbs...), " · ") + " "
	ruleWidth := width - 2 // end caps
	if ruleWidth < len(label) {
		ruleWidth = len(label)
	}
	side := (ruleWidth - len(label)) / 2
	remainder := ruleWidth - len(label) - side*2
	line := "╶" + strings.Repeat("─", side) + label + strings.Repeat("─", side+remainder) + "╴"
	return lipgloss.NewStyle().Foreground(Colors.Border).Render(line)
}

func (s styles) HelpLine(width int, content string) string {
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(content)
}

func (s styles) CenteredLine(width int, content string) string {
	return lipgloss.NewStyle().Width(width).Align(lipgloss.Center).Render(content)
}

