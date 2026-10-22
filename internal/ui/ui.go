// Package ui holds the terminal styling and prompts.
package ui

import (
	"errors"

	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"
)

// Styles used across the report.
var (
	Bold   = lipgloss.NewStyle().Bold(true)
	Dim    = lipgloss.NewStyle().Faint(true)
	Red    = lipgloss.NewStyle().Foreground(lipgloss.Red)
	Green  = lipgloss.NewStyle().Foreground(lipgloss.Green)
	Yellow = lipgloss.NewStyle().Foreground(lipgloss.Yellow)
)

// Answer is the user's response to a removal prompt.
type Answer int

const (
	// No skips this item.
	No Answer = iota
	// Yes acts on this item.
	Yes
	// Quit stops prompting for the rest of the repo.
	Quit
)

// AskRemove asks whether to act on one item. Esc/Ctrl-C counts as Quit.
func AskRemove(title string) Answer {
	ans := No
	err := huh.NewSelect[Answer]().
		Title(title).
		Options(
			huh.NewOption("no", No),
			huh.NewOption("yes", Yes),
			huh.NewOption("quit", Quit),
		).
		Inline(true).
		Value(&ans).
		Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return Quit
	}
	if err != nil {
		return No
	}
	return ans
}

// Confirm asks a yes/no question, defaulting to no.
func Confirm(title string) bool {
	var ok bool
	if err := huh.NewConfirm().Title(title).Affirmative("yes").Negative("no").Value(&ok).Run(); err != nil {
		return false
	}
	return ok
}
