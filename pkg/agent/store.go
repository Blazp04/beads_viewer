package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Store manages agent registrations in .beads/agents.jsonl
type Store struct {
	beadsDir string
	mu       sync.RWMutex
	agents   map[string]*AgentRegistration // keyed by agent name
}

// NewStore creates a new agent store for the given beads directory
func NewStore(beadsDir string) *Store {
	return &Store{
		beadsDir: beadsDir,
		agents:   make(map[string]*AgentRegistration),
	}
}

// AgentsFilePath returns the path to agents.jsonl
func (s *Store) AgentsFilePath() string {
	return filepath.Join(s.beadsDir, "agents.jsonl")
}

// Load reads all agent registrations from disk
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.agents = make(map[string]*AgentRegistration)

	path := s.AgentsFilePath()
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		// No agents file yet - that's fine
		return nil
	}
	if err != nil {
		return fmt.Errorf("opening agents file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var agent AgentRegistration
		if err := json.Unmarshal(line, &agent); err != nil {
			return fmt.Errorf("parsing agent at line %d: %w", lineNum, err)
		}

		s.agents[agent.Name] = &agent
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading agents file: %w", err)
	}

	return nil
}

// Save writes all agent registrations to disk
func (s *Store) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Ensure directory exists
	if err := os.MkdirAll(s.beadsDir, 0755); err != nil {
		return fmt.Errorf("creating beads directory: %w", err)
	}

	path := s.AgentsFilePath()
	tmpPath := path + ".tmp"

	file, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}

	encoder := json.NewEncoder(file)
	for _, agent := range s.agents {
		if err := encoder.Encode(agent); err != nil {
			file.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("encoding agent %s: %w", agent.Name, err)
		}
	}

	if err := file.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

// Register adds or updates an agent registration
func (s *Store) Register(agent *AgentRegistration) error {
	if err := agent.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Update LastSeen for existing agents
	if existing, ok := s.agents[agent.Name]; ok {
		agent.StartedAt = existing.StartedAt // Preserve original start time
		if agent.ClaimedWork == nil {
			agent.ClaimedWork = existing.ClaimedWork
		}
		if agent.FileHints == nil {
			agent.FileHints = existing.FileHints
		}
	}

	agent.LastSeen = time.Now()
	s.agents[agent.Name] = agent
	return nil
}

// Unregister removes an agent registration
func (s *Store) Unregister(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.agents[name]; !ok {
		return fmt.Errorf("agent %s not found", name)
	}

	delete(s.agents, name)
	return nil
}

// Get returns an agent by name
func (s *Store) Get(name string) *AgentRegistration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if agent, ok := s.agents[name]; ok {
		// Return a copy to prevent mutation
		copy := *agent
		return &copy
	}
	return nil
}

// List returns all registered agents
func (s *Store) List() []*AgentRegistration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*AgentRegistration, 0, len(s.agents))
	for _, agent := range s.agents {
		copy := *agent
		result = append(result, &copy)
	}
	return result
}

// ActiveAgents returns agents that have been seen recently
func (s *Store) ActiveAgents(timeout time.Duration) []*AgentRegistration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*AgentRegistration
	for _, agent := range s.agents {
		if agent.IsActive(timeout) {
			copy := *agent
			result = append(result, &copy)
		}
	}
	return result
}

// Heartbeat updates an agent's LastSeen timestamp
func (s *Store) Heartbeat(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, ok := s.agents[name]
	if !ok {
		return fmt.Errorf("agent %s not found", name)
	}

	agent.LastSeen = time.Now()
	agent.Status = AgentStatusActive
	return nil
}

// Claim adds a work claim for an agent
func (s *Store) Claim(agentName string, claim *WorkClaim) error {
	if err := claim.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	agent, ok := s.agents[agentName]
	if !ok {
		return fmt.Errorf("agent %s not found", agentName)
	}

	// Check for conflicts
	for _, other := range s.agents {
		if other.Name == agentName {
			continue
		}
		for _, existingClaim := range other.ClaimedWork {
			if existingClaim.BeadID == claim.BeadID && !existingClaim.IsExpired(time.Now()) {
				return fmt.Errorf("bead %s already claimed by %s", claim.BeadID, other.Name)
			}
		}
	}

	// Remove any existing claim on this bead by this agent
	var newClaims []WorkClaim
	for _, c := range agent.ClaimedWork {
		if c.BeadID != claim.BeadID {
			newClaims = append(newClaims, c)
		}
	}
	newClaims = append(newClaims, *claim)
	agent.ClaimedWork = newClaims
	agent.LastSeen = time.Now()

	return nil
}

