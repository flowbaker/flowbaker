package domain

type ExecutorCredentialDecryptionService interface {
	DecryptCredential(encryptedCred EncryptedExecutionCredential) ([]byte, error)
}
