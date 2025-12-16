// Package agent provides data types and utilities for the Agent Swarm Protocol.
// This enables multiple AI agents to work on the same project without conflicts
// by tracking agent registrations, work claims, and file reservations.
package agent

import (
	"fmt"
	"time"
)

// ============================================================================
// Core Types
// ============================================================================

// AgentRegistration represents an AI agent working on a project.
// Stored in .beads/agents.jsonl with one line per agent.
type AgentRegistration struct {
	Name        string       `json:"name"`         // Unique identifier (e.g., "claude-opus-1")
	Model       string       `json:"model"`        // Model name (e.g., "claude-opus-4")
	Program     string       `json:"program"`      // Client program (e.g., "claude-code", "cursor", "devin")
	StartedAt   time.Time    `json:"started_at"`   // When the agent session started
	LastSeen    time.Time    `json:"last_seen"`    // Last heartbeat timestamp
	ClaimedWork []WorkClaim  `json:"claimed_work"` // Currently claimed beads
	FileHints   []FileHint   `json:"file_hints"`   // Files being worked on
	Status      AgentStatus  `json:"status"`       // Current agent status
	Metadata    AgentMeta    `json:"metadata,omitempty"`
}

// Validate checks if the agent registration is valid
func (a *AgentRegistration) Validate() error {
	if a.Name == "" {
		return fmt.Errorf("agent name cannot be empty")
	}
	if a.Program == "" {
		return fmt.Errorf("agent program cannot be empty")
	}
	if !a.Status.IsValid() {
		return fmt.Errorf("invalid agent status: %s", a.Status)
	}
	return nil
}

// IsActive returns true if the agent has been seen recently (within timeout)
func (a *AgentRegistration) IsActive(timeout time.Duration) bool {
	return time.Since(a.LastSeen) < timeout
}

// IsStale returns true if the agent hasn't been seen for a long time (> 1 hour default)
func (a *AgentRegistration) IsStale(threshold time.Duration) bool {
	return time.Since(a.LastSeen) > threshold
}

// HasClaim returns true if the agent has claimed the specified bead
func (a *AgentRegistration) HasClaim(beadID string) bool {
	for _, claim := range a.ClaimedWork {
		if claim.BeadID == beadID {
			return true
		}
	}
	return false
}

// ActiveClaims returns claims that haven't expired
func (a *AgentRegistration) ActiveClaims(now time.Time) []WorkClaim {
	var active []WorkClaim
	for _, claim := range a.ClaimedWork {
		if !claim.IsExpired(now) {
			active = append(active, claim)
		}
	}
	return active
}

// WorkClaim represents a claim on a bead by an agent.
// Claims auto-expire after a period of inactivity.
type WorkClaim struct {
	BeadID    string        `json:"bead_id"`    // The claimed bead (e.g., "bv-42")
	Agent     string        `json:"agent"`      // Agent name who owns the claim
	ClaimedAt time.Time     `json:"claimed_at"` // When the claim was made
	ExpiresAt time.Time     `json:"expires_at"` // Auto-release after inactivity
	Reason    ClaimReason   `json:"reason"`     // Why the claim was made
	Notes     string        `json:"notes,omitempty"` // Optional context
}

// IsExpired returns true if the claim has passed its expiry time
func (c *WorkClaim) IsExpired(now time.Time) bool {
	return now.After(c.ExpiresAt)
}

// Validate checks if the work claim is valid
func (c *WorkClaim) Validate() error {
	if c.BeadID == "" {
		return fmt.Errorf("bead ID cannot be empty")
	}
	if c.Agent == "" {
		return fmt.Errorf("agent name cannot be empty")
	}
	if c.ClaimedAt.IsZero() {
		return fmt.Errorf("claimed_at cannot be zero")
	}
	if c.ExpiresAt.IsZero() {
		return fmt.Errorf("expires_at cannot be zero")
	}
	if !c.Reason.IsValid() {
		return fmt.Errorf("invalid claim reason: %s", c.Reason)
	}
	return nil
}

// TimeToExpiry returns the duration until the claim expires
func (c *WorkClaim) TimeToExpiry(now time.Time) time.Duration {
	return c.ExpiresAt.Sub(now)
}

