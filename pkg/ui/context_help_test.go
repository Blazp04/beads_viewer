package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestContextHelpContentMap(t *testing.T) {
	// Verify all expected contexts have help content
	expectedContexts := []Context{
		ContextList,
		ContextGraph,
		ContextBoard,
		ContextInsights,
		ContextHistory,
		ContextDetail,
		ContextSplit,
		ContextFilter,
		ContextLabelPicker,
		ContextRecipePicker,
		ContextHelp,
		ContextTimeTravel,
		ContextLabelDashboard,
		ContextAttention,
		ContextAgentPrompt,
	}

	for _, ctx := range expectedContexts {
		content, ok := ContextHelpContent[ctx]
		if !ok {
			t.Errorf("ContextHelpContent missing entry for context: %v", ctx)
			continue
		}
		if content == "" {
			t.Errorf("ContextHelpContent has empty content for context: %v", ctx)
		}
	}
}

func TestGetContextHelp(t *testing.T) {
	tests := []struct {
		name     string
		ctx      Context
		contains string // expected substring in the result
	}{
		{
			name:     "list context",
			ctx:      ContextList,
			contains: "List View",
		},
		{
			name:     "graph context",
			ctx:      ContextGraph,
			contains: "Graph View",
		},
		{
			name:     "board context",
			ctx:      ContextBoard,
			contains: "Board View",
		},
		{
			name:     "insights context",
			ctx:      ContextInsights,
			contains: "Insights Panel",
		},
		{
			name:     "history context",
			ctx:      ContextHistory,
			contains: "History View",
		},
		{
			name:     "detail context",
			ctx:      ContextDetail,
			contains: "Detail View",
		},
		{
			name:     "split context",
			ctx:      ContextSplit,
			contains: "Split View",
		},
		{
			name:     "filter context",
			ctx:      ContextFilter,
			contains: "Filter Mode",
		},
		{
			name:     "unknown context falls back to generic",
			ctx:      Context("unknown-context"), // Invalid context
			contains: "Quick Reference",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetContextHelp(tt.ctx)
			if !strings.Contains(result, tt.contains) {
				t.Errorf("GetContextHelp(%v) should contain %q, got: %s", tt.ctx, tt.contains, result)
			}
		})
	}
}

func TestGetContextHelpFallback(t *testing.T) {
	// Test that unknown contexts fall back to generic content
	unknownCtx := Context("nonexistent-context")
	result := GetContextHelp(unknownCtx)

	if result != contextHelpGeneric {
		t.Errorf("GetContextHelp for unknown context should return contextHelpGeneric")
	}

	// Generic content should contain basic navigation
	if !strings.Contains(result, "Global Keys") {
		t.Error("Generic help should contain Global Keys section")
	}
}

func TestContextHelpContentQuality(t *testing.T) {
	// Verify each help content has expected structure
	for ctx, content := range ContextHelpContent {
		t.Run(fmt.Sprintf("context_%s", ctx), func(t *testing.T) {
			// Should have a heading
			if !strings.Contains(content, "##") {
				t.Errorf("Context %v help should have markdown heading", ctx)
			}

			// Most should have Navigation/Actions/Input/Focus/Search section (except generic)
			if ctx != ContextHelp && !strings.Contains(content, "Navigation") &&
				!strings.Contains(content, "Actions") &&
				!strings.Contains(content, "Input") &&
				!strings.Contains(content, "Focus") &&
				!strings.Contains(content, "Search") {
				t.Errorf("Context %v help should have Navigation/Actions/Input/Focus/Search section", ctx)
			}

			// Should not be too short (at least 100 chars of useful content)
			if len(content) < 100 {
				t.Errorf("Context %v help content too short (%d chars)", ctx, len(content))
			}

			// Should not be too long (compact modal, aim for ~20 lines)
			lines := strings.Count(content, "\n")
			if lines > 30 {
				t.Errorf("Context %v help has %d lines (should be <=30 for compact display)", ctx, lines)
			}
		})
	}
}

func TestRenderContextHelp(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	width, height := 80, 40

	result := RenderContextHelp(ContextList, theme, width, height)

	// Should have modal border
	if !strings.Contains(result, "╭") || !strings.Contains(result, "╮") {
		t.Error("RenderContextHelp should render with rounded border")
	}

	// Should have title
	if !strings.Contains(result, "Quick Reference") {
		t.Error("RenderContextHelp should show 'Quick Reference' title")
	}

	// Should have footer hint
	if !strings.Contains(result, "Esc to close") {
		t.Error("RenderContextHelp should show close hint")
	}

	// Should have context-specific content
	if !strings.Contains(result, "List View") {
		t.Error("RenderContextHelp should include context-specific content")
	}
}

func TestRenderContextHelpNarrowWidth(t *testing.T) {
	theme := DefaultTheme(lipgloss.NewRenderer(nil))
	narrowWidth := 50
	height := 40

	result := RenderContextHelp(ContextList, theme, narrowWidth, height)

	// Should adapt to narrow width (modal width = width - 4)
	// Just verify it renders without panicking
	if result == "" {
		t.Error("RenderContextHelp should produce output even for narrow width")
	}
}

func TestContextHelpKeyboardShortcuts(t *testing.T) {
	// Verify essential shortcuts are documented in relevant contexts
	tests := []struct {
		ctx      Context
		shortcut string
	}{
		{ContextList, "j/k"},
		{ContextList, "Enter"},
		{ContextGraph, "h/l"},
		{ContextGraph, "f"},
		{ContextBoard, "m"},
		{ContextDetail, "Esc"},
		{ContextSplit, "Tab"},
		{ContextFilter, "/"},
	}

	for _, tt := range tests {
		content := GetContextHelp(tt.ctx)
		if !strings.Contains(content, tt.shortcut) {
			t.Errorf("Context %v help should document shortcut %q", tt.ctx, tt.shortcut)
		}
	}
}
