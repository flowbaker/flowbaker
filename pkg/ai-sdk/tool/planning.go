package tool

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/flowbaker/flowbaker/pkg/ai-sdk/types"
	"github.com/google/uuid"
)

var (
	createPlanSchema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"goal": map[string]any{
				"type":        "string",
				"description": "The overall goal of this plan",
			},
			"steps": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"description": map[string]any{
							"type":        "string",
							"description": "Description of what this step should accomplish",
						},
					},
					"required": []string{"description"},
				},
				"description": "List of steps to accomplish the goal",
			},
		},
		"required": []string{"goal", "steps"},
	}

	startNextStepSchema = map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}

	completeCurrentStepSchema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"result": map[string]any{
				"type":        "string",
				"description": "Optional description of what was accomplished",
			},
		},
	}

	updatePlanSchema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"enum":        []string{"add_step", "remove_step"},
				"description": "What to do: add_step or remove_step",
			},
			"step_description": map[string]any{
				"type":        "string",
				"description": "For add_step: description of the new step",
			},
			"step_id": map[string]any{
				"type":        "string",
				"description": "For remove_step: ID of step to remove",
			},
			"position": map[string]any{
				"type":        "string",
				"description": "For add_step: where to insert (end, or before:step_id)",
			},
		},
		"required": []string{"action"},
	}
)

// PlanManager manages the active plan during agent execution
type PlanManager struct {
	activePlan   *types.Plan
	eventEmitter func(types.StreamEvent)
}

// NewPlanManager creates a new plan manager
func NewPlanManager() *PlanManager {
	return &PlanManager{}
}

// PlanningTool wraps a function and provides access to the PlanManager's event emitter
type PlanningTool struct {
	*FuncTool
	manager *PlanManager
}

// SetEventEmitter implements EventEmittingTool interface
func (pt *PlanningTool) SetEventEmitter(emitter func(types.StreamEvent)) {
	pt.manager.SetEventEmitter(emitter)
}

// newPlanningTool creates a PlanningTool with access to the PlanManager
func (pm *PlanManager) newPlanningTool(name, description string, parameters map[string]any, fn func(string) (string, error)) *PlanningTool {
	return &PlanningTool{
		FuncTool: &FuncTool{
			name:        name,
			description: description,
			parameters:  parameters,
			fn:          fn,
		},
		manager: pm,
	}
}

// SetEventEmitter sets the event emitter for plan events
func (pm *PlanManager) SetEventEmitter(emitter func(types.StreamEvent)) {
	pm.eventEmitter = emitter
}

// GetActivePlan returns the current active plan, or nil if none exists
func (pm *PlanManager) GetActivePlan() *types.Plan {
	return pm.activePlan
}

// CreatePlanTool returns a tool that allows AI to create a plan
func (pm *PlanManager) CreatePlanTool() Tool {
	return pm.newPlanningTool(
		"create_plan",
		"Create a structured plan with multiple steps to accomplish a goal. Use this when you need to break down a complex task into manageable steps.",
		createPlanSchema,
		func(argsJSON string) (string, error) {
			var args struct {
				Goal  string `json:"goal"`
				Steps []struct {
					Description string `json:"description"`
				} `json:"steps"`
			}

			if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}

			if args.Goal == "" {
				return "", fmt.Errorf("goal is required")
			}

			if len(args.Steps) == 0 {
				return "", fmt.Errorf("at least one step is required")
			}

			// Create plan
			plan := &types.Plan{
				ID:          uuid.New().String(),
				Goal:        args.Goal,
				Steps:       make([]types.PlanStep, len(args.Steps)),
				Status:      types.PlanStatusActive,
				CurrentStep: -1, // No step started yet
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}

			for i, step := range args.Steps {
				plan.Steps[i] = types.PlanStep{
					ID:          fmt.Sprintf("step_%d", i+1),
					Description: step.Description,
					Status:      types.PlanStepStatusPending,
				}
			}

			pm.activePlan = plan

			// Emit plan created event
			if pm.eventEmitter != nil {
				pm.eventEmitter(types.NewPlanCreatedEvent(*plan))
			}

			return fmt.Sprintf("Plan created with %d steps. Plan ID: %s", len(plan.Steps), plan.ID), nil
		},
	)
}

// StartNextStepTool returns a tool that allows AI to start the next pending step
func (pm *PlanManager) StartNextStepTool() Tool {
	return pm.newPlanningTool(
		"start_next_step",
		"Start the next pending step in the plan. Use this before you begin working on the next task. Only one step can be in progress at a time.",
		startNextStepSchema,
		func(argsJSON string) (string, error) {
			if pm.activePlan == nil {
				return "", fmt.Errorf("no active plan")
			}

			// Check if any step is already in progress
			for _, step := range pm.activePlan.Steps {
				if step.Status == types.PlanStepStatusInProgress {
					return "", fmt.Errorf("step already in progress: %s. Complete it first before starting next step", step.Description)
				}
			}

			// Find next pending step
			nextStep := pm.activePlan.GetNextPendingStep()
			if nextStep == nil {
				return "", fmt.Errorf("no pending steps remaining")
			}

			now := time.Now()
			nextStep.Status = types.PlanStepStatusInProgress
			nextStep.StartedAt = &now
			pm.activePlan.UpdatedAt = time.Now()

			// Update current step index
			for i, s := range pm.activePlan.Steps {
				if s.ID == nextStep.ID {
					pm.activePlan.CurrentStep = i
					break
				}
			}

			// Emit plan step started event
			if pm.eventEmitter != nil {
				pm.eventEmitter(types.NewPlanStepStartedEvent(pm.activePlan.ID, *nextStep))
			}

			return fmt.Sprintf("Started step %d/%d: %s", pm.activePlan.CurrentStep+1, len(pm.activePlan.Steps), nextStep.Description), nil
		},
	)
}

