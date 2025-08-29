package jwtintegration

import (
	"context"
	"encoding/json"
	"flowbaker/internal/domain"
	"flowbaker/internal/managers"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	ActionCreateToken domain.IntegrationActionType = "create_token"
	ActionDecodeToken domain.IntegrationActionType = "decode_token"
)

type JWTIntegrationCreator struct {
	credentialGetter domain.CredentialGetter[JWTCredential]
	binder           domain.IntegrationParameterBinder
}

func NewJWTIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &JWTIntegrationCreator{
		credentialGetter: managers.NewExecutorCredentialGetter[JWTCredential](deps.ExecutorCredentialManager),
		binder:           deps.ParameterBinder,
	}
}

func (c *JWTIntegrationCreator) CreateIntegration(ctx context.Context, params domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	deps := JWTIntegrationDependencies{
		CredentialID:     params.CredentialID,
		CredentialGetter: c.credentialGetter,
		Binder:           c.binder,
	}

	integration, err := NewJWTIntegration(ctx, deps)
	if err != nil {
		return nil, err
	}

	return integration, nil
}

type JWTCredential struct {
	Secret    string `json:"jwt_secret"`
	Algorithm string `json:"jwt_algorithm"`
}

type JWTIntegration struct {
	jwtSecret    string
	jwtAlgorithm string

	jwtTokenGenerator *JWTTokenGenerator
	actionManager     *domain.IntegrationActionManager
	binder            domain.IntegrationParameterBinder
}

type JWTIntegrationDependencies struct {
	CredentialID     string
	CredentialGetter domain.CredentialGetter[JWTCredential]
	Binder           domain.IntegrationParameterBinder
}

func NewJWTIntegration(ctx context.Context, deps JWTIntegrationDependencies) (*JWTIntegration, error) {
	integration := &JWTIntegration{
		binder: deps.Binder,
	}

	credential, err := deps.CredentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
	if err != nil {
		return nil, err
	}

	integration.jwtSecret = credential.Secret
	integration.jwtAlgorithm = credential.Algorithm

	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(ActionCreateToken, integration.CreateToken).
		AddPerItem(ActionDecodeToken, integration.DecodeToken)

	integration.actionManager = actionManager

	jwtTokenGenerator := NewJWTTokenGenerator()

	integration.jwtTokenGenerator = jwtTokenGenerator

	return integration, nil
}

func (i *JWTIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

type CreateTokenParams struct {
	ClaimsJSON    string `json:"claims"`
	ExpireSeconds int64  `json:"expire_seconds,omitempty"`
}

func (i *JWTIntegration) CreateToken(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	createTokenParams := CreateTokenParams{}

	err := i.binder.BindToStruct(ctx, item, &createTokenParams, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	claims := map[string]any{}

	err = json.Unmarshal([]byte(createTokenParams.ClaimsJSON), &claims)
	if err != nil {
		return nil, err
	}

	expiresIn := time.Duration(createTokenParams.ExpireSeconds) * time.Second

	tokenOptions := JWTTokenGeneratorOptions{
		Algorithm: i.jwtAlgorithm,
		Secret:    i.jwtSecret,
		Claims:    claims,
		ExpiresIn: &expiresIn,
	}

	token, err := i.jwtTokenGenerator.GenerateToken(tokenOptions)
	if err != nil {
		return nil, err
	}

	tokenItem := map[string]any{
		"token": token,
	}

	return tokenItem, nil
}

type DecodeTokenParams struct {
	Token string `json:"token"`
}

func (i *JWTIntegration) DecodeToken(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	decodeTokenParams := DecodeTokenParams{}

	err := i.binder.BindToStruct(ctx, item, &decodeTokenParams, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	tokenClaims, err := i.jwtTokenGenerator.DecodeToken(decodeTokenParams.Token, i.jwtSecret, i.jwtAlgorithm)
	if err != nil {
		return nil, err
	}

	tokenItem := map[string]any{
		"claims": tokenClaims,
	}

	return tokenItem, nil
}

type JWTTokenGenerator struct {
	keyParsers map[string]func([]byte) (any, error)
}

func NewJWTTokenGenerator() *JWTTokenGenerator {
	return &JWTTokenGenerator{
		keyParsers: map[string]func([]byte) (any, error){
			"HS":    func(secret []byte) (any, error) { return secret, nil },
			"RS":    func(secret []byte) (any, error) { return jwt.ParseRSAPrivateKeyFromPEM(secret) },
			"PS":    func(secret []byte) (any, error) { return jwt.ParseRSAPrivateKeyFromPEM(secret) },
			"ES":    func(secret []byte) (any, error) { return jwt.ParseECPrivateKeyFromPEM(secret) },
			"EdDSA": func(secret []byte) (any, error) { return jwt.ParseEdPrivateKeyFromPEM(secret) },
		},
	}
}

type JWTTokenGeneratorOptions struct {
	Algorithm string         `json:"algorithm"`
	Secret    string         `json:"secret"`
	Claims    map[string]any `json:"claims"`
	ExpiresIn *time.Duration `json:"expires_in,omitempty"`
}

func (g *JWTTokenGenerator) GenerateToken(opts JWTTokenGeneratorOptions) (string, error) {
	signingMethod := jwt.GetSigningMethod(opts.Algorithm)
	if signingMethod == nil {
		return "", fmt.Errorf("unsupported algorithm: %s", opts.Algorithm)
	}

	key, err := g.parseKey(opts.Algorithm, []byte(opts.Secret))
	if err != nil {
		return "", fmt.Errorf("failed to parse key for algorithm %s: %v", opts.Algorithm, err)
	}

	claims := jwt.MapClaims{}

	for k, v := range opts.Claims {
		claims[k] = v
	}

	now := time.Now()
	claims["iat"] = now.Unix()

	if opts.ExpiresIn != nil {
		claims["exp"] = now.Add(*opts.ExpiresIn).Unix()
	}

	token := jwt.NewWithClaims(signingMethod, claims)

	tokenString, err := token.SignedString(key)
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	return tokenString, nil
}

func (g *JWTTokenGenerator) DecodeToken(tokenString string, secret string, algorithm string) (jwt.MapClaims, error) {
	signingMethod := jwt.GetSigningMethod(algorithm)

	if signingMethod == nil {
		return nil, fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	key, err := g.parseKey(algorithm, []byte(secret))
	if err != nil {
		return nil, err
	}

	tokenClaims := jwt.MapClaims{}

	token, err := jwt.ParseWithClaims(tokenString, tokenClaims, func(token *jwt.Token) (any, error) {
		if token.Method.Alg() != signingMethod.Alg() {
			return nil, fmt.Errorf("token algorithm %s does not match as expected", token.Method.Alg())
		}

		return key, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return tokenClaims, nil
}

func (g *JWTTokenGenerator) parseKey(algorithm string, secret []byte) (any, error) {
	for prefix, parser := range g.keyParsers {
		if strings.HasPrefix(algorithm, prefix) {
			return parser(secret)
		}
	}

	return nil, fmt.Errorf("unsupported algorithm: %s", algorithm)
}
