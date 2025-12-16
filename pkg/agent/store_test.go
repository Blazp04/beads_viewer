package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStore_RegisterAndGet(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	agent := NewAgentRegistration("test-agent", "claude-opus-4", "claude-code")
	if err := store.Register(agent); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	got := store.Get("test-agent")
	if got == nil {
		t.Fatal("Get() returned nil")
	}
	if got.Name != "test-agent" {
		t.Errorf("Name = %s, want test-agent", got.Name)
	}
	if got.Model != "claude-opus-4" {
		t.Errorf("Model = %s, want claude-opus-4", got.Model)
	}
}

func TestStore_RegisterInvalid(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Missing name
	agent := &AgentRegistration{
		Program: "claude-code",
		Status:  AgentStatusActive,
	}
	if err := store.Register(agent); err == nil {
		t.Error("Register() expected error for invalid agent")
	}
}

func TestStore_Unregister(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	agent := NewAgentRegistration("test-agent", "claude-opus-4", "claude-code")
	store.Register(agent)

	if err := store.Unregister("test-agent"); err != nil {
		t.Fatalf("Unregister() error = %v", err)
	}

	if got := store.Get("test-agent"); got != nil {
		t.Error("Get() after Unregister() should return nil")
	}
}

func TestStore_UnregisterNotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	if err := store.Unregister("nonexistent"); err == nil {
		t.Error("Unregister() expected error for nonexistent agent")
	}
}

func TestStore_List(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	store.Register(NewAgentRegistration("agent-1", "model", "program"))
	store.Register(NewAgentRegistration("agent-2", "model", "program"))
	store.Register(NewAgentRegistration("agent-3", "model", "program"))

	agents := store.List()
	if len(agents) != 3 {
		t.Errorf("List() returned %d agents, want 3", len(agents))
	}
}

func TestStore_ActiveAgents(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Register active agent
	active := NewAgentRegistration("active", "model", "program")
	store.Register(active)

	// Register inactive agent (manually set old LastSeen)
	inactive := NewAgentRegistration("inactive", "model", "program")
	store.Register(inactive)
	store.agents["inactive"].LastSeen = time.Now().Add(-1 * time.Hour)

	agents := store.ActiveAgents(30 * time.Minute)
	if len(agents) != 1 {
		t.Errorf("ActiveAgents() returned %d agents, want 1", len(agents))
	}
	if agents[0].Name != "active" {
		t.Errorf("Active agent name = %s, want active", agents[0].Name)
	}
}

func TestStore_Heartbeat(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	agent := NewAgentRegistration("test-agent", "model", "program")
	store.Register(agent)

	// Set old LastSeen
	oldTime := time.Now().Add(-1 * time.Hour)
	store.agents["test-agent"].LastSeen = oldTime
	store.agents["test-agent"].Status = AgentStatusInactive

	if err := store.Heartbeat("test-agent"); err != nil {
		t.Fatalf("Heartbeat() error = %v", err)
	}

	got := store.Get("test-agent")
	if got.LastSeen.Before(oldTime.Add(time.Second)) {
		t.Error("Heartbeat() did not update LastSeen")
	}
	if got.Status != AgentStatusActive {
		t.Errorf("Heartbeat() Status = %s, want active", got.Status)
	}
}

func TestStore_HeartbeatNotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	if err := store.Heartbeat("nonexistent"); err == nil {
		t.Error("Heartbeat() expected error for nonexistent agent")
	}
}

func TestStore_ClaimAndRelease(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	agent := NewAgentRegistration("test-agent", "model", "program")
	store.Register(agent)

	claim := NewWorkClaim("bv-42", "test-agent", ClaimReasonImplementing)
	if err := store.Claim("test-agent", claim); err != nil {
		t.Fatalf("Claim() error = %v", err)
	}

	got := store.Get("test-agent")
	if len(got.ClaimedWork) != 1 {
		t.Errorf("ClaimedWork length = %d, want 1", len(got.ClaimedWork))
	}
	if got.ClaimedWork[0].BeadID != "bv-42" {
		t.Errorf("Claimed BeadID = %s, want bv-42", got.ClaimedWork[0].BeadID)
	}

	// Release
	if err := store.Release("test-agent", "bv-42"); err != nil {
		t.Fatalf("Release() error = %v", err)
	}

	got = store.Get("test-agent")
	if len(got.ClaimedWork) != 0 {
		t.Errorf("ClaimedWork length after release = %d, want 0", len(got.ClaimedWork))
	}
}

