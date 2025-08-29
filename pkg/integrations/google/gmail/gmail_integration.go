package gmail

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flowbaker/internal/domain"
	"flowbaker/internal/managers"
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
	"golang.org/x/oauth2"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

const (
	IntegrationTriggerType_OnMessageReceived domain.IntegrationTriggerEventType = "on_message_received"

	IntegrationActionType_GetMails             domain.IntegrationActionType = "get_mails"
	IntegrationActionType_GetMail              domain.IntegrationActionType = "get_mail"
	IntegrationActionType_SendMail             domain.IntegrationActionType = "send_mail"
	IntegrationActionType_FindMail             domain.IntegrationActionType = "find_mail"
	IntegrationActionType_DeleteMails          domain.IntegrationActionType = "delete_mails"
	IntegrationActionType_DeleteMail           domain.IntegrationActionType = "delete_mail"
	IntegrationActionType_SendMailsToTrash     domain.IntegrationActionType = "send_mails_to_trash"
	IntegrationActionType_SendMailToTrash      domain.IntegrationActionType = "send_mail_to_trash"
	IntegrationActionType_GetThreads           domain.IntegrationActionType = "get_threads"
	IntegrationActionType_GetThread            domain.IntegrationActionType = "get_thread"
	IntegrationActionType_MarkMailAsRead       domain.IntegrationActionType = "mark_mail_as_read"
	IntegrationActionType_MarkMailAsUnread     domain.IntegrationActionType = "mark_mail_as_unread"
	IntegrationActionType_MarkMailsAsRead      domain.IntegrationActionType = "mark_mails_as_read"
	IntegrationActionType_MarkMailsAsUnread    domain.IntegrationActionType = "mark_mails_as_unread"
	IntegrationActionType_CreateDraft          domain.IntegrationActionType = "create_draft"
	IntegrationActionType_GetDrafts            domain.IntegrationActionType = "get_drafts"
	IntegrationActionType_GetDraft             domain.IntegrationActionType = "get_draft"
	IntegrationActionType_SendDraft            domain.IntegrationActionType = "send_draft"
	IntegrationActionType_DeleteDraft          domain.IntegrationActionType = "delete_draft"
	IntegrationActionType_DeleteDrafts         domain.IntegrationActionType = "delete_drafts"
	IntegrationActionType_ReplyMail            domain.IntegrationActionType = "reply_mail"
	IntegrationActionType_GetLabels            domain.IntegrationActionType = "get_labels"
	IntegrationActionType_AddLabel             domain.IntegrationActionType = "add_label"
	IntegrationActionType_RemoveLabel          domain.IntegrationActionType = "remove_label"
	IntegrationActionType_AddLabelToEmail      domain.IntegrationActionType = "add_label_to_email"
	IntegrationActionType_RemoveLabelFromEmail domain.IntegrationActionType = "remove_label_from_email"
)

type GmailIntegrationCreator struct {
	credentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	binder           domain.IntegrationParameterBinder
}

func NewGmailIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &GmailIntegrationCreator{
		credentialGetter: managers.NewExecutorCredentialGetter[domain.OAuthAccountSensitiveData](deps.ExecutorCredentialManager),
		binder:           deps.ParameterBinder,
	}
}

