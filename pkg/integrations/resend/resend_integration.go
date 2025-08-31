package resendintegration

import (
	"context"
	"fmt"
	"strings"

	"github.com/flowbaker/flowbaker/internal/managers"

	"github.com/flowbaker/flowbaker/pkg/domain"

	"github.com/resend/resend-go/v2"
)

const (
	ResendIntegrationActionType_CreateContact domain.IntegrationActionType = "create_contact"
	ResendIntegrationActionType_SendEmail     domain.IntegrationActionType = "send_email"

	ResendIntegrationPeekable_Audiences domain.IntegrationPeekableType = "audiences"
)

type ResendIntegrationCreator struct {
	binder           domain.IntegrationParameterBinder
	credentialGetter domain.CredentialGetter[ResendCredential]
}

func NewResendIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &ResendIntegrationCreator{
		binder:           deps.ParameterBinder,
		credentialGetter: managers.NewExecutorCredentialGetter[ResendCredential](deps.ExecutorCredentialManager),
	}
}

func (c *ResendIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewResendIntegration(ctx, ResendIntegrationDependencies{
		CredentialID:     p.CredentialID,
		ParameterBinder:  c.binder,
		CredentialGetter: c.credentialGetter,
	})
}

type ResendIntegration struct {
	client *resend.Client

	binder           domain.IntegrationParameterBinder
	credentialGetter domain.CredentialGetter[ResendCredential]

	actionManager *domain.IntegrationActionManager
	peekFuncs     map[domain.IntegrationPeekableType]func(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error)
}

type ResendCredential struct {
	APIKey string `json:"api_key"`
}

type ResendIntegrationDependencies struct {
	CredentialID     string
	ParameterBinder  domain.IntegrationParameterBinder
	CredentialGetter domain.CredentialGetter[ResendCredential]
}

func NewResendIntegration(ctx context.Context, deps ResendIntegrationDependencies) (*ResendIntegration, error) {
	integration := &ResendIntegration{
		binder:           deps.ParameterBinder,
		credentialGetter: deps.CredentialGetter,
		actionManager:    domain.NewIntegrationActionManager(),
	}

	actionManager := domain.NewIntegrationActionManager().
		AddPerItem(ResendIntegrationActionType_CreateContact, integration.CreateContact).
		AddPerItem(ResendIntegrationActionType_SendEmail, integration.SendEmail)

	integration.actionManager = actionManager

	peekFuncs := map[domain.IntegrationPeekableType]func(ctx context.Context, p domain.PeekParams) (domain.PeekResult, error){
		ResendIntegrationPeekable_Audiences: integration.PeekAudiences,
	}

	integration.peekFuncs = peekFuncs

	// Get API key from credentials
	credential, err := deps.CredentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
	if err != nil {
		return nil, err
	}

	integration.client = resend.NewClient(credential.APIKey)

	return integration, nil
}

func (i *ResendIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

type CreateContactParams struct {
	AudienceID   string `json:"audience_id"`
	Email        string `json:"email"`
	FirstName    string `json:"first_name,omitempty"`
	LastName     string `json:"last_name,omitempty"`
	Unsubscribed bool   `json:"unsubscribed,omitempty"`
}

type CreateContactOutputItem struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	FirstName    string `json:"first_name,omitempty"`
	LastName     string `json:"last_name,omitempty"`
	CreatedAt    string `json:"created_at"`
	Unsubscribed bool   `json:"unsubscribed"`
}

func (i *ResendIntegration) CreateContact(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := CreateContactParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	// Create contact via Resend API
	output, err := i.createResendContact(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("failed to create contact: %w", err)
	}

	return output, nil
}

// createContactAPI makes the actual API call to create a contact in Resend
func (i *ResendIntegration) createResendContact(ctx context.Context, params CreateContactParams) (CreateContactOutputItem, error) {
	// Create contact using Resend SDK
	createContactRequest := &resend.CreateContactRequest{
		Email:        params.Email,
		AudienceId:   params.AudienceID,
		FirstName:    params.FirstName,
		LastName:     params.LastName,
		Unsubscribed: params.Unsubscribed,
	}

	response, err := i.client.Contacts.CreateWithContext(ctx, createContactRequest)
	if err != nil {
		return CreateContactOutputItem{}, fmt.Errorf("failed to create contact via SDK: %w", err)
	}

	// Convert SDK response to our output format
	return CreateContactOutputItem{
		ID:           response.Id,
		Email:        params.Email,
		FirstName:    params.FirstName,
		LastName:     params.LastName,
		CreatedAt:    "", // SDK doesn't return created_at in create response
		Unsubscribed: params.Unsubscribed,
	}, nil
}

