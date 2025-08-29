package stripe

import (
	"context"
	"encoding/json"
	"flowbaker/internal/domain"
	"flowbaker/internal/managers"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/balance"
	"github.com/stripe/stripe-go/v82/charge"
	"github.com/stripe/stripe-go/v82/coupon"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/paymentintent"
	"github.com/stripe/stripe-go/v82/paymentmethod"
	"github.com/stripe/stripe-go/v82/source"
)

type StripeIntegrationCreator struct {
	credentialGetter domain.CredentialGetter[StripeCredential]
	binder           domain.IntegrationParameterBinder
}

func NewStripeIntegrationCreator(deps domain.IntegrationDeps) domain.IntegrationCreator {
	return &StripeIntegrationCreator{
		credentialGetter: managers.NewExecutorCredentialGetter[StripeCredential](deps.ExecutorCredentialManager),
		binder:           deps.ParameterBinder,
	}
}

func (c *StripeIntegrationCreator) CreateIntegration(ctx context.Context, p domain.CreateIntegrationParams) (domain.IntegrationExecutor, error) {

	return NewStripeIntegration(StripeIntegrationDependencies{
		CredentialGetter: c.credentialGetter,
		ParameterBinder:  c.binder,
		CredentialID:     p.CredentialID,
	})
}

type StripeIntegration struct {
	binder           domain.IntegrationParameterBinder
	credentialGetter domain.CredentialGetter[StripeCredential]
	actionManager    *domain.IntegrationActionManager
	credentialID     string
	// Add thread-safe cached credential
	cachedCredential *StripeCredential
	credentialCached bool
	cacheMutex       sync.RWMutex
}

type StripeCredential struct {
	SecretKey             string `json:"secret_key"`
	WebhookSecretSnapshot string `json:"webhook_secret_snapshot"`
	WebhookSecretThin     string `json:"webhook_secret_thin"`
}

type StripeIntegrationDependencies struct {
	ParameterBinder  domain.IntegrationParameterBinder
	CredentialID     string
	CredentialGetter domain.CredentialGetter[StripeCredential]
}

func NewStripeIntegration(deps StripeIntegrationDependencies) (*StripeIntegration, error) {
	integration := &StripeIntegration{
		credentialGetter: deps.CredentialGetter,
		binder:           deps.ParameterBinder,
		credentialID:     deps.CredentialID,
		actionManager:    domain.NewIntegrationActionManager(),
	}

	actionManager := domain.NewIntegrationActionManager().
		// Balance actions
		AddPerItem(IntegrationActionType_GetBalance, integration.GetBalance).
		// Payment Intents API (Modern)
		AddPerItem(IntegrationActionType_CreatePaymentIntent, integration.CreatePaymentIntent).
		AddPerItem(IntegrationActionType_ConfirmPaymentIntent, integration.ConfirmPaymentIntent).
		AddPerItem(IntegrationActionType_GetPaymentIntent, integration.GetPaymentIntent).
		AddPerItemMulti(IntegrationActionType_GetManyPaymentIntents, integration.GetManyPaymentIntents).
		AddPerItem(IntegrationActionType_UpdatePaymentIntent, integration.UpdatePaymentIntent).
		AddPerItem(IntegrationActionType_CancelPaymentIntent, integration.CancelPaymentIntent).
		// Payment Methods API

		AddPerItem(IntegrationActionType_GetPaymentMethod, integration.GetPaymentMethod).
		// Legacy Charge actions
		AddPerItem(IntegrationActionType_CreateCharge, integration.CreateCharge).
		AddPerItem(IntegrationActionType_GetCharge, integration.GetCharge).
		AddPerItemMulti(IntegrationActionType_GetManyCharges, integration.GetManyCharges).
		AddPerItem(IntegrationActionType_UpdateCharge, integration.UpdateCharge).
		// Coupon actions
		AddPerItem(IntegrationActionType_CreateCoupon, integration.CreateCoupon).
		AddPerItemMulti(IntegrationActionType_GetManyCoupons, integration.GetManyCoupons).
		// Customer actions
		AddPerItem(IntegrationActionType_CreateCustomer, integration.CreateCustomer).
		AddPerItem(IntegrationActionType_DeleteCustomer, integration.DeleteCustomer).
		AddPerItem(IntegrationActionType_GetCustomer, integration.GetCustomer).
		AddPerItemMulti(IntegrationActionType_GetManyCustomers, integration.GetManyCustomers).
		AddPerItem(IntegrationActionType_UpdateCustomer, integration.UpdateCustomer).
		// Source actions
		AddPerItem(IntegrationActionType_GetSource, integration.GetSource)

	integration.actionManager = actionManager

	return integration, nil
}

func (i *StripeIntegration) Execute(ctx context.Context, params domain.IntegrationInput) (domain.IntegrationOutput, error) {
	return i.actionManager.Run(ctx, params.ActionType, params)
}

