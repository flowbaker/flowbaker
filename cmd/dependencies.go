package main

import (
	"context"
	"time"

	"github.com/flowbaker/flowbaker/pkg/clients/flowbaker"

	"github.com/flowbaker/flowbaker/internal/controllers"
	"github.com/flowbaker/flowbaker/internal/expressions"
	"github.com/flowbaker/flowbaker/pkg/domain/executor"

	"github.com/flowbaker/flowbaker/internal/managers"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/rs/zerolog/log"
)

// ExecutorDependencies contains all dependencies needed by the executor
type ExecutorDependencies struct {
	FlowbakerClient         *flowbaker.Client
	IntegrationSelector     domain.IntegrationSelector
	WorkflowExecutorService executor.WorkflowExecutorService
	ExecutorController      *controllers.ExecutorController
}

// ExecutorDependencyConfig contains configuration for dependency injection
type ExecutorDependencyConfig struct {
	FlowbakerClient *flowbaker.Client
	ExecutorID      string
	Config          *Config
}

// BuildExecutorDependencies creates and wires up all executor dependencies
func BuildExecutorDependencies(ctx context.Context, config ExecutorDependencyConfig) (*ExecutorDependencies, error) {
	log.Info().Msg("Building executor dependencies")
	logger := log.Logger

	kangarooBinder, err := expressions.NewKangarooBinder(expressions.KangarooBinderOptions{
		Logger:         logger,
		DefaultTimeout: 5 * time.Second,
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Kangaroo expression binder")

		return nil, err
	}

	integrationSelector := domain.NewIntegrationSelector()

	executorStorageManager := managers.NewExecutorStorageManager(managers.ExecutorStorageManagerDependencies{
		Client: config.FlowbakerClient,
	})

	executorCredentialDecryptor := managers.NewExecutorCredentialDecryptionService(config.Config.X25519PrivateKey)
	executorCredentialManager := managers.NewExecutorCredentialManager(config.FlowbakerClient, executorCredentialDecryptor)
	executorEventPublisher := managers.NewExecutorEventPublisher(config.FlowbakerClient)
	executorTaskPublisher := managers.NewExecutorTaskPublisher(managers.ExecutorTaskPublisherDependencies{
		Client: config.FlowbakerClient,
	})
	executorIntegrationManager := managers.NewExecutorIntegrationManager(managers.ExecutorIntegrationManagerDependencies{
		Client: config.FlowbakerClient,
	})
	orderedEventPublisher := domain.NewOrderedEventPublisher(executorEventPublisher)
	executorScheduleManager := managers.NewExecutorScheduleManager(managers.ExecutorScheduleManagerDependencies{
		Client: config.FlowbakerClient,
	})

	executorAgentMemoryService := managers.NewExecutorAgentMemoryService(managers.ExecutorAgentMemoryServiceDependencies{
		Client:      config.FlowbakerClient,
		WorkspaceID: config.Config.WorkspaceID,
	})

	executorKnowledgeManager := managers.NewExecutorKnowledgeManager(managers.ExecutorKnowledgeManagerDependencies{
		Client: config.FlowbakerClient,
	})

	// Create common dependencies for integrations
	integrationDeps := domain.IntegrationDeps{
		FlowbakerClient:            config.FlowbakerClient,
		IntegrationSelector:        integrationSelector,
		ExecutorStorageManager:     executorStorageManager,
		ExecutorCredentialManager:  executorCredentialManager,
		ParameterBinder:            kangarooBinder,
		AgentMemoryService:         executorAgentMemoryService,
		ExecutorEventPublisher:     orderedEventPublisher,
		ExecutorTaskPublisher:      executorTaskPublisher,
		ExecutorIntegrationManager: executorIntegrationManager,
		ExecutorScheduleManager:    executorScheduleManager,
		ExecutorKnowledgeManager:   executorKnowledgeManager,
	}

	// Register integrations
	if err := RegisterIntegrations(ctx, RegisterIntegrationParams{
		IntegrationSelector: integrationSelector,
		Deps:                integrationDeps,
	}); err != nil {
		return nil, err
	}

	// Create workflow executor service
	workflowExecutorService := executor.NewWorkflowExecutorService(executor.WorkflowExecutorServiceDependencies{
		IntegrationSelector:   integrationSelector,
		OrderedEventPublisher: orderedEventPublisher,
		FlowbakerClient:       config.FlowbakerClient,
		CredentialManager:     executorCredentialManager,
	})

	// Create executor controller
	executorController := controllers.NewExecutorController(controllers.ExecutorControllerDependencies{
		WorkflowExecutorService: workflowExecutorService,
	})

	log.Info().Msg("Executor dependencies built successfully")

	return &ExecutorDependencies{
		FlowbakerClient:         config.FlowbakerClient,
		IntegrationSelector:     integrationSelector,
		WorkflowExecutorService: workflowExecutorService,
		ExecutorController:      executorController,
	}, nil
}
