package model

import (
	"testing"
	"time"
)

func TestStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{"Open", StatusOpen, true},
		{"InProgress", StatusInProgress, true},
		{"Blocked", StatusBlocked, true},
		{"Closed", StatusClosed, true},
		{"Invalid", "unknown", false},
		{"Empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Errorf("Status.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatus_IsClosed(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{"Open", StatusOpen, false},
		{"InProgress", StatusInProgress, false},
		{"Blocked", StatusBlocked, false},
		{"Closed", StatusClosed, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsClosed(); got != tt.want {
				t.Errorf("Status.IsClosed() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestStatus_IsOpen(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{"Open", StatusOpen, true},
		{"InProgress", StatusInProgress, true},
		{"Blocked", StatusBlocked, false},
		{"Closed", StatusClosed, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.status.IsOpen(); got != tt.want {
				t.Errorf("Status.IsOpen() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIssueType_IsValid(t *testing.T) {
	tests := []struct {
		name      string
		issueType IssueType
		want      bool
	}{
		{"Bug", TypeBug, true},
		{"Feature", TypeFeature, true},
		{"Task", TypeTask, true},
		{"Epic", TypeEpic, true},
		{"Chore", TypeChore, true},
		{"Invalid", "random", false},
		{"Empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.issueType.IsValid(); got != tt.want {
				t.Errorf("IssueType.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDependencyType_IsValid(t *testing.T) {
	tests := []struct {
		name    string
		depType DependencyType
		want    bool
	}{
		{"Blocks", DepBlocks, true},
		{"Related", DepRelated, true},
		{"ParentChild", DepParentChild, true},
		{"DiscoveredFrom", DepDiscoveredFrom, true},
		{"Invalid", "causes", false},
		{"Empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.depType.IsValid(); got != tt.want {
				t.Errorf("DependencyType.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDependencyType_IsBlocking(t *testing.T) {
	tests := []struct {
		name    string
		depType DependencyType
		want    bool
	}{
		{"Blocks", DepBlocks, true},
		{"Related", DepRelated, false},
		{"ParentChild", DepParentChild, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.depType.IsBlocking(); got != tt.want {
				t.Errorf("DependencyType.IsBlocking() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIssue_Struct(t *testing.T) {
	// This test verifies that we can construct an Issue with valid data
	now := time.Now()
	issue := &Issue{
		ID:          "TEST-123",
		Title:       "Test Issue",
		Description: "This is a test issue",
		Status:      StatusOpen,
		Priority:    1, // lower is higher priority
		IssueType:   TypeTask,
		CreatedAt:   now,
		UpdatedAt:   now,
		Labels:      []string{"test", "unit"},
	}

	if issue.ID != "TEST-123" {
		t.Errorf("Issue ID mismatch: got %s, want TEST-123", issue.ID)
	}
	if !issue.Status.IsValid() {
		t.Errorf("Issue Status should be valid")
	}
	if !issue.IssueType.IsValid() {
		t.Errorf("Issue Type should be valid")
	}

	// UpdatedAt should never be before CreatedAt in valid data
	if issue.UpdatedAt.Before(issue.CreatedAt) {
		t.Errorf("UpdatedAt should be >= CreatedAt")
	}
}

func TestDependency_Struct(t *testing.T) {
	now := time.Now()
	dep := &Dependency{
		IssueID:     "A",
		DependsOnID: "B",
		Type:        DepBlocks,
		CreatedAt:   now,
		CreatedBy:   "user",
	}

	if dep.IssueID != "A" {
		t.Errorf("IssueID mismatch")
	}
	if !dep.Type.IsValid() {
		t.Errorf("Dependency type should be valid")
	}
	if !dep.Type.IsBlocking() {
		t.Errorf("DepBlocks should be blocking")
	}
}

func TestComment_Struct(t *testing.T) {
	now := time.Now()
	comment := &Comment{
		ID:        1,
		IssueID:   "A",
		Author:    "user",
		Text:      "hello",
		CreatedAt: now,
	}

	if comment.IssueID != "A" {
		t.Errorf("IssueID mismatch")
	}
	if comment.Text != "hello" {
		t.Errorf("Text mismatch")
	}
}

func TestIssue_UpdatedBeforeCreated(t *testing.T) {
	created := time.Now()
	updated := created.Add(-time.Hour)
	issue := Issue{
		ID:        "TEST-999",
		Title:     "Backwards time",
		Status:    StatusOpen,
		IssueType: TypeTask,
		CreatedAt: created,
		UpdatedAt: updated,
	}

	if !issue.UpdatedAt.Before(issue.CreatedAt) {
		t.Fatalf("sanity check failed: expected UpdatedAt before CreatedAt")
	}
	if issue.UpdatedAt.Before(issue.CreatedAt) {
		// The struct has no validation method; document expected invariant for future validators
		t.Log("UpdatedAt is before CreatedAt; future validation should reject this")
	}
}