// Add method to get cached credential with thread safety
func (i *StripeIntegration) getCredential(ctx context.Context) (*StripeCredential, error) {
	// Check cache first with read lock
	i.cacheMutex.RLock()
	if i.credentialCached && i.cachedCredential != nil {
		i.cacheMutex.RUnlock()
		return i.cachedCredential, nil
	}
	i.cacheMutex.RUnlock()

	// Acquire write lock for cache update
	i.cacheMutex.Lock()
	defer i.cacheMutex.Unlock()

	// Double check after acquiring write lock
	if i.credentialCached && i.cachedCredential != nil {
		return i.cachedCredential, nil
	}

	// Get decrypted credential
	decryptedCredential, err := i.credentialGetter.GetDecryptedCredential(ctx, i.credentialID)
	if err != nil {
		return nil, fmt.Errorf("failed to get credential: %w", err)
	}

	// Cache the credential
	i.cachedCredential = &decryptedCredential
	i.credentialCached = true

	return &decryptedCredential, nil
}

// Parameter structs for actions
type GetBalanceParams struct {
	CredentialID string `json:"credential_id"`
}

// Payment Intents API Parameters
type CreatePaymentIntentParams struct {
	CredentialID     string `json:"credential_id"`
	Amount           string `json:"amount"`
	Currency         string `json:"currency"`
	PaymentMethod    string `json:"payment_method"`
	Customer         string `json:"customer"`
	Description      string `json:"description"`
	AutoConfirm      bool   `json:"auto_confirm"`
	ReturnURL        string `json:"return_url"`
	DisableRedirects bool   `json:"disable_redirects"`
}

type ConfirmPaymentIntentParams struct {
	CredentialID    string `json:"credential_id"`
	PaymentIntentID string `json:"payment_intent_id"`
	PaymentMethod   string `json:"payment_method"`
	ReturnURL       string `json:"return_url"`
}

type GetPaymentIntentParams struct {
	CredentialID    string `json:"credential_id"`
	PaymentIntentID string `json:"payment_intent_id"`
}

type GetManyPaymentIntentsParams struct {
	CredentialID string `json:"credential_id"`
	Customer     string `json:"customer"`
	Created      string `json:"created"`
	AfterDate    string `json:"after_date"`
	BeforeDate   string `json:"before_date"`
}

type UpdatePaymentIntentParams struct {
	CredentialID    string `json:"credential_id"`
	PaymentIntentID string `json:"payment_intent_id"`
	Amount          string `json:"amount"`
	Description     string `json:"description"`
	Metadata        string `json:"metadata"`
}

type CancelPaymentIntentParams struct {
	CredentialID       string `json:"credential_id"`
	PaymentIntentID    string `json:"payment_intent_id"`
	CancellationReason string `json:"cancellation_reason"`
}

// Payment Methods API Parameters

type GetPaymentMethodParams struct {
	CredentialID    string `json:"credential_id"`
	PaymentMethodID string `json:"payment_method_id"`
}

type CreateChargeParams struct {
	CredentialID string `json:"credential_id"`
	Amount       string `json:"amount"`
	Currency     string `json:"currency"`
	Source       string `json:"source"`
	Description  string `json:"description"`
	Customer     string `json:"customer"`
}

type GetChargeParams struct {
	CredentialID string `json:"credential_id"`
	ChargeID     string `json:"charge_id"`
}

type GetManyChargesParams struct {
	CredentialID string `json:"credential_id"`
	Customer     string `json:"customer"`
	AfterDate    string `json:"after_date"`
	BeforeDate   string `json:"before_date"`
}

type UpdateChargeParams struct {
	CredentialID              string `json:"credential_id"`
	ChargeID                  string `json:"charge_id"`
	Customer                  string `json:"customer"`
	Description               string `json:"description"`
	ReceiptEmail              string `json:"receipt_email"`
	ShippingName              string `json:"shipping_name"`
	ShippingPhone             string `json:"shipping_phone"`
	ShippingAddressLine1      string `json:"shipping_address_line1"`
	ShippingAddressLine2      string `json:"shipping_address_line2"`
	ShippingAddressCity       string `json:"shipping_address_city"`
	ShippingAddressState      string `json:"shipping_address_state"`
	ShippingAddressPostalCode string `json:"shipping_address_postal_code"`
	ShippingAddressCountry    string `json:"shipping_address_country"`
	Metadata                  string `json:"metadata"`
}

type CreateCouponParams struct {
	CredentialID     string `json:"credential_id"`
	ID               string `json:"id"`
	PercentOff       string `json:"percent_off"`
	AmountOff        string `json:"amount_off"`
	Currency         string `json:"currency"`
	Duration         string `json:"duration"`
	DurationInMonths string `json:"duration_in_months"`
}

type GetManyCouponsParams struct {
	CredentialID string `json:"credential_id"`
	AfterDate    string `json:"after_date"`
	BeforeDate   string `json:"before_date"`
}