func TestStore_ClaimConflict(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	agent1 := NewAgentRegistration("agent-1", "model", "program")
	agent2 := NewAgentRegistration("agent-2", "model", "program")
	store.Register(agent1)
	store.Register(agent2)

	claim1 := NewWorkClaim("bv-42", "agent-1", ClaimReasonImplementing)
	if err := store.Claim("agent-1", claim1); err != nil {
		t.Fatalf("Claim() by agent-1 error = %v", err)
	}

	claim2 := NewWorkClaim("bv-42", "agent-2", ClaimReasonImplementing)
	if err := store.Claim("agent-2", claim2); err == nil {
		t.Error("Claim() by agent-2 expected conflict error")
	}
}

func TestStore_GetClaimHolder(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	agent := NewAgentRegistration("test-agent", "model", "program")
	store.Register(agent)

	claim := NewWorkClaim("bv-42", "test-agent", ClaimReasonImplementing)
	store.Claim("test-agent", claim)

	holder := store.GetClaimHolder("bv-42")
	if holder == nil {
		t.Fatal("GetClaimHolder() returned nil")
	}
	if holder.Name != "test-agent" {
		t.Errorf("Claim holder = %s, want test-agent", holder.Name)
	}

	// Nonexistent claim
	holder = store.GetClaimHolder("bv-99")
	if holder != nil {
		t.Error("GetClaimHolder() for nonexistent claim should return nil")
	}
}

func TestStore_GetAllClaims(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	agent1 := NewAgentRegistration("agent-1", "model", "program")
	agent2 := NewAgentRegistration("agent-2", "model", "program")
	store.Register(agent1)
	store.Register(agent2)

	store.Claim("agent-1", NewWorkClaim("bv-1", "agent-1", ClaimReasonImplementing))
	store.Claim("agent-1", NewWorkClaim("bv-2", "agent-1", ClaimReasonTesting))
	store.Claim("agent-2", NewWorkClaim("bv-3", "agent-2", ClaimReasonReviewing))

	claims := store.GetAllClaims()
	if len(claims) != 3 {
		t.Errorf("GetAllClaims() returned %d claims, want 3", len(claims))
	}
}

func TestStore_FileHints(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	agent := NewAgentRegistration("test-agent", "model", "program")
	store.Register(agent)

	hint := NewFileHint("pkg/agent/*.go", "test-agent", "bv-42")
	if err := store.AddFileHint("test-agent", hint); err != nil {
		t.Fatalf("AddFileHint() error = %v", err)
	}

	got := store.Get("test-agent")
	if len(got.FileHints) != 1 {
		t.Errorf("FileHints length = %d, want 1", len(got.FileHints))
	}

	// Remove
	if err := store.RemoveFileHint("test-agent", "pkg/agent/*.go"); err != nil {
		t.Fatalf("RemoveFileHint() error = %v", err)
	}

	got = store.Get("test-agent")
	if len(got.FileHints) != 0 {
		t.Errorf("FileHints length after remove = %d, want 0", len(got.FileHints))
	}
}

func TestStore_GetFileConflicts(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	agent1 := NewAgentRegistration("agent-1", "model", "program")
	agent2 := NewAgentRegistration("agent-2", "model", "program")
	store.Register(agent1)
	store.Register(agent2)

	// Both agents work on same file
	store.AddFileHint("agent-1", NewFileHint("main.go", "agent-1", "bv-1"))
	store.AddFileHint("agent-2", NewFileHint("main.go", "agent-2", "bv-2"))

	conflicts := store.GetFileConflicts()
	if len(conflicts) != 1 {
		t.Errorf("GetFileConflicts() returned %d conflicts, want 1", len(conflicts))
	}
	if len(conflicts[0].Agents) != 2 {
		t.Errorf("Conflict has %d agents, want 2", len(conflicts[0].Agents))
	}
}

