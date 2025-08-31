package domain

import "time"

type KnowledgeType string

var (
	KnowledgeType_Embeddings KnowledgeType = "embeddings"
)

type Knowledge struct {
	ID          string
	WorkspaceID string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	FileCount   int64
	Type        KnowledgeType

	EmbeddingsOptions KnowledgeEmbeddingsOptions
}

type EmbeddingModelType string
type RerankingModelType string

type CreateEmbeddingsParams struct {
	Text      string
	ModelType EmbeddingModelType
}

type KnowledgeEmbeddingsOptions struct {
	EmbeddingCredentialID     string
	EmbeddingModelIntegration IntegrationType
	EmbeddingModelType        EmbeddingModelType

	RerankingCredentialID     string
	RerankingModelIntegration IntegrationType
	RerankingModelType        RerankingModelType
}

type KnowledgeFileStatus string

const (
	KnowledgeFileStatus_Pending    KnowledgeFileStatus = "pending"
	KnowledgeFileStatus_Processing KnowledgeFileStatus = "processing"
	KnowledgeFileStatus_Completed  KnowledgeFileStatus = "completed"
	KnowledgeFileStatus_Failed     KnowledgeFileStatus = "failed"
)

type KnowledgeFile struct {
	ID           string
	KnowledgeID  string
	FileID       string
	FileMetadata KnowledgeFileMetadata
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Settings     KnowledgeFileSettings
	Status       KnowledgeFileStatus
}

type KnowledgeFileMetadata struct {
	FileName    string
	ContentType string
	Size        int64
	UploadedAt  time.Time
}

type KnowledgeFileSettings struct {
	Delimiter          string
	MaximumChunkLength int
	ChunkOverlap       int
}
