package initialization

import (
	"context"
	"time"

	"github.com/flowbaker/flowbaker/internal/controllers"
	"github.com/flowbaker/flowbaker/internal/expressions"
	"github.com/flowbaker/flowbaker/internal/managers"
	"github.com/flowbaker/flowbaker/pkg/clients/flowbaker"
	"github.com/flowbaker/flowbaker/pkg/domain"
	"github.com/flowbaker/flowbaker/pkg/domain/executor"

	"github.com/rs/zerolog/log"
)

type ExecutorDependencies struct {
	FlowbakerClient         *flowbaker.Client
	IntegrationSelector     domain.IntegrationSelector
	WorkflowExecutorService executor.WorkflowExecutorService
	ExecutorController      *controllers.ExecutorController
}

type ExecutorDependencyConfig struct {
	FlowbakerClient *flowbaker.Client
	ExecutorID      string
	Config          domain.ExecutorConfig
}

type ExecutorContainer struct {
	configManager                domain.ConfigManager
	workspaceRegistrationManager domain.WorkspaceRegistrationManager
}

func NewExecutorContainer() (*ExecutorContainer, error) {
	configManager, err := domain.NewConfigManager()
	if err != nil {
		return nil, err
	}

	config, err := configManager.GetConfig(context.Background())
	if err != nil {
		return nil, err
	}

	workspaceRegistrationManager := domain.NewWorkspaceRegistrationManager(config, configManager)

	return &ExecutorContainer{
		configManager:                configManager,
		workspaceRegistrationManager: workspaceRegistrationManager,
	}, nil
}

func (c *ExecutorContainer) GetConfigManager() domain.ConfigManager {
	return c.configManager
}

func (c *ExecutorContainer) GetWorkspaceRegistrationManager() domain.WorkspaceRegistrationManager {
	return c.workspaceRegistrationManager
}

func (c *ExecutorContainer) BuildExecutorDependencies(ctx context.Context, config ExecutorDependencyConfig) (*ExecutorDependencies, error) {
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
		Client: config.FlowbakerClient,
	})

	executorKnowledgeManager := managers.NewExecutorKnowledgeManager(managers.ExecutorKnowledgeManagerDependencies{
		Client: config.FlowbakerClient,
	})

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

	if err := registerIntegrations(integrationSelector, integrationDeps); err != nil {
		return nil, err
	}

	workflowExecutorService := executor.NewWorkflowExecutorService(executor.WorkflowExecutorServiceDependencies{
		IntegrationSelector:   integrationSelector,
		OrderedEventPublisher: orderedEventPublisher,
		FlowbakerClient:       config.FlowbakerClient,
		CredentialManager:     executorCredentialManager,
	})

	executorController := controllers.NewExecutorController(controllers.ExecutorControllerDependencies{
		WorkflowExecutorService:      workflowExecutorService,
		WorkspaceRegistrationManager: c.workspaceRegistrationManager,
	})

	log.Info().Msg("Executor dependencies built successfully")

	return &ExecutorDependencies{
		FlowbakerClient:         config.FlowbakerClient,
		IntegrationSelector:     integrationSelector,
		WorkflowExecutorService: workflowExecutorService,
		ExecutorController:      executorController,
	}, nil
}