// Release removes a work claim from an agent
func (s *Store) Release(agentName, beadID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, ok := s.agents[agentName]
	if !ok {
		return fmt.Errorf("agent %s not found", agentName)
	}

	var newClaims []WorkClaim
	found := false
	for _, c := range agent.ClaimedWork {
		if c.BeadID == beadID {
			found = true
		} else {
			newClaims = append(newClaims, c)
		}
	}

	if !found {
		return fmt.Errorf("claim for bead %s not found", beadID)
	}

	agent.ClaimedWork = newClaims
	agent.LastSeen = time.Now()
	return nil
}

// GetClaimHolder returns the agent holding a claim on a bead, if any
func (s *Store) GetClaimHolder(beadID string) *AgentRegistration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	for _, agent := range s.agents {
		for _, claim := range agent.ClaimedWork {
			if claim.BeadID == beadID && !claim.IsExpired(now) {
				copy := *agent
				return &copy
			}
		}
	}
	return nil
}

// GetAllClaims returns all active work claims across all agents
func (s *Store) GetAllClaims() []WorkClaim {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	var claims []WorkClaim
	for _, agent := range s.agents {
		for _, claim := range agent.ClaimedWork {
			if !claim.IsExpired(now) {
				claims = append(claims, claim)
			}
		}
	}
	return claims
}

// AddFileHint adds a file hint for an agent
func (s *Store) AddFileHint(agentName string, hint *FileHint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, ok := s.agents[agentName]
	if !ok {
		return fmt.Errorf("agent %s not found", agentName)
	}

	agent.FileHints = append(agent.FileHints, *hint)
	agent.LastSeen = time.Now()
	return nil
}

// RemoveFileHint removes a file hint from an agent
func (s *Store) RemoveFileHint(agentName, pattern string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	agent, ok := s.agents[agentName]
	if !ok {
		return fmt.Errorf("agent %s not found", agentName)
	}

	var newHints []FileHint
	for _, h := range agent.FileHints {
		if h.Pattern != pattern {
			newHints = append(newHints, h)
		}
	}
	agent.FileHints = newHints
	agent.LastSeen = time.Now()
	return nil
}

// GetFileConflicts returns potential file conflicts between agents
func (s *Store) GetFileConflicts() []FileConflict {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	fileAgents := make(map[string][]string) // file -> agents
	fileBeads := make(map[string][]string)  // file -> beads

	for _, agent := range s.agents {
		if !agent.IsActive(DefaultInactivityThreshold) {
			continue
		}
		for _, hint := range agent.FileHints {
			if hint.IsExpired(now) {
				continue
			}
			fileAgents[hint.Pattern] = append(fileAgents[hint.Pattern], agent.Name)
			if hint.BeadID != "" {
				fileBeads[hint.Pattern] = append(fileBeads[hint.Pattern], hint.BeadID)
			}
		}
	}

	var conflicts []FileConflict
	for file, agents := range fileAgents {
		if len(agents) > 1 {
			conflicts = append(conflicts, FileConflict{
				File:    file,
				Agents:  agents,
				BeadIDs: fileBeads[file],
			})
		}
	}
	return conflicts
}

// CleanupExpired removes expired claims and file hints, marks inactive agents
func (s *Store) CleanupExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, agent := range s.agents {
		// Clean expired claims
		var activeClaims []WorkClaim
		for _, claim := range agent.ClaimedWork {
			if !claim.IsExpired(now) {
				activeClaims = append(activeClaims, claim)
			}
		}
		agent.ClaimedWork = activeClaims

		// Clean expired file hints
		var activeHints []FileHint
		for _, hint := range agent.FileHints {
			if !hint.IsExpired(now) {
				activeHints = append(activeHints, hint)
			}
		}
		agent.FileHints = activeHints

		// Update agent status based on activity
		if agent.IsStale(DefaultStaleThreshold) {
			agent.Status = AgentStatusGone
		} else if !agent.IsActive(DefaultInactivityThreshold) {
			agent.Status = AgentStatusInactive
		}
	}
}

// Summary returns a summary of all agent activity
func (s *Store) Summary() []AgentSummary {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	var summaries []AgentSummary
	for _, agent := range s.agents {
		activeClaims := agent.ActiveClaims(now)
		claimIDs := make([]string, len(activeClaims))
		for i, c := range activeClaims {
			claimIDs[i] = c.BeadID
		}

		lastSeenAgo := formatDuration(now.Sub(agent.LastSeen))

		summaries = append(summaries, AgentSummary{
			Name:         agent.Name,
			Program:      agent.Program,
			Status:       agent.Status,
			ClaimCount:   len(activeClaims),
			ActiveClaims: claimIDs,
			LastSeen:     agent.LastSeen,
			LastSeenAgo:  lastSeenAgo,
		})
	}
	return summaries
}

// formatDuration formats a duration for human display
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return "just now"
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	}
	if d < 24*time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	}
	days := int(d.Hours() / 24)
	if days == 1 {
		return "1 day ago"
	}
	return fmt.Sprintf("%d days ago", days)
}
