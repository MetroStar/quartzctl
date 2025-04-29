package util

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/MetroStar/quartzctl/internal/log"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// RowStatus represents the status of a row in a table.
type RowStatus string

// Constants for row statuses and their associated colors.
const (
	StatusOk      RowStatus = "[✔]"
	StatusWarning RowStatus = "[i]"
	StatusError   RowStatus = "[✘]"
	StatusUnknown RowStatus = ""

	ColorOk      = "#00AA00"
	ColorWarning = "#AAAA00"
	ColorError   = "#AA0000"
)

var (
	writer io.Writer = os.Stderr

	width = 100

	hdrStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorWarning)).
			Border(lipgloss.DoubleBorder(), true, false, true, false). // top right bottom left
			Width(width)

	msgStyle                                        = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")). // bright white
			PaddingTop(1).
			PaddingLeft(1).
			Width(width)

	txtStyle = lipgloss.NewStyle().
			PaddingLeft(2).
			Italic(true).
			Width(width)

	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Italic(true).
			Foreground(lipgloss.Color(ColorError)). // red
			PaddingLeft(2).
			Width(width)

	tableHeaderStyle                                        = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")). // bright white
				Bold(true).
				Align(lipgloss.Center)

	tableCellStyle = lipgloss.NewStyle().
			Padding(0, 1)

	tableEvenRowStyle = tableCellStyle.Foreground(lipgloss.Color("#808080"))
	tableOddRowStyle  = tableCellStyle.Foreground(lipgloss.Color("#C0C0C0"))

	tableStatusOkCellStyle      = tableCellStyle.Foreground(lipgloss.Color(ColorOk))
	tableStatusWarningCellStyle = tableCellStyle.Foreground(lipgloss.Color(ColorWarning))
	tableStatusErrorCellStyle   = tableCellStyle.Foreground(lipgloss.Color(ColorError))
)

// SetWriter sets the output writer for console messages.
func SetWriter(w io.Writer) {
	writer = w
}

// Hdr prints a header-formatted string to the console.
func Hdr(a ...any) {
	log.Debug("Formatted Header", "content", fmt.Sprint(a...))
	println(writer, &hdrStyle, a...)
}

// Hdrf prints a formatted header string to the console.
func Hdrf(format string, a ...any) {
	log.Debug("Formatted Header", "content", fmt.Sprintf(format, a...))
	printfln(writer, &hdrStyle, format, a...)
}

// Msg prints a message-formatted string to the console.
func Msg(a ...any) {
	log.Debug("Formatted Message", "content", fmt.Sprint(a...))
	println(writer, &msgStyle, a...)
}

// Msgf prints a formatted message string to the console.
func Msgf(format string, a ...any) {
	log.Debug("Formatted Message", "content", fmt.Sprintf(format, a...))
	printfln(writer, &msgStyle, format, a...)
}

// Print prints a formatted string to the console.
func Print(a ...any) {
	log.Debug("Formatted Text", "content", fmt.Sprint(a...))
	println(writer, &txtStyle, a...)
}

// Printf prints a formatted string to the console.
func Printf(format string, a ...any) {
	log.Debug("Formatted Text", "content", fmt.Sprintf(format, a...))
	printfln(writer, &txtStyle, format, a...)
}

// Error prints an error-formatted string to the console.
func Error(a ...any) {
	log.Debug("Formatted Error", "content", fmt.Sprint(a...))
	println(writer, &errorStyle, a...)
}

// Errorf prints a formatted error string to the console.
func Errorf(format string, a ...any) {
	log.Debug("Formatted Error", "content", fmt.Sprintf(format, a...))
	printfln(writer, &errorStyle, format, a...)
}

// PrintTable prints a formatted table to the console.
func PrintTable(headers []string, rows [][]string) {
	log.Debug("Formatted Table", "headers", strings.Join(headers, ","), "rowCount", len(rows))
	printTableC(headers, rows, nil)
}