type CreateCustomerParams struct {
	CredentialID string `json:"credential_id"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	Phone        string `json:"phone"`
	Description  string `json:"description"`
}

type DeleteCustomerParams struct {
	CredentialID string `json:"credential_id"`
	CustomerID   string `json:"customer_id"`
}

type GetCustomerParams struct {
	CredentialID string `json:"credential_id"`
	CustomerID   string `json:"customer_id"`
}

type GetManyCustomersParams struct {
	CredentialID string `json:"credential_id"`
	Limit        string `json:"limit"`
	Email        string `json:"email"`
	AfterDate    string `json:"after_date"`
	BeforeDate   string `json:"before_date"`
}

type UpdateCustomerParams struct {
	CredentialID  string `json:"credential_id"`
	CustomerID    string `json:"customer_id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	Phone         string `json:"phone"`
	Description   string `json:"description"`
	DefaultSource string `json:"default_source"`
}

type GetCustomerCardParams struct {
	CredentialID string `json:"credential_id"`
	CustomerID   string `json:"customer_id"`
	CardID       string `json:"card_id"`
}

type CreateSourceParams struct {
	CredentialID string `json:"credential_id"`
	Type         string `json:"type"`
	Currency     string `json:"currency"`
	Amount       string `json:"amount"`
	Usage        string `json:"usage"`
	CardNumber   string `json:"card_number"`
	CardExpMonth string `json:"card_exp_month"`
	CardExpYear  string `json:"card_exp_year"`
	CardCVC      string `json:"card_cvc"`
	OwnerEmail   string `json:"owner_email"`
}

type GetSourceParams struct {
	CredentialID string `json:"credential_id"`
	SourceID     string `json:"source_id"`
}

// Action implementations
func (i *StripeIntegration) GetBalance(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetBalanceParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	// Get credential with cached approach
	credentialData, err := i.getCredential(ctx)
	if err != nil {
		return nil, err
	}

	// Set the API key for stripe-go
	stripe.Key = credentialData.SecretKey

	// Use stripe-go to get balance
	bal, err := balance.Get(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	return bal, nil
}

func (i *StripeIntegration) CreateCharge(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := CreateChargeParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.Amount) == "" || strings.TrimSpace(p.Currency) == "" || strings.TrimSpace(p.Source) == "" {
		return nil, fmt.Errorf("amount, currency, and source are required")
	}

	// Get credential with cached approach
	credentialData, err := i.getCredential(ctx)
	if err != nil {
		return nil, err
	}

	// Set the API key for stripe-go
	stripe.Key = credentialData.SecretKey

	// Convert amount string to int64 (Stripe expects amount in cents)
	amount, err := strconv.ParseInt(p.Amount, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}

	// Create charge parameters
	chargeParams := &stripe.ChargeParams{
		Amount:   stripe.Int64(amount),
		Currency: stripe.String(p.Currency),
		Source:   &stripe.PaymentSourceSourceParams{Token: stripe.String(p.Source)},
	}

	if p.Description != "" {
		chargeParams.Description = stripe.String(p.Description)
	}
	if p.Customer != "" {
		chargeParams.Customer = stripe.String(p.Customer)
	}

	// Create the charge using stripe-go
	ch, err := charge.New(chargeParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create charge: %w", err)
	}

	return ch, nil
}

func (i *StripeIntegration) GetCharge(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetChargeParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.ChargeID) == "" {
		return nil, fmt.Errorf("charge ID is required")
	}

	// Get credential with cached approach
	credentialData, err := i.getCredential(ctx)
	if err != nil {
		return nil, err
	}

	// Set the API key for stripe-go
	stripe.Key = credentialData.SecretKey

	// Get the charge using stripe-go
	ch, err := charge.Get(p.ChargeID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get charge: %w", err)
	}

	return ch, nil
}

func (i *StripeIntegration) GetManyCharges(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	p := GetManyChargesParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	// Get credential with cached approach
	credentialData, err := i.getCredential(ctx)
	if err != nil {
		return nil, err
	}

	// Set the API key for stripe-go
	stripe.Key = credentialData.SecretKey

	// Create list parameters
	listParams := &stripe.ChargeListParams{}

	// Set customer filter if provided
	if p.Customer != "" {
		listParams.Customer = stripe.String(p.Customer)
	}

	// Handle date parameters
	if p.AfterDate != "" {
		if timestamp, err := i.parseDateToTimestamp(p.AfterDate); err == nil {
			listParams.Filters.AddFilter("created[gte]", "", strconv.FormatInt(timestamp, 10))
		} else {
			return nil, fmt.Errorf("invalid after_date: %w", err)
		}
	}

	if p.BeforeDate != "" {
		if timestamp, err := i.parseDateToTimestamp(p.BeforeDate); err == nil {
			listParams.Filters.AddFilter("created[lte]", "", strconv.FormatInt(timestamp, 10))
		} else {
			return nil, fmt.Errorf("invalid before_date: %w", err)
		}
	}

	// Get charges using stripe-go
	iter := charge.List(listParams)

	var items []domain.Item
	for iter.Next() {
		ch := iter.Charge()
		items = append(items, ch)
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to get charges: %w", err)
	}

	return items, nil
}

