package ui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

type HealthState int

const (
	healthNormal HealthState = iota
	healthWarning
	healthError
)

func (h HealthState) Color() color.Color {
	switch h {
	case healthWarning:
		return chartGradientTop
	case healthError:
		return Colors.Error
	default:
		return chartGradientBottom
	}
}

type MetricThresholds struct {
	Warning float64
	Error   float64
}

func (t MetricThresholds) Health(pct float64) HealthState {
	switch {
	case pct >= t.Error:
		return healthError
	case pct >= t.Warning:
		return healthWarning
	default:
		return healthNormal
	}
}

func (t MetricThresholds) Color(pct float64) color.Color {
	return t.Health(pct).Color()
}

type MetricCard struct {
	title      string
	data       []float64
	scale      ChartScale
	unit       UnitType
	limitLabel string
	healthPct  float64
	thresholds MetricThresholds

	// Traffic-specific fields
	isTraffic bool
	errData   []float64
}

func NewMetricCard(title string, data []float64, scale ChartScale, unit UnitType, limitLabel string, warning, error float64) MetricCard {
	scaleMax := scale.Max()
	if scaleMax == 0 {
		scaleMax = 1
	}
	current := lastValue(data)

	return MetricCard{
		title:      title,
		data:       data,
		scale:      scale,
		unit:       unit,
		limitLabel: limitLabel,
		healthPct:  current / scaleMax * 100,
		thresholds: MetricThresholds{Warning: warning, Error: error},
	}
}

func NewTrafficCard(reqData, errData []float64, scale ChartScale, errPct float64, warning, error float64) MetricCard {
	scaleLabel := ""
	if scale.Max() > 0 {
		scaleLabel = UnitCount.Format(scale.Max())
	}

	return MetricCard{
		title:      "Traffic",
		data:       reqData,
		scale:      scale,
		unit:       UnitCount,
		limitLabel: scaleLabel,
		healthPct:  errPct,
		thresholds: MetricThresholds{Warning: warning, Error: error},
		isTraffic:  true,
		errData:    errData,
	}
}

func (c MetricCard) Health() HealthState {
	return c.thresholds.Health(c.healthPct)
}

func (c MetricCard) View(width int) string {
	borderStyle := lipgloss.NewStyle().Foreground(Colors.Border)
	inner := width - 2

	topFill := max(inner-1-len(c.title), 0)
	topLine := borderStyle.Render("╭─" + c.title + strings.Repeat("─", topFill) + "╮")
	bottomLine := borderStyle.Render("╰" + strings.Repeat("─", inner) + "╯")

	left := borderStyle.Render("│")
	right := borderStyle.Render("│")

	var barStr, valueLine, detailLine string
	if c.isTraffic {
		barStr, valueLine, detailLine = c.trafficLines(inner)
	} else {
		barStr, valueLine, detailLine = c.metricLines(inner)
	}

	contentLines := []string{
		left + " " + barStr + padOrTruncate("", max(inner-1-lipgloss.Width(barStr), 0)) + right,
		left + valueLine + right,
		left + padOrTruncate(detailLine, inner) + right,
	}

	return topLine + "\n" + strings.Join(contentLines, "\n") + "\n" + bottomLine
}

// Private

func (c MetricCard) metricLines(inner int) (barStr, valueLine, detailLine string) {
	current := lastValue(c.data)
	peak := peakValue(c.data, peakWindow)
	scaleMax := c.scale.Max()
	if scaleMax == 0 {
		scaleMax = 1
	}

	barStr = renderBar(current, peak, scaleMax, c.thresholds.Color(c.healthPct), max(inner-2, 0))
	valueLine = formatValueLine(" "+c.unit.Format(current), c.limitLabel, inner)
	detailLine = " peak: " + c.unit.Format(peak)
	return
}

func (c MetricCard) trafficLines(inner int) (barStr, valueLine, detailLine string) {
	currentReq := lastValue(c.data)
	peakReq := peakValue(c.data, peakWindow)
	scaleMax := c.scale.Max()
	if scaleMax == 0 {
		scaleMax = 1
	}

	barStr = renderBar(currentReq, peakReq, scaleMax, c.thresholds.Color(c.healthPct), max(inner-2, 0))
	valueLine = formatValueLine(" "+UnitCount.Format(currentReq)+"/min", c.limitLabel, inner)

	currentErr := lastValue(c.errData)
	if currentErr > 0 && currentReq > 0 {
		pct := currentErr / currentReq * 100
		health := c.thresholds.Health(c.healthPct)
		errText := fmt.Sprintf("%.0f%% errors", pct)
		if health > healthNormal {
			errText = lipgloss.NewStyle().Foreground(health.Color()).Render(errText)
		}
		detailLine = " " + errText
	} else {
		detailLine = " 0% errors"
	}
	return
}

func formatValueLine(valueStr, limitLabel string, inner int) string {
	if limitLabel != "" {
		limitStr := lipgloss.NewStyle().Foreground(Colors.Border).Render("·" + limitLabel)
		gap := max(inner-lipgloss.Width(valueStr)-lipgloss.Width(limitStr)-1, 0)
		return valueStr + strings.Repeat(" ", gap) + limitStr + " "
	}
	return padOrTruncate(valueStr, inner)
}