// PrintRowStatusTable prints a formatted table with a status indicator column to the console.
func PrintRowStatusTable(headers []string, rows [][]string, statusFunc func(i int, row []string) RowStatus) {
	log.Debug("Formatted Status Table", "headers", strings.Join(headers, ","), "rowCount", len(rows))

	headersC := append([]string{""}, headers...) // placeholder header for new status column
	rowsC := make([][]string, len(rows))

	for i, row := range rows {
		s := statusFunc(i, row)
		rowsC[i] = append([]string{string(s)}, row...)
	}

	printTableC(headersC, rowsC, func(row int, col int, cell string) (bool, lipgloss.Style) {
		if col != 0 {
			return false, lipgloss.NewStyle()
		}

		switch cell {
		case string(StatusOk):
			return true, tableStatusOkCellStyle
		case string(StatusWarning):
			return true, tableStatusWarningCellStyle
		case string(StatusError):
			return true, tableStatusErrorCellStyle
		}

		return false, lipgloss.NewStyle()
	})
}

// PromptYesNo displays a yes/no prompt to the console and returns true if "yes" was selected.
func PromptYesNo(msg string) bool {
	log.Debug("Formatted Yes/No Prompt", "message", msg)

	var r bool

	silent := os.Getenv("SILENT") != ""
	if silent {
		return true // assume confirmation when silent is enabled
	}

	accessible := os.Getenv("ACCESSIBLE") != ""

	err := huh.NewConfirm().
		Title(msg).
		Value(&r).
		WithAccessible(accessible).
		Run() // blocking
	if err != nil {
		log.Warn("Error in confirm prompt", "err", err)
		return false
	}

	return r
}

// PrintBanner prints the Quartz ASCII art banner to the console.
func PrintBanner() {
	log.Debug("Printing ASCII banner")

	txt := `
	██████╗ ██╗   ██╗ █████╗ ██████╗ ████████╗███████╗
	██╔═══██╗██║   ██║██╔══██╗██╔══██╗╚══██╔══╝╚══███╔╝
	██║   ██║██║   ██║███████║██████╔╝   ██║     ███╔╝ 
	██║▄▄ ██║██║   ██║██╔══██║██╔══██╗   ██║    ███╔╝  
	╚██████╔╝╚██████╔╝██║  ██║██║  ██║   ██║   ███████╗
	 ╚══▀▀═╝  ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═╝   ╚═╝   ╚══════╝
`

	Hdr(txt)
}

// println writes a styled string to the console with a newline.
func println(w io.Writer, style *lipgloss.Style, a ...any) {
	fmt.Fprintln(w, sprint(style, a...))
}

// printfln writes a formatted styled string to the console with a newline.
func printfln(w io.Writer, style *lipgloss.Style, format string, a ...any) {
	fmt.Fprintln(w, sprintf(style, format, a...))
}

// sprint renders a styled string.
func sprint(style *lipgloss.Style, a ...any) string {
	return style.Render(fmt.Sprint(a...))
}

// sprintf renders a formatted styled string.
func sprintf(style *lipgloss.Style, format string, a ...any) string {
	return style.Render(fmt.Sprintf(format, a...))
}

// printTableC prints a formatted table with custom cell styles to the console.
func printTableC(headers []string, rows [][]string, cellStyleFunc func(row int, col int, cell string) (bool, lipgloss.Style)) {
	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("99"))).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch {
			case row < 0: // -1 for header row
				return tableHeaderStyle
			case cellStyleFunc != nil:
				cell := rows[row][col]
				handled, s := cellStyleFunc(row, col, cell)
				if handled {
					return s
				}
				fallthrough
			case row%2 == 0:
				return tableEvenRowStyle
			default:
				return tableOddRowStyle
			}
		}).
		Headers(headers...).
		Rows(rows...)

	fmt.Println(t)
}