func (i *StripeIntegration) UpdateCharge(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := UpdateChargeParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.ChargeID) == "" {
		return nil, fmt.Errorf("charge ID is required")
	}

	// Get credential with cached approach
	credentialData, err := i.getCredential(ctx)
	if err != nil {
		return nil, err
	}

	// Set the API key for stripe-go
	stripe.Key = credentialData.SecretKey

	// Create update parameters
	updateParams := &stripe.ChargeParams{}
	hasUpdates := false

	if p.Customer != "" {
		updateParams.Customer = stripe.String(p.Customer)
		hasUpdates = true
	}
	if p.Description != "" {
		updateParams.Description = stripe.String(p.Description)
		hasUpdates = true
	}
	if p.ReceiptEmail != "" {
		updateParams.ReceiptEmail = stripe.String(p.ReceiptEmail)
		hasUpdates = true
	}

	// Handle shipping information as an object
	if p.ShippingName != "" || p.ShippingPhone != "" || p.ShippingAddressLine1 != "" ||
		p.ShippingAddressLine2 != "" || p.ShippingAddressCity != "" || p.ShippingAddressState != "" ||
		p.ShippingAddressPostalCode != "" || p.ShippingAddressCountry != "" {

		shipping := &stripe.ShippingDetailsParams{}

		if p.ShippingName != "" {
			shipping.Name = stripe.String(p.ShippingName)
			hasUpdates = true
		}
		if p.ShippingPhone != "" {
			shipping.Phone = stripe.String(p.ShippingPhone)
			hasUpdates = true
		}

		if p.ShippingAddressLine1 != "" || p.ShippingAddressLine2 != "" || p.ShippingAddressCity != "" ||
			p.ShippingAddressState != "" || p.ShippingAddressPostalCode != "" || p.ShippingAddressCountry != "" {

			address := &stripe.AddressParams{}
			if p.ShippingAddressLine1 != "" {
				address.Line1 = stripe.String(p.ShippingAddressLine1)
			}
			if p.ShippingAddressLine2 != "" {
				address.Line2 = stripe.String(p.ShippingAddressLine2)
			}
			if p.ShippingAddressCity != "" {
				address.City = stripe.String(p.ShippingAddressCity)
			}
			if p.ShippingAddressState != "" {
				address.State = stripe.String(p.ShippingAddressState)
			}
			if p.ShippingAddressPostalCode != "" {
				address.PostalCode = stripe.String(p.ShippingAddressPostalCode)
			}
			if p.ShippingAddressCountry != "" {
				address.Country = stripe.String(p.ShippingAddressCountry)
			}
			shipping.Address = address
			hasUpdates = true
		}

		updateParams.Shipping = shipping
	}

	if p.Metadata != "" {
		// Parse metadata JSON and add to params
		var metadata map[string]string
		if err := json.Unmarshal([]byte(p.Metadata), &metadata); err == nil {
			updateParams.Metadata = metadata
			hasUpdates = true
		}
	}

	if !hasUpdates {
		return nil, fmt.Errorf("at least one field must be updated")
	}

	// Update the charge using stripe-go
	ch, err := charge.Update(p.ChargeID, updateParams)
	if err != nil {
		return nil, fmt.Errorf("failed to update charge: %w", err)
	}

	return ch, nil
}

func (i *StripeIntegration) CreateCoupon(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := CreateCouponParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.Duration) == "" {
		return nil, fmt.Errorf("duration is required")
	}

	if p.PercentOff == "" && p.AmountOff == "" {
		return nil, fmt.Errorf("either percent_off or amount_off must be specified")
	}

	// Get credential with cached approach
	credentialData, err := i.getCredential(ctx)
	if err != nil {
		return nil, err
	}

	// Set the API key for stripe-go
	stripe.Key = credentialData.SecretKey

	// Create coupon parameters
	couponParams := &stripe.CouponParams{
		Duration: stripe.String(p.Duration),
	}

	if p.ID != "" {
		couponParams.ID = stripe.String(p.ID)
	}

	if p.PercentOff != "" {
		if percentOff, err := strconv.ParseFloat(p.PercentOff, 64); err == nil {
			couponParams.PercentOff = stripe.Float64(percentOff)
		} else {
			return nil, fmt.Errorf("invalid percent_off: %w", err)
		}
	}

	if p.AmountOff != "" {
		if amountOff, err := strconv.ParseInt(p.AmountOff, 10, 64); err == nil {
			couponParams.AmountOff = stripe.Int64(amountOff)
			if p.Currency != "" {
				couponParams.Currency = stripe.String(p.Currency)
			}
		} else {
			return nil, fmt.Errorf("invalid amount_off: %w", err)
		}
	}

	if p.DurationInMonths != "" {
		if durationInMonths, err := strconv.ParseInt(p.DurationInMonths, 10, 64); err == nil {
			couponParams.DurationInMonths = stripe.Int64(durationInMonths)
		} else {
			return nil, fmt.Errorf("invalid duration_in_months: %w", err)
		}
	}

	// Create the coupon using stripe-go
	cp, err := coupon.New(couponParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create coupon: %w", err)
	}

	return cp, nil
}

