package domain

import "context"

type ResolvedAPIDefinition struct {
	ID          string
	WorkspaceID string
	Name        string
	Type        string
	BaseURL     string
	Endpoints   []ResolvedAPIEndpoint
	Auth        ResolvedAPIAuth
}

type ResolvedAPIEndpoint struct {
	Path    string
	Method  string
	Summary string
}

type ResolvedAPIAuth struct {
	Type          string
	KeyLocation   string
	KeyName       string
	HMACAlgorithm string
	HMACHeader    string
	HMACPrefix    string
	Secret        string
}

type ExecutorAPIDefinitionManager interface {
	Load(ctx context.Context, apiDefinitionID string) (ResolvedAPIDefinition, error)
}