func (c *GmailIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {
	return NewGmailIntegration(ctx, GmailIntegrationDependencies{
		CredentialGetter: c.credentialGetter,
		ParameterBinder:  c.binder,
		CredentialID:     p.CredentialID,
	})
}

type GmailIntegration struct {
	credentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	binder           domain.IntegrationParameterBinder
	service          *gmail.Service
	actionFuncs      map[domain.IntegrationActionType]func(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error)
}

type GmailIntegrationDependencies struct {
	CredentialID     string
	CredentialGetter domain.CredentialGetter[domain.OAuthAccountSensitiveData]
	ParameterBinder  domain.IntegrationParameterBinder
}

func (g *GmailIntegration) getClient(ctx context.Context, oauthAccount domain.OAuthAccountSensitiveData) (*http.Client, error) {
	token := &oauth2.Token{
		AccessToken:  oauthAccount.AccessToken,
		RefreshToken: oauthAccount.RefreshToken,
	}

	ts := oauth2.StaticTokenSource(token)
	client := oauth2.NewClient(ctx, ts)
	return client, nil
}

func NewGmailIntegration(ctx context.Context, deps GmailIntegrationDependencies) (*GmailIntegration, error) {
	integration := &GmailIntegration{
		credentialGetter: deps.CredentialGetter,
		binder:           deps.ParameterBinder,
	}

	oauthAccount, err := deps.CredentialGetter.GetDecryptedCredential(ctx, deps.CredentialID)
	if err != nil {
		return nil, err
	}

	client, err := integration.getClient(ctx, oauthAccount)
	if err != nil {
		return nil, err
	}

	service, err := gmail.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, fmt.Errorf("failed to create gmail service: %w", err)
	}

	integration.service = service

	actionFuncs := map[domain.IntegrationActionType]func(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error){
		IntegrationActionType_SendMail:             integration.SendMail,
		IntegrationActionType_FindMail:             integration.FindMail,
		IntegrationActionType_GetMail:              integration.GetMail,
		IntegrationActionType_GetMails:             integration.GetMails,
		IntegrationActionType_SendMailToTrash:      integration.SendMailToTrash,
		IntegrationActionType_SendMailsToTrash:     integration.SendMailsToTrash,
		IntegrationActionType_DeleteMail:           integration.DeleteMail,
		IntegrationActionType_DeleteMails:          integration.DeleteMails,
		IntegrationActionType_ReplyMail:            integration.ReplyMail,
		IntegrationActionType_GetThread:            integration.GetThread,
		IntegrationActionType_GetThreads:           integration.GetThreads,
		IntegrationActionType_MarkMailAsRead:       integration.MarkMailAsRead,
		IntegrationActionType_MarkMailAsUnread:     integration.MarkMailAsUnread,
		IntegrationActionType_MarkMailsAsRead:      integration.MarkMailsAsRead,
		IntegrationActionType_MarkMailsAsUnread:    integration.MarkMailsAsUnread,
		IntegrationActionType_CreateDraft:          integration.CreateDraft,
		IntegrationActionType_GetDraft:             integration.GetDraft,
		IntegrationActionType_GetDrafts:            integration.GetDrafts,
		IntegrationActionType_SendDraft:            integration.SendDraft,
		IntegrationActionType_DeleteDraft:          integration.DeleteDraft,
		IntegrationActionType_DeleteDrafts:         integration.DeleteDrafts,
		IntegrationActionType_GetLabels:            integration.GetLabels,
		IntegrationActionType_AddLabel:             integration.AddLabel,
		IntegrationActionType_RemoveLabel:          integration.RemoveLabel,
		IntegrationActionType_AddLabelToEmail:      integration.AddLabelToEmail,
		IntegrationActionType_RemoveLabelFromEmail: integration.RemoveLabelFromEmail,
	}
	integration.actionFuncs = actionFuncs

	return integration, nil
}

func (g *GmailIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	actionFunc, ok := g.actionFuncs[params.ActionType]
	if !ok {
		return domain.IntegrationOutput{}, fmt.Errorf("unsupported action type: %s", params.ActionType)
	}
	return actionFunc(ctx, params)
}

type MultipleMailParams struct {
	Limit int64 `json:"limit"`
}

type MailParams struct {
	MessageID string `json:"message_id"`
}

type DraftsParams struct {
	DraftID string `json:"draft_id"`
}

type DraftParams struct {
	DraftID string `json:"draft_id"`
}

type SendMailParams struct {
	To      []string `json:"to"`
	Cc      []string `json:"cc"`
	Bcc     []string `json:"bcc"`
	Subject string   `json:"subject"`
	Body    string   `json:"body"`
}

type ReplyMailParams struct {
	MessageID string `json:"message_id"`
	Body      string `json:"body"`
}

type LabelToEmailParams struct {
	MessageID string `json:"message_id"`
	LabelID   string `json:"label_id"`
}

type LabelParams struct {
	LabelName string `json:"label_name"`
}

type RemoveLabelParams struct {
	LabelID string `json:"label_id"`
}

type FindMailParams struct {
	Query string `json:"query"`
	Limit int64  `json:"limit"`
}