func (i *StripeIntegration) GetManyCoupons(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	p := GetManyCouponsParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	// Get credential with cached approach
	credentialData, err := i.getCredential(ctx)
	if err != nil {
		return nil, err
	}

	// Set the API key for stripe-go
	stripe.Key = credentialData.SecretKey

	// Create list parameters
	listParams := &stripe.CouponListParams{}

	// Handle date parameters
	if p.AfterDate != "" {
		if timestamp, err := i.parseDateToTimestamp(p.AfterDate); err == nil {
			listParams.Filters.AddFilter("created[gte]", "", strconv.FormatInt(timestamp, 10))
		} else {
			return nil, fmt.Errorf("invalid after_date: %w", err)
		}
	}

	if p.BeforeDate != "" {
		if timestamp, err := i.parseDateToTimestamp(p.BeforeDate); err == nil {
			listParams.Filters.AddFilter("created[lte]", "", strconv.FormatInt(timestamp, 10))
		} else {
			return nil, fmt.Errorf("invalid before_date: %w", err)
		}
	}

	// Get coupons using stripe-go
	iter := coupon.List(listParams)

	var items []domain.Item
	for iter.Next() {
		cp := iter.Coupon()
		items = append(items, cp)
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to get coupons: %w", err)
	}

	return items, nil
}

func (i *StripeIntegration) CreateCustomer(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := CreateCustomerParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	// Get credential with cached approach
	credentialData, err := i.getCredential(ctx)
	if err != nil {
		return nil, err
	}

	// Set the API key for stripe-go
	stripe.Key = credentialData.SecretKey

	// Create customer parameters
	customerParams := &stripe.CustomerParams{}

	if p.Email != "" {
		customerParams.Email = stripe.String(p.Email)
	}
	if p.Name != "" {
		customerParams.Name = stripe.String(p.Name)
	}
	if p.Phone != "" {
		customerParams.Phone = stripe.String(p.Phone)
	}
	if p.Description != "" {
		customerParams.Description = stripe.String(p.Description)
	}

	// Create the customer using stripe-go
	cust, err := customer.New(customerParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create customer: %w", err)
	}

	return cust, nil
}

func (i *StripeIntegration) DeleteCustomer(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := DeleteCustomerParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.CustomerID) == "" {
		return nil, fmt.Errorf("customer ID is required")
	}

	// Get credential with cached approach
	credentialData, err := i.getCredential(ctx)
	if err != nil {
		return nil, err
	}

	// Set the API key for stripe-go
	stripe.Key = credentialData.SecretKey

	// Delete the customer using stripe-go
	cust, err := customer.Del(p.CustomerID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to delete customer: %w", err)
	}

	return cust, nil
}

func (i *StripeIntegration) GetCustomer(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetCustomerParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.CustomerID) == "" {
		return nil, fmt.Errorf("customer ID is required")
	}

	// Get credential with cached approach
	credentialData, err := i.getCredential(ctx)
	if err != nil {
		return nil, err
	}

	// Set the API key for stripe-go
	stripe.Key = credentialData.SecretKey

	// Get the customer using stripe-go
	cust, err := customer.Get(p.CustomerID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get customer: %w", err)
	}

	return cust, nil
}

func (i *StripeIntegration) GetManyCustomers(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	p := GetManyCustomersParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	// Get credential with cached approach
	credentialData, err := i.getCredential(ctx)
	if err != nil {
		return nil, err
	}

	// Set the API key for stripe-go
	stripe.Key = credentialData.SecretKey

	// Create list parameters
	listParams := &stripe.CustomerListParams{}

	// Set limit
	limit := int64(10)
	if p.Limit != "" {
		if parsed, err := strconv.ParseInt(p.Limit, 10, 64); err == nil {
			limit = parsed
			if limit > 100 {
				limit = 100
			}
			if limit < 1 {
				limit = 1
			}
		}
	}
	listParams.Limit = stripe.Int64(limit)

	// Set email filter if provided
	if p.Email != "" {
		listParams.Email = stripe.String(p.Email)
	}

	// Handle date parameters
	if p.AfterDate != "" {
		if timestamp, err := i.parseDateToTimestamp(p.AfterDate); err == nil {
			listParams.Filters.AddFilter("created[gte]", "", strconv.FormatInt(timestamp, 10))
		} else {
			return nil, fmt.Errorf("invalid after_date: %w", err)
		}
	}

	if p.BeforeDate != "" {
		if timestamp, err := i.parseDateToTimestamp(p.BeforeDate); err == nil {
			listParams.Filters.AddFilter("created[lte]", "", strconv.FormatInt(timestamp, 10))
		} else {
			return nil, fmt.Errorf("invalid before_date: %w", err)
		}
	}

	// Get customers using stripe-go
	iter := customer.List(listParams)

	var items []domain.Item
	for iter.Next() {
		cust := iter.Customer()
		items = append(items, cust)
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to get customers: %w", err)
	}

	return items, nil
}

