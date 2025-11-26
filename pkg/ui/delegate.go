package ui

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// IssueDelegate renders issue items in the list
type IssueDelegate struct {
	Theme Theme
}

func (d IssueDelegate) Height() int {
	return 1
}

func (d IssueDelegate) Spacing() int {
	return 0
}

func (d IssueDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

func (d IssueDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(IssueItem)
	if !ok {
		return
	}

	t := d.Theme
	width := m.Width()
	if width <= 0 {
		width = 80
	}

	isSelected := index == m.Index()

	// Column definitions - we'll use the FULL width intelligently
	// Layout: [sel] [type] [prio] [status] [ID] [title...] [labels] [assignee] [age] [comments]

	// Get all the data
	icon, iconColor := t.GetTypeIcon(string(i.Issue.IssueType))
	prioIcon := GetPriorityIcon(i.Issue.Priority)
	statusStr := strings.ToUpper(string(i.Issue.Status))
	idStr := i.Issue.ID
	title := i.Issue.Title
	ageStr := FormatTimeRel(i.Issue.CreatedAt)
	commentCount := len(i.Issue.Comments)

	// Calculate widths for right-side columns (fixed)
	// These go on the right: [age 8] [comments 4] [assignee 12] [labels ~20]
	rightWidth := 0
	var rightParts []string

	// Always show age
	rightParts = append(rightParts, fmt.Sprintf("%8s", ageStr))
	rightWidth += 9

	// Comments
	if commentCount > 0 {
		rightParts = append(rightParts, fmt.Sprintf("💬%-2d", commentCount))
	} else {
		rightParts = append(rightParts, "    ")
	}
	rightWidth += 5

	// Assignee (if present and we have room)
	if width > 100 && i.Issue.Assignee != "" {
		assignee := truncateRunesHelper(i.Issue.Assignee, 12, "…")
		rightParts = append(rightParts, fmt.Sprintf("@%-12s", assignee))
		rightWidth += 14
	}

	// Labels (if present and we have room)
	if width > 140 && len(i.Issue.Labels) > 0 {
		labelStr := truncateRunesHelper(strings.Join(i.Issue.Labels, ","), 25, "…")
		rightParts = append(rightParts, fmt.Sprintf("[%-25s]", labelStr))
		rightWidth += 28
	}

	// Left side fixed columns
	// [selector 2] [icon 2] [prio 2] [status 12] [id dynamic] [space]
	leftFixedWidth := 2 + 3 + 3 + 12 + 1

	// ID width - use actual rune length, but cap reasonably
	idRunes := []rune(idStr)
	idWidth := len(idRunes)
	if idWidth > 35 {
		idWidth = 35
		idStr = truncateRunesHelper(idStr, 35, "…")
	}
	leftFixedWidth += idWidth + 1

	// Title gets everything in between
	titleWidth := width - leftFixedWidth - rightWidth - 2
	if titleWidth < 15 {
		titleWidth = 15
	}

	// Truncate title if needed
	titleRunes := []rune(title)
	if len(titleRunes) > titleWidth {
		title = string(titleRunes[:titleWidth-1]) + "…"
	} else {
		// Pad title to fill space
		title = title + strings.Repeat(" ", titleWidth-len(titleRunes))
	}

	// Build left side
	var leftSide strings.Builder
	if isSelected {
		leftSide.WriteString("▸ ")
	} else {
		leftSide.WriteString("  ")
	}

	// Render icon with color
	leftSide.WriteString(t.Renderer.NewStyle().Foreground(iconColor).Render(icon))
	leftSide.WriteString(" ")

	// Priority
	leftSide.WriteString(prioIcon)
	leftSide.WriteString(" ")

	// Status with color
	statusColor := t.GetStatusColor(string(i.Issue.Status))
	leftSide.WriteString(t.Renderer.NewStyle().Width(11).Foreground(statusColor).Bold(true).Render(statusStr))
	leftSide.WriteString(" ")

	// ID
	leftSide.WriteString(t.Renderer.NewStyle().Foreground(t.Secondary).Bold(true).Render(idStr))
	leftSide.WriteString(" ")

	// Title
	if isSelected {
		leftSide.WriteString(t.Renderer.NewStyle().Foreground(t.Primary).Bold(true).Render(title))
	} else {
		leftSide.WriteString(title)
	}

	// Right side
	rightSide := strings.Join(rightParts, " ")

	// Combine: left + padding + right
	leftLen := lipgloss.Width(leftSide.String())
	rightLen := lipgloss.Width(rightSide)
	padding := width - leftLen - rightLen - 1
	if padding < 1 {
		padding = 1
	}

	row := leftSide.String() + strings.Repeat(" ", padding) + t.Renderer.NewStyle().Foreground(t.Secondary).Render(rightSide)

	// Apply row background for selection
	if isSelected {
		row = t.Renderer.NewStyle().
			Background(t.Highlight).
			Width(width - 1).
			Render(row)
	}

	fmt.Fprint(w, row)
}