// CompleteCurrentStepTool returns a tool that allows AI to mark the current in-progress step as completed
func (pm *PlanManager) CompleteCurrentStepTool() Tool {
	return pm.newPlanningTool(
		"complete_current_step",
		"Mark the current in-progress step as completed. Use this after you have finished working on the current task.",
		completeCurrentStepSchema,
		func(argsJSON string) (string, error) {
			var args struct {
				Result string `json:"result"`
			}

			if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}

			if pm.activePlan == nil {
				return "", fmt.Errorf("no active plan")
			}

			// Find current in-progress step
			var currentStep *types.PlanStep
			for i := range pm.activePlan.Steps {
				if pm.activePlan.Steps[i].Status == types.PlanStepStatusInProgress {
					currentStep = &pm.activePlan.Steps[i]
					break
				}
			}

			if currentStep == nil {
				return "", fmt.Errorf("no step is currently in progress")
			}

			now := time.Now()
			currentStep.Status = types.PlanStepStatusCompleted
			currentStep.CompletedAt = &now
			currentStep.Result = args.Result
			pm.activePlan.UpdatedAt = time.Now()

			// Emit plan step completed event
			if pm.eventEmitter != nil {
				pm.eventEmitter(types.NewPlanStepCompletedEvent(pm.activePlan.ID, *currentStep))
			}

			// Check if plan is complete
			if pm.activePlan.IsComplete() {
				pm.activePlan.Status = types.PlanStatusCompleted

				// Emit plan completed event
				if pm.eventEmitter != nil {
					pm.eventEmitter(types.NewPlanCompletedEvent(*pm.activePlan))
				}

				return fmt.Sprintf("Step completed: %s\nAll plan steps completed! Goal achieved: %s", currentStep.Description, pm.activePlan.Goal), nil
			}

			// Show what's next
			nextPending := pm.activePlan.GetNextPendingStep()
			if nextPending != nil {
				return fmt.Sprintf("Step completed: %s\nNext step: %s", currentStep.Description, nextPending.Description), nil
			}

			return fmt.Sprintf("Step completed: %s", currentStep.Description), nil
		},
	)
}

// UpdatePlanTool returns a tool that allows AI to modify the plan
func (pm *PlanManager) UpdatePlanTool() Tool {
	return pm.newPlanningTool(
		"update_plan",
		"Modify the current plan by adding or removing steps. Use this when you realize the plan needs adjustment.",
		updatePlanSchema,
		func(argsJSON string) (string, error) {
			var args struct {
				Action          string `json:"action"`
				StepDescription string `json:"step_description"`
				StepID          string `json:"step_id"`
				Position        string `json:"position"`
			}

			if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
				return "", fmt.Errorf("invalid arguments: %w", err)
			}

			if pm.activePlan == nil {
				return "", fmt.Errorf("no active plan")
			}

			change := ""

			switch args.Action {
			case "add_step":
				if args.StepDescription == "" {
					return "", fmt.Errorf("step_description is required for add_step")
				}

				newStep := types.PlanStep{
					ID:          fmt.Sprintf("step_%d", len(pm.activePlan.Steps)+1),
					Description: args.StepDescription,
					Status:      types.PlanStepStatusPending,
				}

				// Default: add to end
				pm.activePlan.Steps = append(pm.activePlan.Steps, newStep)
				change = fmt.Sprintf("Added step: %s", args.StepDescription)

			case "remove_step":
				if args.StepID == "" {
					return "", fmt.Errorf("step_id is required for remove_step")
				}

				// Find and remove step
				found := false
				for i, step := range pm.activePlan.Steps {
					if step.ID == args.StepID {
						pm.activePlan.Steps = append(pm.activePlan.Steps[:i], pm.activePlan.Steps[i+1:]...)
						change = fmt.Sprintf("Removed step: %s", step.Description)
						found = true
						break
					}
				}

				if !found {
					return "", fmt.Errorf("step not found: %s", args.StepID)
				}

			default:
				return "", fmt.Errorf("invalid action: %s", args.Action)
			}

			pm.activePlan.UpdatedAt = time.Now()

			// Emit plan updated event
			if pm.eventEmitter != nil {
				pm.eventEmitter(types.NewPlanUpdatedEvent(*pm.activePlan, change))
			}

			return change, nil
		},
	)
}