func (i *StripeIntegration) UpdateCustomer(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := UpdateCustomerParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.CustomerID) == "" {
		return nil, fmt.Errorf("customer ID is required")
	}

	// Get credential with cached approach
	credentialData, err := i.getCredential(ctx)
	if err != nil {
		return nil, err
	}

	// Set the API key for stripe-go
	stripe.Key = credentialData.SecretKey

	// Create update parameters
	updateParams := &stripe.CustomerParams{}
	hasUpdates := false

	if p.Email != "" {
		updateParams.Email = stripe.String(p.Email)
		hasUpdates = true
	}
	if p.Name != "" {
		updateParams.Name = stripe.String(p.Name)
		hasUpdates = true
	}
	if p.Phone != "" {
		updateParams.Phone = stripe.String(p.Phone)
		hasUpdates = true
	}
	if p.Description != "" {
		updateParams.Description = stripe.String(p.Description)
		hasUpdates = true
	}
	if p.DefaultSource != "" {
		updateParams.DefaultSource = stripe.String(p.DefaultSource)
		hasUpdates = true
	}

	if !hasUpdates {
		return nil, fmt.Errorf("at least one field must be updated")
	}

	// Update the customer using stripe-go
	cust, err := customer.Update(p.CustomerID, updateParams)
	if err != nil {
		return nil, fmt.Errorf("failed to update customer: %w", err)
	}

	return cust, nil
}

func (i *StripeIntegration) GetSource(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetSourceParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.SourceID) == "" {
		return nil, fmt.Errorf("source ID is required")
	}

	// Get credential with cached approach
	credentialData, err := i.getCredential(ctx)
	if err != nil {
		return nil, err
	}

	// Set the API key for stripe-go
	stripe.Key = credentialData.SecretKey

	// Get the source using stripe-go
	src, err := source.Get(p.SourceID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get source: %w", err)
	}

	return src, nil
}

// Helper function to parse date string to Unix timestamp
func (i *StripeIntegration) parseDateToTimestamp(dateStr string) (int64, error) {
	if dateStr == "" {
		return 0, fmt.Errorf("empty date string")
	}

	// Sanitize input to prevent potential issues
	dateStr = strings.TrimSpace(dateStr)
	if dateStr == "" {
		return 0, fmt.Errorf("empty date string after trimming")
	}

	// Try parsing as timestamp first
	if timestamp, err := strconv.ParseInt(dateStr, 10, 64); err == nil {
		// Validate timestamp is reasonable (between 2000-01-01 and 2100-01-01)
		if timestamp >= 946684800 && timestamp <= 4102444800 {
			return timestamp, nil
		}
		return 0, fmt.Errorf("timestamp out of reasonable range: %d", timestamp)
	}

	// Try various date formats
	dateFormats := []string{
		"2006-01-02",           // ISO date: 2021-01-01
		"2006-01-02T15:04:05Z", // ISO datetime: 2021-01-01T00:00:00Z
		"2006-01-02 15:04:05",  // Common format: 2021-01-01 00:00:00
		"Jan 2, 2006",          // Human format: Jan 1, 2021
		"January 2, 2006",      // Full month: January 1, 2021
		"02/01/2006",           // US format: 01/01/2021
		"2006/01/02",           // ISO-like: 2021/01/01
	}

	for _, format := range dateFormats {
		if parsedTime, err := time.Parse(format, dateStr); err == nil {
			timestamp := parsedTime.Unix()
			// Validate parsed timestamp is reasonable
			if timestamp >= 946684800 && timestamp <= 4102444800 {
				return timestamp, nil
			}
			return 0, fmt.Errorf("parsed timestamp out of reasonable range: %d", timestamp)
		}
	}

	return 0, fmt.Errorf("unable to parse date: %s", dateStr)
}

// Helper function to handle created parameter (timestamp or JSON range)
func (i *StripeIntegration) handleCreatedParameter(query url.Values, created string) error {
	if created == "" {
		return nil
	}

	// Sanitize input
	created = strings.TrimSpace(created)
	if created == "" {
		return nil
	}

	// Handle created parameter: direct timestamp or JSON range
	if strings.HasPrefix(created, "{") {
		// Clean up escaped JSON string
		cleanedJSON := strings.ReplaceAll(created, `\"`, `"`)

		// Parse JSON for date range queries like {"gte":1609459200,"lte":1609545600}
		var createdRange map[string]interface{}
		if err := json.Unmarshal([]byte(cleanedJSON), &createdRange); err != nil {
			// If JSON parsing fails, treat as direct value
			query.Set("created", created)
			return nil
		}

		// Validate JSON structure
		for key, value := range createdRange {
			if key != "gte" && key != "lte" && key != "gt" && key != "lt" {
				return fmt.Errorf("invalid created parameter key: %s", key)
			}
			// Validate value is numeric
			switch v := value.(type) {
			case float64, int64, int:
				query.Set(fmt.Sprintf("created[%s]", key), fmt.Sprintf("%v", v))
			default:
				return fmt.Errorf("invalid created parameter value type for key %s: %T", key, v)
			}
		}
	} else {
		// Direct timestamp - validate it's numeric
		if _, err := strconv.ParseInt(created, 10, 64); err != nil {
			return fmt.Errorf("invalid timestamp format: %s", created)
		}
		query.Set("created", created)
	}

	return nil
}

