package agent

import (
	"testing"
	"time"
)

func TestAgentRegistration_Validate(t *testing.T) {
	tests := []struct {
		name    string
		agent   AgentRegistration
		wantErr bool
	}{
		{
			name: "valid agent",
			agent: AgentRegistration{
				Name:    "test-agent",
				Program: "claude-code",
				Status:  AgentStatusActive,
			},
			wantErr: false,
		},
		{
			name: "missing name",
			agent: AgentRegistration{
				Program: "claude-code",
				Status:  AgentStatusActive,
			},
			wantErr: true,
		},
		{
			name: "missing program",
			agent: AgentRegistration{
				Name:   "test-agent",
				Status: AgentStatusActive,
			},
			wantErr: true,
		},
		{
			name: "invalid status",
			agent: AgentRegistration{
				Name:    "test-agent",
				Program: "claude-code",
				Status:  "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.agent.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAgentRegistration_IsActive(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		lastSeen time.Time
		timeout  time.Duration
		want     bool
	}{
		{
			name:     "recently seen",
			lastSeen: now.Add(-5 * time.Minute),
			timeout:  30 * time.Minute,
			want:     true,
		},
		{
			name:     "just now",
			lastSeen: now,
			timeout:  30 * time.Minute,
			want:     true,
		},
		{
			name:     "inactive",
			lastSeen: now.Add(-1 * time.Hour),
			timeout:  30 * time.Minute,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := &AgentRegistration{LastSeen: tt.lastSeen}
			if got := agent.IsActive(tt.timeout); got != tt.want {
				t.Errorf("IsActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAgentRegistration_IsStale(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		lastSeen  time.Time
		threshold time.Duration
		want      bool
	}{
		{
			name:      "recently seen",
			lastSeen:  now.Add(-5 * time.Minute),
			threshold: 1 * time.Hour,
			want:      false,
		},
		{
			name:      "stale",
			lastSeen:  now.Add(-2 * time.Hour),
			threshold: 1 * time.Hour,
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := &AgentRegistration{LastSeen: tt.lastSeen}
			if got := agent.IsStale(tt.threshold); got != tt.want {
				t.Errorf("IsStale() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAgentRegistration_HasClaim(t *testing.T) {
	agent := &AgentRegistration{
		Name: "test",
		ClaimedWork: []WorkClaim{
			{BeadID: "bv-1"},
			{BeadID: "bv-2"},
		},
	}

	if !agent.HasClaim("bv-1") {
		t.Error("HasClaim(bv-1) = false, want true")
	}
	if agent.HasClaim("bv-3") {
		t.Error("HasClaim(bv-3) = true, want false")
	}
}

func TestAgentRegistration_ActiveClaims(t *testing.T) {
	now := time.Now()
	agent := &AgentRegistration{
		Name: "test",
		ClaimedWork: []WorkClaim{
			{BeadID: "bv-1", ExpiresAt: now.Add(1 * time.Hour)},  // Active
			{BeadID: "bv-2", ExpiresAt: now.Add(-1 * time.Hour)}, // Expired
			{BeadID: "bv-3", ExpiresAt: now.Add(30 * time.Minute)}, // Active
		},
	}

	active := agent.ActiveClaims(now)
	if len(active) != 2 {
		t.Errorf("ActiveClaims() returned %d claims, want 2", len(active))
	}

	// Check the right ones are active
	activeIDs := make(map[string]bool)
	for _, c := range active {
		activeIDs[c.BeadID] = true
	}
	if !activeIDs["bv-1"] || !activeIDs["bv-3"] {
		t.Error("ActiveClaims() returned wrong claims")
	}
}

func TestWorkClaim_Validate(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		claim   WorkClaim
		wantErr bool
	}{
		{
			name: "valid claim",
			claim: WorkClaim{
				BeadID:    "bv-1",
				Agent:     "test-agent",
				ClaimedAt: now,
				ExpiresAt: now.Add(30 * time.Minute),
				Reason:    ClaimReasonImplementing,
			},
			wantErr: false,
		},
		{
			name: "missing bead ID",
			claim: WorkClaim{
				Agent:     "test-agent",
				ClaimedAt: now,
				ExpiresAt: now.Add(30 * time.Minute),
				Reason:    ClaimReasonImplementing,
			},
			wantErr: true,
		},
		{
			name: "missing agent",
			claim: WorkClaim{
				BeadID:    "bv-1",
				ClaimedAt: now,
				ExpiresAt: now.Add(30 * time.Minute),
				Reason:    ClaimReasonImplementing,
			},
			wantErr: true,
		},
		{
			name: "invalid reason",
			claim: WorkClaim{
				BeadID:    "bv-1",
				Agent:     "test-agent",
				ClaimedAt: now,
				ExpiresAt: now.Add(30 * time.Minute),
				Reason:    "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.claim.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWorkClaim_IsExpired(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{
			name:      "not expired",
			expiresAt: now.Add(1 * time.Hour),
			want:      false,
		},
		{
			name:      "expired",
			expiresAt: now.Add(-1 * time.Hour),
			want:      true,
		},
		{
			name:      "just expired",
			expiresAt: now.Add(-1 * time.Second),
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			claim := &WorkClaim{ExpiresAt: tt.expiresAt}
			if got := claim.IsExpired(now); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWorkClaim_TimeToExpiry(t *testing.T) {
	now := time.Now()
	claim := &WorkClaim{
		ExpiresAt: now.Add(30 * time.Minute),
	}

	tte := claim.TimeToExpiry(now)
	if tte < 29*time.Minute || tte > 31*time.Minute {
		t.Errorf("TimeToExpiry() = %v, want ~30m", tte)
	}
}

func TestFileHint_IsExpired(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name      string
		expiresAt time.Time
		want      bool
	}{
		{
			name:      "not expired",
			expiresAt: now.Add(1 * time.Hour),
			want:      false,
		},
		{
			name:      "expired",
			expiresAt: now.Add(-1 * time.Hour),
			want:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint := &FileHint{ExpiresAt: tt.expiresAt}
			if got := hint.IsExpired(now); got != tt.want {
				t.Errorf("IsExpired() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAgentStatus_IsValid(t *testing.T) {
	tests := []struct {
		status AgentStatus
		want   bool
	}{
		{AgentStatusActive, true},
		{AgentStatusIdle, true},
		{AgentStatusInactive, true},
		{AgentStatusGone, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClaimReason_IsValid(t *testing.T) {
	tests := []struct {
		reason ClaimReason
		want   bool
	}{
		{ClaimReasonImplementing, true},
		{ClaimReasonReviewing, true},
		{ClaimReasonTesting, true},
		{ClaimReasonDebugging, true},
		{ClaimReasonPlanning, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(string(tt.reason), func(t *testing.T) {
			if got := tt.reason.IsValid(); got != tt.want {
				t.Errorf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewAgentRegistration(t *testing.T) {
	agent := NewAgentRegistration("test-agent", "claude-opus-4", "claude-code")

	if agent.Name != "test-agent" {
		t.Errorf("Name = %s, want test-agent", agent.Name)
	}
	if agent.Model != "claude-opus-4" {
		t.Errorf("Model = %s, want claude-opus-4", agent.Model)
	}
	if agent.Program != "claude-code" {
		t.Errorf("Program = %s, want claude-code", agent.Program)
	}
	if agent.Status != AgentStatusActive {
		t.Errorf("Status = %s, want active", agent.Status)
	}
	if agent.StartedAt.IsZero() {
		t.Error("StartedAt is zero")
	}
	if agent.LastSeen.IsZero() {
		t.Error("LastSeen is zero")
	}
}

func TestNewWorkClaim(t *testing.T) {
	claim := NewWorkClaim("bv-42", "test-agent", ClaimReasonImplementing)

	if claim.BeadID != "bv-42" {
		t.Errorf("BeadID = %s, want bv-42", claim.BeadID)
	}
	if claim.Agent != "test-agent" {
		t.Errorf("Agent = %s, want test-agent", claim.Agent)
	}
	if claim.Reason != ClaimReasonImplementing {
		t.Errorf("Reason = %s, want implementing", claim.Reason)
	}
	if claim.ClaimedAt.IsZero() {
		t.Error("ClaimedAt is zero")
	}
	if claim.ExpiresAt.IsZero() {
		t.Error("ExpiresAt is zero")
	}
	if claim.ExpiresAt.Before(claim.ClaimedAt) {
		t.Error("ExpiresAt before ClaimedAt")
	}

	// Check default expiry is ~30 minutes
	expiry := claim.ExpiresAt.Sub(claim.ClaimedAt)
	if expiry < 29*time.Minute || expiry > 31*time.Minute {
		t.Errorf("Expiry = %v, want ~30m", expiry)
	}
}

func TestNewFileHint(t *testing.T) {
	hint := NewFileHint("pkg/agent/*.go", "test-agent", "bv-42")

	if hint.Pattern != "pkg/agent/*.go" {
		t.Errorf("Pattern = %s, want pkg/agent/*.go", hint.Pattern)
	}
	if hint.Agent != "test-agent" {
		t.Errorf("Agent = %s, want test-agent", hint.Agent)
	}
	if hint.BeadID != "bv-42" {
		t.Errorf("BeadID = %s, want bv-42", hint.BeadID)
	}
	if hint.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero")
	}
	if hint.ExpiresAt.IsZero() {
		t.Error("ExpiresAt is zero")
	}
}
