package main

import (
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/Rhymond/go-money"
)

// Common style definitions
var (
	// Colors
	goldColor   = lipgloss.Color("#fd3")
	borderColor = lipgloss.Color("#cdaa01")
	textColor   = lipgloss.Color("252")

	// Base styles
	headerStyle = lipgloss.NewStyle().
			AlignHorizontal(lipgloss.Center).
			Bold(true).
			Foreground(goldColor)

	titleStyle = headerStyle.Padding(1)

	tableBorderStyle = lipgloss.NewStyle().Foreground(borderColor)
)

func newTable() *table.Table {
	return table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(tableBorderStyle)
}

// Summary Table style function
func summaryTableStyleFunc() func(row, col int) lipgloss.Style {
	return func(row, col int) lipgloss.Style {
		style := lipgloss.NewStyle()

		// Header row styling
		if row == table.HeaderRow {
			return headerStyle
		}

		// Right-align the Value column (column 1) for 2-column tables
		if col == 1 {
			style = style.AlignHorizontal(lipgloss.Right)
		}

		return style.Foreground(textColor)
	}
}

func styleRedGreen(value *money.Money) lipgloss.Style {
	style := lipgloss.NewStyle()
	if value.IsPositive() {
		style = style.Foreground(lipgloss.Green) // Green for gains
	} else if value.IsNegative() {
		style = style.Foreground(lipgloss.Red) // Red for losses
	}
	return style
}

func displayRedGreen(value *money.Money) string {
	style := styleRedGreen(value)
	return style.Render(value.Display())
}

func monetaryTableStyleFunc() func(row, col int) lipgloss.Style {
	return func(row, col int) lipgloss.Style {
		style := lipgloss.NewStyle()

		// Header row styling
		if row == table.HeaderRow {
			return headerStyle
		}

		// Align monetary columns to the right (all columns except Date)
		style = style.AlignHorizontal(lipgloss.Right)
		if col == 0 {
			style = style.AlignHorizontal(lipgloss.Center)
		}

		return style.Foreground(textColor)
	}
}