// Helper function to handle user-friendly date parameters
func (i *StripeIntegration) handleDateParameters(query url.Values, afterDate, beforeDate string) error {
	if afterDate != "" {
		if timestamp, err := i.parseDateToTimestamp(afterDate); err != nil {
			return fmt.Errorf("invalid after_date: %w", err)
		} else {
			query.Set("created[gte]", strconv.FormatInt(timestamp, 10))
		}
	}

	if beforeDate != "" {
		if timestamp, err := i.parseDateToTimestamp(beforeDate); err != nil {
			return fmt.Errorf("invalid before_date: %w", err)
		} else {
			query.Set("created[lte]", strconv.FormatInt(timestamp, 10))
		}
	}

	return nil
}

// Payment Intents API implementations
func (i *StripeIntegration) CreatePaymentIntent(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := CreatePaymentIntentParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.Amount) == "" || strings.TrimSpace(p.Currency) == "" || strings.TrimSpace(p.Customer) == "" {
		return nil, fmt.Errorf("amount, currency, and customer are required")
	}

	// Get credential with cached approach
	credentialData, err := i.getCredential(ctx)
	if err != nil {
		return nil, err
	}

	// Set the API key for stripe-go
	stripe.Key = credentialData.SecretKey

	// Convert amount string to int64 (Stripe expects amount in cents)
	amount, err := strconv.ParseInt(p.Amount, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}

	// Create payment intent parameters
	piParams := &stripe.PaymentIntentParams{
		Amount:   stripe.Int64(amount),
		Currency: stripe.String(p.Currency),
		Customer: stripe.String(p.Customer),
	}

	if p.PaymentMethod != "" {
		piParams.PaymentMethod = stripe.String(p.PaymentMethod)
	}
	if p.Description != "" {
		piParams.Description = stripe.String(p.Description)
	}
	if p.AutoConfirm {
		piParams.Confirm = stripe.Bool(true)
	}
	if p.ReturnURL != "" {
		piParams.ReturnURL = stripe.String(p.ReturnURL)
	}

	// Handle redirect-based payment methods
	if p.DisableRedirects {
		piParams.AutomaticPaymentMethods = &stripe.PaymentIntentAutomaticPaymentMethodsParams{
			Enabled:        stripe.Bool(true),
			AllowRedirects: stripe.String("never"),
		}
	}

	// Create the payment intent using stripe-go
	pi, err := paymentintent.New(piParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create payment intent: %w", err)
	}

	return pi, nil
}

func (i *StripeIntegration) ConfirmPaymentIntent(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := ConfirmPaymentIntentParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.PaymentIntentID) == "" {
		return nil, fmt.Errorf("payment intent ID is required")
	}

	// Get credential with cached approach
	credentialData, err := i.getCredential(ctx)
	if err != nil {
		return nil, err
	}

	// Set the API key for stripe-go
	stripe.Key = credentialData.SecretKey

	// Create confirm parameters
	confirmParams := &stripe.PaymentIntentConfirmParams{}

	if p.PaymentMethod != "" {
		confirmParams.PaymentMethod = stripe.String(p.PaymentMethod)
	}
	if p.ReturnURL != "" {
		confirmParams.ReturnURL = stripe.String(p.ReturnURL)
	}

	// Confirm the payment intent using stripe-go
	pi, err := paymentintent.Confirm(p.PaymentIntentID, confirmParams)
	if err != nil {
		return nil, fmt.Errorf("failed to confirm payment intent: %w", err)
	}

	return pi, nil
}

func (i *StripeIntegration) GetPaymentIntent(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetPaymentIntentParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.PaymentIntentID) == "" {
		return nil, fmt.Errorf("payment intent ID is required")
	}

	// Get credential with cached approach
	credentialData, err := i.getCredential(ctx)
	if err != nil {
		return nil, err
	}

	// Set the API key for stripe-go
	stripe.Key = credentialData.SecretKey

	// Get the payment intent using stripe-go
	pi, err := paymentintent.Get(p.PaymentIntentID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get payment intent: %w", err)
	}

	return pi, nil
}