func TestStore_CleanupExpired(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	agent := NewAgentRegistration("test-agent", "model", "program")
	store.Register(agent)

	// Add expired claim
	expiredClaim := &WorkClaim{
		BeadID:    "bv-old",
		Agent:     "test-agent",
		ClaimedAt: time.Now().Add(-2 * time.Hour),
		ExpiresAt: time.Now().Add(-1 * time.Hour),
		Reason:    ClaimReasonImplementing,
	}
	store.agents["test-agent"].ClaimedWork = append(
		store.agents["test-agent"].ClaimedWork,
		*expiredClaim,
	)

	// Add valid claim
	validClaim := NewWorkClaim("bv-new", "test-agent", ClaimReasonImplementing)
	store.agents["test-agent"].ClaimedWork = append(
		store.agents["test-agent"].ClaimedWork,
		*validClaim,
	)

	store.CleanupExpired()

	got := store.Get("test-agent")
	if len(got.ClaimedWork) != 1 {
		t.Errorf("ClaimedWork length after cleanup = %d, want 1", len(got.ClaimedWork))
	}
	if got.ClaimedWork[0].BeadID != "bv-new" {
		t.Errorf("Remaining claim = %s, want bv-new", got.ClaimedWork[0].BeadID)
	}
}

func TestStore_SaveAndLoad(t *testing.T) {
	dir := t.TempDir()

	// Create and save
	store1 := NewStore(dir)
	agent := NewAgentRegistration("test-agent", "claude-opus-4", "claude-code")
	store1.Register(agent)
	store1.Claim("test-agent", NewWorkClaim("bv-42", "test-agent", ClaimReasonImplementing))

	if err := store1.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Check file exists
	path := filepath.Join(dir, "agents.jsonl")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("agents.jsonl not created")
	}

	// Load in new store
	store2 := NewStore(dir)
	if err := store2.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	got := store2.Get("test-agent")
	if got == nil {
		t.Fatal("Get() after Load() returned nil")
	}
	if got.Name != "test-agent" {
		t.Errorf("Loaded agent name = %s, want test-agent", got.Name)
	}
	if len(got.ClaimedWork) != 1 {
		t.Errorf("Loaded claims = %d, want 1", len(got.ClaimedWork))
	}
}

func TestStore_LoadEmpty(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	// Load from nonexistent file should not error
	if err := store.Load(); err != nil {
		t.Fatalf("Load() on empty dir error = %v", err)
	}

	if len(store.List()) != 0 {
		t.Error("Load() on empty dir should have no agents")
	}
}

func TestStore_Summary(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	agent := NewAgentRegistration("test-agent", "model", "program")
	store.Register(agent)
	store.Claim("test-agent", NewWorkClaim("bv-42", "test-agent", ClaimReasonImplementing))

	summaries := store.Summary()
	if len(summaries) != 1 {
		t.Errorf("Summary() returned %d summaries, want 1", len(summaries))
	}
	if summaries[0].ClaimCount != 1 {
		t.Errorf("ClaimCount = %d, want 1", summaries[0].ClaimCount)
	}
	if summaries[0].LastSeenAgo == "" {
		t.Error("LastSeenAgo is empty")
	}
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "just now"},
		{5 * time.Minute, "5 minutes ago"},
		{1 * time.Minute, "1 minute ago"},
		{1 * time.Hour, "1 hour ago"},
		{3 * time.Hour, "3 hours ago"},
		{24 * time.Hour, "1 day ago"},
		{72 * time.Hour, "3 days ago"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatDuration(tt.d)
			if got != tt.want {
				t.Errorf("formatDuration(%v) = %s, want %s", tt.d, got, tt.want)
			}
		})
	}
}
