package controllers

import (
	executortypes "github.com/flowbaker/flowbaker/pkg/clients/flowbaker-executor"

	"github.com/flowbaker/flowbaker/pkg/domain/executor"
	"github.com/flowbaker/flowbaker/pkg/domain/mappers"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog/log"
)

// ExecutorController handles API-initiated requests to executor services
// This controller is used when the API needs to send commands to executors
type ExecutorController struct {
	executorService executor.WorkflowExecutorService
}

type ExecutorControllerDependencies struct {
	WorkflowExecutorService executor.WorkflowExecutorService
}

func NewExecutorController(deps ExecutorControllerDependencies) *ExecutorController {
	return &ExecutorController{
		executorService: deps.WorkflowExecutorService,
	}
}

// StartExecution handles the start of a workflow execution
func (c *ExecutorController) StartExecution(ctx fiber.Ctx) error {
	var req executortypes.StartExecutionRequest

	if err := ctx.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	isTestingWorkflow := req.WorkflowType == executortypes.WorkflowTypeTesting

	if isTestingWorkflow {
		req.Workflow = &req.TestingWorkflow.Workflow
	}

	log.Info().Msgf("Starting execution for workflow %s", req.Workflow.ID)

	p := executor.ExecuteParams{
		Workflow:          mappers.ExecutorWorkflowToDomain(req.Workflow),
		EventName:         req.EventName,
		PayloadJSON:       string(req.PayloadJSON),
		EnableEvents:      req.EnableEvents,
		IsTestingWorkflow: isTestingWorkflow,
	}

	result, err := c.executorService.Execute(ctx.RequestCtx(), p)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to execute workflow")
	}

	response := executortypes.ExecutionResult{
		Payload:    result.Payload,
		Headers:    result.Headers,
		StatusCode: result.StatusCode,
	}

	return ctx.JSON(response)
}

// HandlePollingEvent handles a polling event request from the API
func (c *ExecutorController) HandlePollingEvent(ctx fiber.Ctx) error {
	var req executortypes.PollingEventRequest

	if err := ctx.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	// Convert executor types to domain types
	pollingEvent := domain.PollingEvent{
		IntegrationType: domain.IntegrationType(req.IntegrationType),
		Trigger:         mappers.ExecutorTriggerToDomain(&req.Trigger),
		Workflow:        mappers.ExecutorWorkflowToDomain(&req.Workflow),
		UserID:          req.UserID,
		WorkflowType:    mappers.ExecutorWorkflowTypeToDomain(req.WorkflowType),
		WorkspaceID:     req.WorkspaceID,
	}

	// Call the executor service to handle the polling event
	result, err := c.executorService.HandlePollingEvent(ctx.RequestCtx(), pollingEvent)
	if err != nil {
		log.Error().Err(err).Msg("Failed to handle polling event")
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to handle polling event")
	}

	response := executortypes.PollingEventResponse{
		LastModifiedData: result.LastModifiedData,
	}

	return ctx.JSON(response)
}

// TestConnection handles connection testing requests from the API
func (c *ExecutorController) TestConnection(ctx fiber.Ctx) error {
	var req executortypes.ConnectionTestRequest

	if err := ctx.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	log.Info().
		Str("integration_type", string(req.IntegrationType)).
		Str("credential_id", req.CredentialID).
		Str("workspace_id", req.WorkspaceID).
		Msg("Testing connection")

	// Call the executor service to test the connection
	success, err := c.executorService.TestConnection(ctx.RequestCtx(), executor.TestConnectionParams{
		IntegrationType: domain.IntegrationType(req.IntegrationType),
		CredentialID:    req.CredentialID,
		WorkspaceID:     req.WorkspaceID,
		Payload:         req.Payload,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to test connection")
		return ctx.JSON(executortypes.ConnectionTestResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	return ctx.JSON(executortypes.ConnectionTestResponse{
		Success: success,
	})
}

func (c *ExecutorController) PeekData(ctx fiber.Ctx) error {
	var req executortypes.PeekDataRequest

	if err := ctx.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	result, err := c.executorService.PeekData(ctx.RequestCtx(), executor.PeekDataParams{
		IntegrationType: domain.IntegrationType(req.IntegrationType),
		CredentialID:    req.CredentialID,
		WorkspaceID:     req.WorkspaceID,
		UserID:          req.UserID,
		PeekableType:    req.PeekableType,
		Cursor:          req.Cursor,
		PayloadJSON:     req.PayloadJSON,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to peek data")

		return ctx.JSON(executortypes.PeekDataResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	return ctx.JSON(executortypes.PeekDataResponse{
		Success:    true,
		ResultJSON: result.ResultJSON,
		Result:     mappers.DomainPeekResultItemsToExecutor(result.Result),
		Cursor:     result.Cursor,
		HasMore:    result.HasMore,
	})
}
