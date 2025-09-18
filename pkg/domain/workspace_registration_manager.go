package domain

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

var (
	ErrInvalidRegistrationCode = errors.New("invalid registration code")
)

type WorkspaceAssignment struct {
	WorkspaceID   string `json:"workspace_id" mapstructure:"workspace_id"`
	WorkspaceName string `json:"workspace_name" mapstructure:"workspace_name"`
	WorkspaceSlug string `json:"workspace_slug" mapstructure:"workspace_slug"`
	APIPublicKey  string `json:"api_public_key" mapstructure:"api_public_key"`
}

type RegisterWorkspaceParams struct {
	ExecutorID string              `json:"executor_id"`
	Passcode   string              `json:"passcode"`
	Assignment WorkspaceAssignment `json:"assignment"`
}

type WorkspaceRegistrationManager interface {
	TryRegisterWorkspace(ctx context.Context, params RegisterWorkspaceParams) error
	UnregisterWorkspace(ctx context.Context, workspaceID string) error
	AddPasscode(ctx context.Context, params AddPasscodeParams) error
}

type workspaceRegistrationManager struct {
	codeStore      map[string]PassCodeEntry
	codeStoreMutex sync.RWMutex
	configManager  ConfigManager
}

func NewWorkspaceRegistrationManager(config ExecutorConfig, configManager ConfigManager) WorkspaceRegistrationManager {
	store := map[string]PassCodeEntry{}

	if config.EnableStaticPasscode {
		store[config.StaticPasscode] = PassCodeEntry{
			Passcode:  config.StaticPasscode,
			ExpiresAt: time.Time{},
			OnSuccess: func(ctx context.Context, params RegisterWorkspaceParams) error {
				log.Info().
					Str("workspace_id", params.Assignment.WorkspaceID).
					Str("workspace_slug", params.Assignment.WorkspaceSlug).
					Str("workspace_name", params.Assignment.WorkspaceName).
					Msg("Registered workspace via static passcode")

				return nil
			},
		}
	}

	return &workspaceRegistrationManager{
		codeStore:     store,
		configManager: configManager,
	}
}

func (m *workspaceRegistrationManager) TryRegisterWorkspace(ctx context.Context, params RegisterWorkspaceParams) error {
	m.codeStoreMutex.RLock()
	defer m.codeStoreMutex.RUnlock()

	passcodeEntry, ok := m.codeStore[params.Passcode]
	if !ok {
		log.Debug().Str("passcode", params.Passcode).Msg("not found")

		return ErrInvalidRegistrationCode
	}

	if passcodeEntry.Passcode != params.Passcode {
		log.Debug().Str("passcode", params.Passcode).Str("stored_passcode", passcodeEntry.Passcode).Msg("Invalid passcode")

		return ErrInvalidRegistrationCode
	}

	if time.Now().After(passcodeEntry.ExpiresAt) && !passcodeEntry.ExpiresAt.IsZero() {
		log.Debug().Str("passcode", params.Passcode).Str("expires_at", passcodeEntry.ExpiresAt.Format(time.RFC3339)).Msg("expired")

		return ErrInvalidRegistrationCode
	}

	if passcodeEntry.OnSuccess == nil {
		log.Debug().Str("passcode", params.Passcode).Msg("on success callback is nil")

		return fmt.Errorf("on success callback is nil")
	}

	return passcodeEntry.OnSuccess(ctx, params)
}

type PassCodeEntry struct {
	Passcode  string
	ExpiresAt time.Time
	OnSuccess func(ctx context.Context, params RegisterWorkspaceParams) error
}

const (
	PassCodeExpiresIn = 10 * time.Minute
)

type AddPasscodeParams struct {
	Passcode  string
	OnSuccess func(ctx context.Context, params RegisterWorkspaceParams) error
}

func (m *workspaceRegistrationManager) AddPasscode(ctx context.Context, params AddPasscodeParams) error {
	m.codeStoreMutex.Lock()
	defer m.codeStoreMutex.Unlock()

	now := time.Now()

	m.codeStore[params.Passcode] = PassCodeEntry{
		Passcode:  params.Passcode,
		ExpiresAt: now.Add(PassCodeExpiresIn),
		OnSuccess: params.OnSuccess,
	}

	return nil
}

func (m *workspaceRegistrationManager) UnregisterWorkspace(ctx context.Context, workspaceID string) error {
	config, err := m.configManager.GetConfig(ctx)
	if err != nil {
		return fmt.Errorf("unregistering workspace: failed to get config: %w", err)
	}

	var updatedAssignments []WorkspaceAssignment

	found := false

	for _, assignment := range config.WorkspaceAssignments {
		if assignment.WorkspaceID != workspaceID {
			updatedAssignments = append(updatedAssignments, assignment)
		} else {
			found = true
		}
	}

	if !found {
		log.Warn().Str("workspace_id", workspaceID).Msg("Unregistering workspace: Workspace not found in assignments")

		return fmt.Errorf("unregistering workspace: workspace not found: %s", workspaceID)
	}

	config.WorkspaceAssignments = updatedAssignments

	if err := m.configManager.SaveConfig(ctx, config); err != nil {
		log.Error().Err(err).Msg("Unregistering workspace: Failed to save config")

		return fmt.Errorf("unregistering workspace: failed to save config: %w", err)
	}

	log.Info().Str("workspace_id", workspaceID).Msg("Successfully unregistered workspace")

	return nil
}