type SendEmailParams struct {
	From    string   `json:"from"`
	To      []string `json:"to"` // tag input array
	Subject string   `json:"subject"`
	Html    string   `json:"html,omitempty"`
	Text    string   `json:"text,omitempty"`
	ReplyTo string   `json:"reply_to,omitempty"`
	Cc      []string `json:"cc,omitempty"`   // tag input array
	Bcc     []string `json:"bcc,omitempty"`  // tag input array
	Tags    []string `json:"tags,omitempty"` // tag input array
}

type SendEmailOutputItem struct {
	ID        string `json:"id"`
	From      string `json:"from"`
	To        string `json:"to"` // comma-separated for consistency with input
	CreatedAt string `json:"created_at"`
}

func (i *ResendIntegration) SendEmail(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := SendEmailParams{}

	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	// Send email via Resend API
	output, err := i.sendResendEmail(ctx, p)
	if err != nil {
		return nil, fmt.Errorf("failed to send email: %w", err)
	}

	return output, nil
}

// sendEmailAPI makes the actual API call to send an email via Resend
func (i *ResendIntegration) sendResendEmail(ctx context.Context, params SendEmailParams) (SendEmailOutputItem, error) {
	// Create email request using Resend SDK
	sendEmailRequest := &resend.SendEmailRequest{
		From:    params.From,
		To:      params.To, // Already a []string array
		Subject: params.Subject,
		Html:    params.Html,
		Text:    params.Text,
		ReplyTo: params.ReplyTo,
	}

	if len(params.Cc) > 0 {
		sendEmailRequest.Cc = params.Cc
	}
	if len(params.Bcc) > 0 {
		sendEmailRequest.Bcc = params.Bcc
	}
	if len(params.Tags) > 0 {
		tags := make([]resend.Tag, len(params.Tags))
		for i, tagStr := range params.Tags {
			tags[i] = resend.Tag{Name: tagStr, Value: tagStr}
		}
		sendEmailRequest.Tags = tags
	}

	response, err := i.client.Emails.SendWithContext(ctx, sendEmailRequest)
	if err != nil {
		return SendEmailOutputItem{}, fmt.Errorf("failed to send email via SDK: %w", err)
	}

	// Convert SDK response to our output format
	return SendEmailOutputItem{
		ID:        response.Id,
		From:      params.From,
		To:        strings.Join(params.To, ", "), // Convert array back to comma-separated string for consistency
		CreatedAt: "",                            // SDK doesn't return created_at in send response
	}, nil
}

func (i *ResendIntegration) Peek(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	peekFunc, ok := i.peekFuncs[params.PeekableType]
	if !ok {
		return domain.PeekResult{}, fmt.Errorf("peek function not found")
	}

	return peekFunc(ctx, params)
}

func (i *ResendIntegration) PeekAudiences(ctx context.Context, params domain.PeekParams) (domain.PeekResult, error) {
	// Get audiences via Resend API
	audiences, err := i.getAudiencesAPI(ctx)
	if err != nil {
		return domain.PeekResult{}, fmt.Errorf("failed to get audiences: %w", err)
	}

	var results []domain.PeekResultItem
	for _, audience := range audiences {
		results = append(results, domain.PeekResultItem{
			Key:     audience.ID,
			Value:   audience.ID,
			Content: audience.Name,
		})
	}

	return domain.PeekResult{
		Result: results,
	}, nil
}

// ResendAudience represents an audience from the Resend API
type ResendAudience struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// getAudiencesAPI makes the actual API call to get audiences from Resend
func (i *ResendIntegration) getAudiencesAPI(ctx context.Context) ([]ResendAudience, error) {
	// Get audiences using Resend SDK
	response, err := i.client.Audiences.ListWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get audiences via SDK: %w", err)
	}

	// Convert SDK response to our format
	audiences := make([]ResendAudience, len(response.Data))
	for i, audience := range response.Data {
		audiences[i] = ResendAudience{
			ID:   audience.Id,
			Name: audience.Name,
		}
	}

	return audiences, nil
}
