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
	executorService              executor.WorkflowExecutorService
	workspaceRegistrationManager domain.WorkspaceRegistrationManager
}

type ExecutorControllerDependencies struct {
	WorkflowExecutorService      executor.WorkflowExecutorService
	WorkspaceRegistrationManager domain.WorkspaceRegistrationManager
}

func NewExecutorController(deps ExecutorControllerDependencies) *ExecutorController {
	return &ExecutorController{
		executorService:              deps.WorkflowExecutorService,
		workspaceRegistrationManager: deps.WorkspaceRegistrationManager,
	}
}

func (c *ExecutorController) RegisterWorkspace(ctx fiber.Ctx) error {
	log.Info().Msg("Registering workspace")

	var req executortypes.RegisterWorkspaceRequest

	if err := ctx.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	p := domain.RegisterWorkspaceParams{
		ExecutorID: req.ExecutorID,
		Passcode:   req.Passcode,
		Assignment: domain.WorkspaceAssignment{
			WorkspaceID:   req.Assignment.WorkspaceID,
			WorkspaceName: req.Assignment.WorkspaceName,
			WorkspaceSlug: req.Assignment.WorkspaceSlug,
			APIPublicKey:  req.Assignment.APIPublicKey,
		},
	}

	err := c.workspaceRegistrationManager.TryRegisterWorkspace(ctx.RequestCtx(), p)
	if err != nil {
		log.Error().Err(err).Msg("Failed to register workspace")
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to register workspace")
	}

	return ctx.JSON(executortypes.RegisterWorkspaceResponse{
		Success: true,
	})
}

// StartExecution handles the start of a workflow execution
func (c *ExecutorController) StartExecution(ctx fiber.Ctx) error {
	workspaceID := ctx.Params("workspaceID")
	if workspaceID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Workspace ID is required")
	}

	var req executortypes.StartExecutionRequest

	if err := ctx.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	isTestingWorkflow := req.WorkflowType == executortypes.WorkflowTypeTesting

	if isTestingWorkflow {
		req.Workflow = &req.TestingWorkflow.Workflow
	}

	p := executor.ExecuteParams{
		ExecutionID:       req.ExecutionID,
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

func (c *ExecutorController) RerunNode(ctx fiber.Ctx) error {
	workspaceID := ctx.Params("workspaceID")
	if workspaceID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Workspace ID is required")
	}

	var req executortypes.RerunNodeRequest

	if err := ctx.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	result, err := c.executorService.RerunNode(ctx.RequestCtx(), executor.RerunNodeParams{
		ExecutionID:        req.ExecutionID,
		WorkspaceID:        workspaceID,
		NodeID:             req.NodeID,
		NodeExecutionEntry: mappers.FlowbakerNodeExecutionEntryToDomain(req.NodeExecutionEntry),
		Workflow:           mappers.ExecutorWorkflowToDomain(&req.Workflow),
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to rerun node")
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to rerun node")
	}

	return ctx.JSON(executortypes.RerunNodeResponse{
		Payload: result.Payload,
	})
}

func (c *ExecutorController) RunNode(ctx fiber.Ctx) error {
	workspaceID := ctx.Params("workspaceID")
	if workspaceID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Workspace ID is required")
	}

	var req executortypes.RunNodeRequest
	if err := ctx.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	result, err := c.executorService.RunNode(ctx.RequestCtx(), executor.RunNodeParams{
		ExecutionID:  req.ExecutionID,
		NodeID:       req.NodeID,
		Workflow:     mappers.ExecutorWorkflowToDomain(&req.Workflow),
		ItemsByInput: req.ItemsByInputID,
		WorkspaceID:  workspaceID,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to run node")
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to run node")
	}

	return ctx.JSON(executortypes.RunNodeResponse{
		Results: result.Results,
	})
}

func (c *ExecutorController) StopExecution(ctx fiber.Ctx) error {
	executionID := ctx.Params("executionID")
	if executionID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Execution ID is required")
	}

	// it should return success no matter what
	err := c.executorService.Stop(ctx.RequestCtx(), executionID)
	if err != nil {
		return ctx.JSON(executortypes.StopExecutionResponse{
			Success: true,
		})
	}

	return ctx.JSON(executortypes.StopExecutionResponse{
		Success: true,
	})
}

// HandlePollingEvent handles a polling event request from the API
func (c *ExecutorController) HandlePollingEvent(ctx fiber.Ctx) error {
	workspaceID := ctx.Params("workspaceID")
	if workspaceID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Workspace ID is required")
	}

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
		WorkspaceID:     workspaceID,
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
	workspaceID := ctx.Params("workspaceID")
	if workspaceID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Workspace ID is required")
	}

	var req executortypes.ConnectionTestRequest

	if err := ctx.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	log.Info().
		Str("integration_type", string(req.IntegrationType)).
		Str("credential_id", req.CredentialID).
		Str("workspace_id", workspaceID).
		Msg("Testing connection")

	// Call the executor service to test the connection
	success, err := c.executorService.TestConnection(ctx.RequestCtx(), executor.TestConnectionParams{
		IntegrationType: domain.IntegrationType(req.IntegrationType),
		CredentialID:    req.CredentialID,
		WorkspaceID:     workspaceID,
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
	workspaceID := ctx.Params("workspaceID")
	if workspaceID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Workspace ID is required")
	}

	var req executortypes.PeekDataRequest

	if err := ctx.Bind().Body(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid request body")
	}

	var pagination domain.PaginationParams
	if req.Pagination != nil {
		pagination = *req.Pagination
	}

	result, err := c.executorService.PeekData(ctx.RequestCtx(), executor.PeekDataParams{
		IntegrationType: domain.IntegrationType(req.IntegrationType),
		CredentialID:    req.CredentialID,
		WorkspaceID:     workspaceID,
		UserID:          req.UserID,
		PeekableType:    req.PeekableType,
		Cursor:          req.Cursor,
		Pagination:      pagination,
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
		Result:     mappers.DomainPeekResultItemsToExecutor(result.Result),
		Pagination: result.Pagination,
	})
}

func (c *ExecutorController) UnregisterWorkspace(ctx fiber.Ctx) error {
	workspaceID := ctx.Params("workspaceID")
	if workspaceID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Workspace ID is required")
	}

	err := c.workspaceRegistrationManager.UnregisterWorkspace(ctx.RequestCtx(), workspaceID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to unregister workspace")
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to unregister workspace")
	}

	return ctx.JSON(executortypes.UnregisterWorkspaceResponse{
		Success: true,
	})
}
