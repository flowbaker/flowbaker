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
	WorkspaceID   string `json:"workspace_id"`
	WorkspaceName string `json:"workspace_name"`
	WorkspaceSlug string `json:"workspace_slug"`
	APIPublicKey  string `json:"api_public_key"`
}

type RegisterWorkspaceParams struct {
	Passcode   string              `json:"passcode"`
	Assignment WorkspaceAssignment `json:"assignment"`
}

type WorkspaceRegistrationManager interface {
	TryRegisterWorkspace(ctx context.Context, params RegisterWorkspaceParams) error
	AddPasscode(ctx context.Context, params AddPasscodeParams) error
}

type workspaceRegistrationManager struct {
	codeStore      map[string]PassCodeEntry
	codeStoreMutex sync.RWMutex
}

func NewWorkspaceRegistrationManager() WorkspaceRegistrationManager {
	return &workspaceRegistrationManager{
		codeStore: make(map[string]PassCodeEntry),
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

	if time.Now().After(passcodeEntry.ExpiresAt) {
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