func (i *StripeIntegration) GetManyPaymentIntents(ctx context.Context, params domain.IntegrationInput, item domain.Item) ([]domain.Item, error) {
	p := GetManyPaymentIntentsParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	// Get credential with cached approach
	credentialData, err := i.getCredential(ctx)
	if err != nil {
		return nil, err
	}

	// Set the API key for stripe-go
	stripe.Key = credentialData.SecretKey

	// Create list parameters
	listParams := &stripe.PaymentIntentListParams{}

	// Set customer filter if provided
	if p.Customer != "" {
		listParams.Customer = stripe.String(p.Customer)
	}

	// Handle advanced created parameter (for backward compatibility)
	if p.Created != "" {
		// Parse created parameter: direct timestamp or JSON range
		if strings.HasPrefix(p.Created, "{") {
			// Clean up escaped JSON string
			cleanedJSON := strings.ReplaceAll(p.Created, `\"`, `"`)

			// Parse JSON for date range queries like {"gte":1609459200,"lte":1609545600}
			var createdRange map[string]interface{}
			if err := json.Unmarshal([]byte(cleanedJSON), &createdRange); err == nil {
				for key, value := range createdRange {
					if key == "gte" || key == "lte" || key == "gt" || key == "lt" {
						if timestampStr := fmt.Sprintf("%v", value); timestampStr != "" {
							listParams.Filters.AddFilter(fmt.Sprintf("created[%s]", key), "", timestampStr)
						}
					}
				}
			}
		} else {
			// Direct timestamp
			listParams.Filters.AddFilter("created", "", p.Created)
		}
	}

	// Handle user-friendly date parameters
	if p.AfterDate != "" {
		if timestamp, err := i.parseDateToTimestamp(p.AfterDate); err == nil {
			listParams.Filters.AddFilter("created[gte]", "", strconv.FormatInt(timestamp, 10))
		} else {
			return nil, fmt.Errorf("invalid after_date: %w", err)
		}
	}

	if p.BeforeDate != "" {
		if timestamp, err := i.parseDateToTimestamp(p.BeforeDate); err == nil {
			listParams.Filters.AddFilter("created[lte]", "", strconv.FormatInt(timestamp, 10))
		} else {
			return nil, fmt.Errorf("invalid before_date: %w", err)
		}
	}

	// Get payment intents using stripe-go
	iter := paymentintent.List(listParams)

	var items []domain.Item
	for iter.Next() {
		pi := iter.PaymentIntent()
		items = append(items, pi)
	}

	if err := iter.Err(); err != nil {
		return nil, fmt.Errorf("failed to get payment intents: %w", err)
	}

	return items, nil
}

func (i *StripeIntegration) UpdatePaymentIntent(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := UpdatePaymentIntentParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.PaymentIntentID) == "" {
		return nil, fmt.Errorf("payment intent ID is required")
	}

	// Get credential with cached approach
	credentialData, err := i.getCredential(ctx)
	if err != nil {
		return nil, err
	}

	// Set the API key for stripe-go
	stripe.Key = credentialData.SecretKey

	// Create update parameters
	updateParams := &stripe.PaymentIntentParams{}
	hasUpdates := false

	if p.Amount != "" {
		// Convert amount string to int64 (Stripe expects amount in cents)
		if amount, err := strconv.ParseInt(p.Amount, 10, 64); err == nil {
			updateParams.Amount = stripe.Int64(amount)
			hasUpdates = true
		} else {
			return nil, fmt.Errorf("invalid amount: %w", err)
		}
	}
	if p.Description != "" {
		updateParams.Description = stripe.String(p.Description)
		hasUpdates = true
	}
	if p.Metadata != "" {
		// Parse metadata JSON and add to params
		var metadata map[string]string
		if err := json.Unmarshal([]byte(p.Metadata), &metadata); err == nil {
			updateParams.Metadata = metadata
			hasUpdates = true
		} else {
			return nil, fmt.Errorf("invalid metadata JSON: %w", err)
		}
	}

	if !hasUpdates {
		return nil, fmt.Errorf("at least one field must be updated")
	}

	// Update the payment intent using stripe-go
	pi, err := paymentintent.Update(p.PaymentIntentID, updateParams)
	if err != nil {
		return nil, fmt.Errorf("failed to update payment intent: %w", err)
	}

	return pi, nil
}

func (i *StripeIntegration) CancelPaymentIntent(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := CancelPaymentIntentParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.PaymentIntentID) == "" {
		return nil, fmt.Errorf("payment intent ID is required")
	}

	// Get credential with cached approach
	credentialData, err := i.getCredential(ctx)
	if err != nil {
		return nil, err
	}

	// Set the API key for stripe-go
	stripe.Key = credentialData.SecretKey

	// Create cancel parameters
	cancelParams := &stripe.PaymentIntentCancelParams{}

	if p.CancellationReason != "" {
		cancelParams.CancellationReason = stripe.String(p.CancellationReason)
	}

	// Cancel the payment intent using stripe-go
	pi, err := paymentintent.Cancel(p.PaymentIntentID, cancelParams)
	if err != nil {
		return nil, fmt.Errorf("failed to cancel payment intent: %w", err)
	}

	return pi, nil
}

// Payment Methods API implementations

func (i *StripeIntegration) GetPaymentMethod(ctx context.Context, params domain.IntegrationInput, item domain.Item) (domain.Item, error) {
	p := GetPaymentMethodParams{}
	err := i.binder.BindToStruct(ctx, item, &p, params.IntegrationParams.Settings)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(p.PaymentMethodID) == "" {
		return nil, fmt.Errorf("payment method ID is required")
	}

	// Get credential with cached approach
	credentialData, err := i.getCredential(ctx)
	if err != nil {
		return nil, err
	}

	// Set the API key for stripe-go
	stripe.Key = credentialData.SecretKey

	// Get the payment method using stripe-go
	pm, err := paymentmethod.Get(p.PaymentMethodID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get payment method: %w", err)
	}

	return pm, nil
}