// FileHint represents a file or pattern an agent is working on.
// These are advisory, not locks.
type FileHint struct {
	Pattern   string    `json:"pattern"`    // File path or glob pattern
	Agent     string    `json:"agent"`      // Agent who registered the hint
	BeadID    string    `json:"bead_id,omitempty"` // Associated bead
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// IsExpired returns true if the file hint has passed its expiry time
func (f *FileHint) IsExpired(now time.Time) bool {
	return now.After(f.ExpiresAt)
}

// AgentMeta holds optional metadata about an agent
type AgentMeta struct {
	Version   string `json:"version,omitempty"`    // Agent/client version
	SessionID string `json:"session_id,omitempty"` // Unique session identifier
	Tags      []string `json:"tags,omitempty"`     // Custom tags
}

// ============================================================================
// Enums and Constants
// ============================================================================

// AgentStatus represents the current status of an agent
type AgentStatus string

const (
	AgentStatusActive   AgentStatus = "active"    // Agent is actively working
	AgentStatusIdle     AgentStatus = "idle"      // Agent is registered but not actively working
	AgentStatusInactive AgentStatus = "inactive"  // Agent hasn't been seen recently
	AgentStatusGone     AgentStatus = "gone"      // Agent has explicitly unregistered
)

// IsValid returns true if the agent status is a recognized value
func (s AgentStatus) IsValid() bool {
	switch s {
	case AgentStatusActive, AgentStatusIdle, AgentStatusInactive, AgentStatusGone:
		return true
	}
	return false
}

// ClaimReason describes why a work claim was made
type ClaimReason string

const (
	ClaimReasonImplementing ClaimReason = "implementing" // Writing code
	ClaimReasonReviewing    ClaimReason = "reviewing"    // Code review
	ClaimReasonTesting      ClaimReason = "testing"      // Writing/running tests
	ClaimReasonDebugging    ClaimReason = "debugging"    // Investigating issue
	ClaimReasonPlanning     ClaimReason = "planning"     // Planning implementation
)

// IsValid returns true if the claim reason is a recognized value
func (r ClaimReason) IsValid() bool {
	switch r {
	case ClaimReasonImplementing, ClaimReasonReviewing, ClaimReasonTesting,
		ClaimReasonDebugging, ClaimReasonPlanning:
		return true
	}
	return false
}

// ============================================================================
// Configuration Constants
// ============================================================================

const (
	// DefaultClaimExpiry is the default time until a claim auto-expires
	DefaultClaimExpiry = 30 * time.Minute

	// DefaultHeartbeatInterval is the recommended interval for agent heartbeats
	DefaultHeartbeatInterval = 5 * time.Minute

	// DefaultInactivityThreshold is when an agent is considered inactive
	DefaultInactivityThreshold = 30 * time.Minute

	// DefaultStaleThreshold is when an agent is considered stale/gone
	DefaultStaleThreshold = 1 * time.Hour

	// DefaultFileHintExpiry is the default time until file hints expire
	DefaultFileHintExpiry = 30 * time.Minute
)

// ============================================================================
// Helper Types
// ============================================================================

// AgentSummary provides a quick overview of an agent's state
type AgentSummary struct {
	Name           string        `json:"name"`
	Program        string        `json:"program"`
	Status         AgentStatus   `json:"status"`
	ClaimCount     int           `json:"claim_count"`
	ActiveClaims   []string      `json:"active_claims"` // Bead IDs
	LastSeen       time.Time     `json:"last_seen"`
	LastSeenAgo    string        `json:"last_seen_ago"` // Human-readable
}

// ClaimConflict represents a conflict when multiple agents claim the same work
type ClaimConflict struct {
	BeadID      string   `json:"bead_id"`
	Claimants   []string `json:"claimants"`  // Agent names
	FirstClaim  time.Time `json:"first_claim"`
	Resolution  string   `json:"resolution,omitempty"` // Suggested resolution
}

// FileConflict represents potential file conflicts between agents
type FileConflict struct {
	File       string   `json:"file"`       // File path or pattern
	Agents     []string `json:"agents"`     // Agents touching this file
	BeadIDs    []string `json:"bead_ids"`   // Associated beads
	Resolution string   `json:"resolution,omitempty"`
}

// ============================================================================
// Factory Functions
// ============================================================================

// NewAgentRegistration creates a new agent registration with sensible defaults
func NewAgentRegistration(name, model, program string) *AgentRegistration {
	now := time.Now()
	return &AgentRegistration{
		Name:        name,
		Model:       model,
		Program:     program,
		StartedAt:   now,
		LastSeen:    now,
		ClaimedWork: []WorkClaim{},
		FileHints:   []FileHint{},
		Status:      AgentStatusActive,
	}
}

// NewWorkClaim creates a new work claim with default expiry
func NewWorkClaim(beadID, agent string, reason ClaimReason) *WorkClaim {
	now := time.Now()
	return &WorkClaim{
		BeadID:    beadID,
		Agent:     agent,
		ClaimedAt: now,
		ExpiresAt: now.Add(DefaultClaimExpiry),
		Reason:    reason,
	}
}

// NewFileHint creates a new file hint with default expiry
func NewFileHint(pattern, agent string, beadID string) *FileHint {
	now := time.Now()
	return &FileHint{
		Pattern:   pattern,
		Agent:     agent,
		BeadID:    beadID,
		CreatedAt: now,
		ExpiresAt: now.Add(DefaultFileHintExpiry),
	}
}
