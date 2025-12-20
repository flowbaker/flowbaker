package types

import "time"

// PlanStatus represents the current state of a plan
type PlanStatus string

const (
	PlanStatusActive    PlanStatus = "active"
	PlanStatusCompleted PlanStatus = "completed"
	PlanStatusFailed    PlanStatus = "failed"
)

// PlanStepStatus represents the current state of a plan step
type PlanStepStatus string

const (
	PlanStepStatusPending    PlanStepStatus = "pending"
	PlanStepStatusInProgress PlanStepStatus = "in_progress"
	PlanStepStatusCompleted  PlanStepStatus = "completed"
	PlanStepStatusFailed     PlanStepStatus = "failed"
)

// PlanStep represents a single step in a plan
type PlanStep struct {
	ID          string         `json:"id"`
	Description string         `json:"description"`
	Status      PlanStepStatus `json:"status"`
	Result      string         `json:"result,omitempty"`
	Error       string         `json:"error,omitempty"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

// Plan represents a structured plan with multiple steps
type Plan struct {
	ID          string     `json:"id"`
	Goal        string     `json:"goal"`
	Steps       []PlanStep `json:"steps"`
	Status      PlanStatus `json:"status"`
	CurrentStep int        `json:"current_step"` // Index of current step
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// GetCurrentStep returns the current step being executed, or nil if none
func (p *Plan) GetCurrentStep() *PlanStep {
	if p.CurrentStep >= 0 && p.CurrentStep < len(p.Steps) {
		return &p.Steps[p.CurrentStep]
	}
	return nil
}

// GetNextPendingStep returns the next pending step, or nil if none
func (p *Plan) GetNextPendingStep() *PlanStep {
	for i := range p.Steps {
		if p.Steps[i].Status == PlanStepStatusPending {
			return &p.Steps[i]
		}
	}
	return nil
}

// IsComplete returns true if all steps are completed
func (p *Plan) IsComplete() bool {
	for _, step := range p.Steps {
		if step.Status != PlanStepStatusCompleted {
			return false
		}
	}
	return true
}

// HasFailedSteps returns true if any step has failed
func (p *Plan) HasFailedSteps() bool {
	for _, step := range p.Steps {
		if step.Status == PlanStepStatusFailed {
			return true
		}
	}
	return false
}

// GetStepByID finds a step by its ID
func (p *Plan) GetStepByID(id string) *PlanStep {
	for i := range p.Steps {
		if p.Steps[i].ID == id {
			return &p.Steps[i]
		}
	}
	return nil
}