func (g *GmailIntegration) SendMail(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	// Get items by input ID
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	log.Info().Msgf("Send Mail Items: %v", itemsByInputID)

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {
		// Bind item to SendMailParams
		p := SendMailParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		// Format raw email
		rawMessage, err := formatRawEmail(p)
		if err != nil {
			log.Warn().Err(err).Msg("failed to format raw email, skipping item")
			continue
		}

		// Send email
		sentMail, err := g.service.Users.Messages.Send("me", &gmail.Message{
			Raw: rawMessage,
		}).Do()
		if err != nil {
			return domain.IntegrationOutput{}, fmt.Errorf("failed to send email: %w", err)
		}

		// Append sent mail to output items
		outputItems = append(outputItems, sentMail)
	}

	if len(outputItems) == 0 {
		return domain.IntegrationOutput{}, fmt.Errorf("no mails sent")
	}

	// Marshal output items to JSON
	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, fmt.Errorf("failed to marshal output items: %w", err)
	}

	// Return the output
	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			domain.Payload(resultJSON),
		},
	}, nil
}

func (g *GmailIntegration) FindMail(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {
		p := FindMailParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		messages, err := g.service.Users.Messages.List("me").Q(p.Query).MaxResults(p.Limit).Do()
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		if messages == nil || messages.Messages == nil {
			return domain.IntegrationOutput{}, fmt.Errorf("no messages found for query: %s", p.Query)
		}

		if len(messages.Messages) == 0 {
			return domain.IntegrationOutput{}, fmt.Errorf("no messages found for query: %s", p.Query)
		}

		for _, message := range messages.Messages {
			m, err := g.service.Users.Messages.Get("me", message.Id).Do()
			if err != nil {
				return domain.IntegrationOutput{}, err
			}

			outputItems = append(outputItems, m)
		}
	}

	if len(outputItems) == 0 {
		return domain.IntegrationOutput{}, fmt.Errorf("no mails found")
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (g *GmailIntegration) GetMail(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {
		p := MailParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		m, err := g.service.Users.Messages.Get("me", p.MessageID).Do()
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		outputItems = append(outputItems, m)
	}

	if len(outputItems) == 0 {
		return domain.IntegrationOutput{}, fmt.Errorf("no mails found")
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (g *GmailIntegration) SendMailToTrash(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {
		p := MailParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		deletedMail, err := g.service.Users.Messages.Trash("me", p.MessageID).Do()
		if err != nil {
			return domain.IntegrationOutput{}, err
		}
		outputItems = append(outputItems, deletedMail)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (g *GmailIntegration) SendMailsToTrash(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {

		p := MultipleMailParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		messages, err := g.service.Users.Messages.List("me").MaxResults(p.Limit).Do()
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		var deletedMails []*gmail.Message
		for _, message := range messages.Messages {
			deletedMail, err := g.service.Users.Messages.Trash("me", message.Id).Do()
			if err != nil {
				return domain.IntegrationOutput{}, err
			}

			deletedMails = append(deletedMails, deletedMail)

			outputItems = append(outputItems, deletedMails)
		}

	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (g *GmailIntegration) DeleteMail(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {
		p := MailParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		err = g.service.Users.Messages.Delete("me", p.MessageID).Do()
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		outputItems = append(outputItems, p.MessageID)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (g *GmailIntegration) DeleteMails(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {
		p := MultipleMailParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		log.Info().Msgf("DeleteMails: %v", p)

		messages, err := g.service.Users.Messages.List("me").MaxResults(p.Limit).Do()
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		var deletedMails []*gmail.Message
		for _, message := range messages.Messages {
			err := g.service.Users.Messages.Delete("me", message.Id).Do()
			if err != nil {
				return domain.IntegrationOutput{}, err
			}

			outputItems = append(outputItems, deletedMails)
		}

	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

type GetMailItemOutput struct {
	ID      string `json:"id"`
	Snippet string `json:"snippet"`
	Raw     any    `json:"raw"`
}

func (g *GmailIntegration) GetMails(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {
		p := MultipleMailParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		// Fetch messages
		messages, err := g.service.Users.Messages.List("me").MaxResults(p.Limit).Do()
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		if messages == nil || messages.Messages == nil {
			return domain.IntegrationOutput{}, fmt.Errorf("no messages found")
		}

		if len(messages.Messages) == 0 {
			return domain.IntegrationOutput{}, fmt.Errorf("no messages found")
		}

		for _, message := range messages.Messages {
			m, err := g.service.Users.Messages.Get("me", message.Id).Do()
			if err != nil {
				return domain.IntegrationOutput{}, err
			}

			outputItems = append(outputItems, m)
		}

	}

	log.Info().Msgf("OutputItems: %v", outputItems)

	if len(outputItems) == 0 {
		return domain.IntegrationOutput{}, fmt.Errorf("no mails found")
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (g *GmailIntegration) ReplyMail(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {
		p := ReplyMailParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			log.Printf("Error binding item: %v", err)
			continue
		}

		if p.MessageID == "" {
			log.Printf("Skipping item with no message ID")
			continue
		}

		// Fetch the original message to get its details
		originalMessage, err := g.service.Users.Messages.Get("me", p.MessageID).Format("full").Do()
		if err != nil {
			log.Printf("Error fetching original message: %v", err)
			continue
		}

		// Extract headers from the original message
		var subject, from, messageID string
		for _, header := range originalMessage.Payload.Headers {
			switch header.Name {
			case "Subject":
				subject = header.Value
			case "From":
				from = header.Value
			case "Message-ID":
				messageID = header.Value
			}
		}

		// Extract email address from "From" header
		replyTo := extractEmailFromHeader(from)
		if replyTo == "" {
			log.Printf("Could not extract email address from From header: %s", from)
			continue
		}

		// Prepare reply subject
		replySubject := subject
		if !strings.HasPrefix(subject, "Re:") {
			replySubject = "Re: " + subject
		}

		// Build reply message
		var headers []string
		headers = append(headers, fmt.Sprintf("To: %s", replyTo))
		headers = append(headers, fmt.Sprintf("Subject: %s", replySubject))

		if messageID != "" {
			headers = append(headers, fmt.Sprintf("In-Reply-To: %s", messageID))
			headers = append(headers, fmt.Sprintf("References: %s", messageID))
		}

		headers = append(headers, "")
		headers = append(headers, p.Body)

		messageStr := strings.Join(headers, "\r\n")

		// Encode the message
		rawMessage := base64.URLEncoding.EncodeToString([]byte(messageStr))

		// Send the reply
		sentMail, err := g.service.Users.Messages.Send("me", &gmail.Message{
			Raw:      rawMessage,
			ThreadId: originalMessage.ThreadId,
		}).Do()

		if err != nil {
			log.Printf("Error sending reply: %v", err)
			continue
		}

		outputItems = append(outputItems, sentMail)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

// extractEmailFromHeader extracts email address from a header like "John Doe <john@example.com>"
func extractEmailFromHeader(header string) string {
	if header == "" {
		return ""
	}

	// Look for email in angle brackets
	if strings.Contains(header, "<") && strings.Contains(header, ">") {
		start := strings.Index(header, "<")
		end := strings.Index(header, ">")
		if start != -1 && end != -1 && end > start {
			email := header[start+1 : end]
			if strings.Contains(email, "@") {
				return email
			}
		}
	}

	// If no angle brackets, try to find an email pattern
	parts := strings.Fields(header)
	for _, part := range parts {
		if strings.Contains(part, "@") {
			if strings.Contains(part, "@") {
				return part
			}
		}
	}

	return ""
}

func (g *GmailIntegration) CreateDraft(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {
		p := SendMailParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		rawMessage, err := formatRawEmail(p)
		if err != nil {
			log.Printf("Error formatting raw email: %v", err)
			continue
		}
		// Send the message
		sentMail, err := g.service.Users.Drafts.Create("me", &gmail.Draft{
			Message: &gmail.Message{
				Raw: rawMessage,
			},
		}).Do()
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		outputItems = append(outputItems, sentMail)
	}
	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}
	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (g *GmailIntegration) MarkMailAsRead(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {
		p := MailParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		if p.MessageID == "" {
			log.Warn().Msg("Skipping item with empty message ID")
			continue
		}

		_, err = g.service.Users.Messages.Modify("me", p.MessageID, &gmail.ModifyMessageRequest{
			RemoveLabelIds: []string{"UNREAD"},
		}).Do()
		if err != nil {
			log.Error().Msgf("Error marking message as read: %v", err)
			continue
		}

		outputItems = append(outputItems, item)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (g *GmailIntegration) MarkMailAsUnread(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {
		p := MailParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		if p.MessageID == "" {
			log.Warn().Msg("Skipping item with empty message ID")
			continue
		}

		log.Info().Msgf("MarkMailAsUnread - Marking message %s as unread", p.MessageID)

		_, err = g.service.Users.Messages.Modify("me", p.MessageID, &gmail.ModifyMessageRequest{
			AddLabelIds: []string{"UNREAD"},
		}).Do()
		if err != nil {
			log.Error().Msgf("Error marking message as unread: %v", err)
			continue
		}

		outputItems = append(outputItems, item)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (g *GmailIntegration) MarkMailsAsRead(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {
		p := MultipleMailParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		log.Info().Msgf("MarkMailsAsRead - Limit: %d", p.Limit)

		messages, err := g.service.Users.Messages.List("me").MaxResults(p.Limit).Do()
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		if messages == nil || messages.Messages == nil {
			log.Info().Msgf("MarkMailsAsRead - No messages found")
			continue
		}

		if len(messages.Messages) == 0 {
			log.Info().Msgf("MarkMailsAsRead - No messages found")
			continue
		}

		log.Info().Msgf("MarkMailsAsRead - Found %d messages", len(messages.Messages))

		for _, message := range messages.Messages {
			if message.Id == "" {
				log.Warn().Msg("Skipping message with empty ID")
				continue
			}

			log.Info().Msgf("MarkMailsAsRead - Marking message %s as read", message.Id)

			_, err := g.service.Users.Messages.Modify("me", message.Id, &gmail.ModifyMessageRequest{
				RemoveLabelIds: []string{"UNREAD"},
			}).Do()
			if err != nil {
				log.Error().Msgf("Error marking message as read: %v", err)
				continue
			}
		}

		outputItems = append(outputItems, item)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (g *GmailIntegration) MarkMailsAsUnread(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {
		p := MultipleMailParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		log.Info().Msgf("MarkMailsAsUnread - Limit: %d", p.Limit)

		messages, err := g.service.Users.Messages.List("me").MaxResults(p.Limit).Do()
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		if messages == nil || messages.Messages == nil {
			log.Info().Msgf("MarkMailsAsUnread - No messages found")
			continue
		}

		if len(messages.Messages) == 0 {
			log.Info().Msgf("MarkMailsAsUnread - No messages found")
			continue
		}

		log.Info().Msgf("MarkMailsAsUnread - Found %d messages", len(messages.Messages))

		for _, message := range messages.Messages {
			if message.Id == "" {
				log.Warn().Msg("Skipping message with empty ID")
				continue
			}

			log.Info().Msgf("MarkMailsAsUnread - Marking message %s as unread", message.Id)

			_, err := g.service.Users.Messages.Modify("me", message.Id, &gmail.ModifyMessageRequest{
				AddLabelIds: []string{"UNREAD"},
			}).Do()
			if err != nil {
				log.Error().Msgf("Error marking message as unread: %v", err)
				continue
			}
		}

		outputItems = append(outputItems, item)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (g *GmailIntegration) GetDraft(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {
		p := MailParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		d, err := g.service.Users.Drafts.Get("me", p.MessageID).Do()
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		outputItems = append(outputItems, d)
	}

	if len(outputItems) == 0 {
		return domain.IntegrationOutput{}, fmt.Errorf("no drafts found")
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (g *GmailIntegration) GetDrafts(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {
		p := MultipleMailParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		drafts, err := g.service.Users.Drafts.List("me").MaxResults(p.Limit).Do()
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		if drafts == nil || drafts.Drafts == nil {
			return domain.IntegrationOutput{}, fmt.Errorf("no drafts found")
		}

		if len(drafts.Drafts) == 0 {
			return domain.IntegrationOutput{}, fmt.Errorf("no drafts found")
		}

		for _, draft := range drafts.Drafts {
			d, err := g.service.Users.Drafts.Get("me", draft.Id).Do()
			if err != nil {
				return domain.IntegrationOutput{}, err
			}

			outputItems = append(outputItems, d)
		}

	}

	if len(outputItems) == 0 {
		return domain.IntegrationOutput{}, fmt.Errorf("no drafts found")
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (g *GmailIntegration) SendDraft(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {
		p := DraftParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		d, err := g.service.Users.Drafts.Get("me", p.DraftID).Do()
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		log.Printf("Fetching draft with MessageID: %s", p.DraftID)

		if d.Message == nil || d.Message.Id == "" {
			return domain.IntegrationOutput{}, fmt.Errorf("draft message is empty or missing ID")
		}

		m, err := g.service.Users.Messages.Get("me", d.Message.Id).Format("raw").Do()
		if err != nil {
			return domain.IntegrationOutput{}, fmt.Errorf("failed to fetch message with raw content: %v", err)
		}

		decodedRaw, err := base64.URLEncoding.DecodeString(m.Raw)
		if err != nil {
			return domain.IntegrationOutput{}, fmt.Errorf("failed to decode raw message: %v", err)
		}

		encodedRaw := base64.URLEncoding.EncodeToString(decodedRaw)

		msg := &gmail.Message{
			Raw: encodedRaw,
		}

		_, err = g.service.Users.Messages.Send("me", msg).Do()
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		outputItems = append(outputItems, d)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (g *GmailIntegration) DeleteDraft(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	log.Info().Msgf("DeleteDraft - All items: %+v", allItems)

	for _, item := range allItems {
		p := DraftParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		err = g.service.Users.Drafts.Delete("me", p.DraftID).Do()
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		outputItems = append(outputItems, p.DraftID)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (g *GmailIntegration) DeleteDrafts(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {
		p := MultipleMailParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		drafts, err := g.service.Users.Drafts.List("me").MaxResults(p.Limit).Do()
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		for _, draft := range drafts.Drafts {
			err := g.service.Users.Drafts.Delete("me", draft.Id).Do()
			if err != nil {
				return domain.IntegrationOutput{}, err
			}
			outputItems = append(outputItems, draft)
		}
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (g *GmailIntegration) GetLabels(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {
		p := MultipleMailParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		labels, err := g.service.Users.Labels.List("me").Do()
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		if labels == nil || labels.Labels == nil {
			return domain.IntegrationOutput{}, fmt.Errorf("no labels found")
		}

		if len(labels.Labels) == 0 {
			return domain.IntegrationOutput{}, fmt.Errorf("no labels found")
		}

		result := make([]any, 0, len(labels.Labels))
		for i, label := range labels.Labels {
			if int64(i) >= p.Limit {
				break
			}
			log.Info().Msgf("Label: %v", label)
			result = append(result, label)
		}

		if len(result) == 0 {
			return domain.IntegrationOutput{}, fmt.Errorf("no labels found within limit")
		}

		outputItems = append(outputItems, result)
	}

	if len(outputItems) == 0 {
		return domain.IntegrationOutput{}, fmt.Errorf("no labels found")
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (g *GmailIntegration) AddLabel(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {
		p := LabelParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		_, err = g.service.Users.Labels.Create("me", &gmail.Label{
			Name: p.LabelName,
		}).Do()
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		outputItems = append(outputItems, item)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (g *GmailIntegration) RemoveLabel(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {
		p := RemoveLabelParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		labels, err := g.service.Users.Labels.List("me").Do()
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		for _, label := range labels.Labels {
			log.Info().Msgf("Label ID: %s, Name: %s", label.Id, label.Name)
		}

		log.Info().Msgf("Deleting label: %s", p.LabelID)

		err = g.service.Users.Labels.Delete("me", p.LabelID).Do()
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		outputItems = append(outputItems, item)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (g *GmailIntegration) AddLabelToEmail(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {
		p := LabelToEmailParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		_, err = g.service.Users.Messages.Modify("me", p.MessageID, &gmail.ModifyMessageRequest{
			AddLabelIds: []string{p.LabelID},
		}).Do()
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		outputItems = append(outputItems, item)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (g *GmailIntegration) RemoveLabelFromEmail(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {
		p := LabelToEmailParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		_, err = g.service.Users.Messages.Modify("me", p.MessageID, &gmail.ModifyMessageRequest{
			RemoveLabelIds: []string{p.LabelID},
		}).Do()
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		outputItems = append(outputItems, item)
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (g *GmailIntegration) GetThreads(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {
		p := MultipleMailParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)

		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		threads, err := g.service.Users.Threads.List("me").MaxResults(p.Limit).Do()
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		if threads == nil || threads.Threads == nil {
			return domain.IntegrationOutput{}, fmt.Errorf("no threads found")
		}

		if len(threads.Threads) == 0 {
			return domain.IntegrationOutput{}, fmt.Errorf("no threads found")
		}

		for _, thread := range threads.Threads {
			t, err := g.service.Users.Threads.Get("me", thread.Id).Do()
			if err != nil {
				return domain.IntegrationOutput{}, err
			}

			outputItems = append(outputItems, t)
		}

		if len(outputItems) == 0 {
			return domain.IntegrationOutput{}, fmt.Errorf("no threads found")
		}

	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func (g *GmailIntegration) GetThread(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	itemsByInputID, err := params.GetItemsByInputID()
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	allItems := make([]any, 0)

	for _, items := range itemsByInputID {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	outputItems := make([]domain.Item, 0)

	for _, item := range allItems {
		p := MailParams{}
		err := g.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		t, err := g.service.Users.Threads.Get("me", p.MessageID).Do()
		if err != nil {
			return domain.IntegrationOutput{}, err
		}

		if t.Messages == nil {
			return domain.IntegrationOutput{}, fmt.Errorf("thread messages are nil")
		}
		if len(t.Messages) == 0 {
			return domain.IntegrationOutput{}, fmt.Errorf("thread messages are empty")
		}

		outputItems = append(outputItems, t)
	}

	if len(outputItems) == 0 {
		return domain.IntegrationOutput{}, fmt.Errorf("no threads found")
	}

	resultJSON, err := json.Marshal(outputItems)
	if err != nil {
		return domain.IntegrationOutput{}, err
	}

	return domain.IntegrationOutput{
		ResultJSONByOutputID: []domain.Payload{
			resultJSON,
		},
	}, nil
}

func formatRawEmail(p SendMailParams) (string, error) {
	// Basic validation - ensure we have at least one recipient
	if len(p.To) == 0 {
		return "", fmt.Errorf("no recipients provided")
	}

	// Validate email addresses using the isValidEmail function
	var validTo []string
	for _, email := range p.To {
		if strings.Contains(email, "@") {
			validTo = append(validTo, email)
		} else {
			log.Warn().Msgf("Skipping invalid email address: %s", email)
		}
	}

	// Skip if no valid recipients left
	if len(validTo) == 0 {
		return "", fmt.Errorf("no valid recipients found")
	}

	// Similarly validate Cc and Bcc
	var validCc, validBcc []string
	for _, email := range p.Cc {
		if strings.Contains(email, "@") {
			validCc = append(validCc, email)
		} else {
			log.Warn().Msgf("Skipping invalid CC email address: %s", email)
		}
	}

	for _, email := range p.Bcc {
		if strings.Contains(email, "@") {
			validBcc = append(validBcc, email)
		} else {
			log.Warn().Msgf("Skipping invalid BCC email address: %s", email)
		}
	}

	to := strings.Join(validTo, ", ")
	cc := strings.Join(validCc, ", ")
	bcc := strings.Join(validBcc, ", ")

	var headers []string
	headers = append(headers, fmt.Sprintf("To: %s", to))

	if len(validCc) > 0 {
		headers = append(headers, fmt.Sprintf("Cc: %s", cc))
	}

	if len(validBcc) > 0 {
		headers = append(headers, fmt.Sprintf("Bcc: %s", bcc))
	}

	headers = append(headers, fmt.Sprintf("Subject: %s", p.Subject))
	headers = append(headers, "")
	headers = append(headers, p.Body)

	messageStr := strings.Join(headers, "\r\n")

	rawMessage := base64.URLEncoding.EncodeToString([]byte(messageStr))

	return rawMessage, nil
}

// func isValidEmail(email string) bool {
// 	_, err := mail.ParseAddress(email)
// 	return err == nil
// }
