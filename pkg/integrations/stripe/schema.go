package stripe

import (
	"github.com/flowbaker/flowbaker/pkg/domain"
)

// Stripe universal trigger event type (following Jira pattern)
const (
	IntegrationTriggerType_StripeUniversalTrigger domain.IntegrationTriggerEventType = "stripe_universal_trigger"
)

// Stripe integration action types
const (
	IntegrationActionType_GetBalance domain.IntegrationActionType = "get_balance"
	// Payment Intents API (Modern - Recommended)
	IntegrationActionType_CreatePaymentIntent   domain.IntegrationActionType = "create_payment_intent"
	IntegrationActionType_ConfirmPaymentIntent  domain.IntegrationActionType = "confirm_payment_intent"
	IntegrationActionType_GetPaymentIntent      domain.IntegrationActionType = "get_payment_intent"
	IntegrationActionType_GetManyPaymentIntents domain.IntegrationActionType = "get_many_payment_intents"
	IntegrationActionType_UpdatePaymentIntent   domain.IntegrationActionType = "update_payment_intent"
	IntegrationActionType_CancelPaymentIntent   domain.IntegrationActionType = "cancel_payment_intent"
	// Payment Methods API

	IntegrationActionType_GetPaymentMethod domain.IntegrationActionType = "get_payment_method"
	// Legacy Charges API (for backward compatibility)
	IntegrationActionType_CreateCharge   domain.IntegrationActionType = "create_charge"
	IntegrationActionType_GetCharge      domain.IntegrationActionType = "get_charge"
	IntegrationActionType_GetManyCharges domain.IntegrationActionType = "get_many_charges"
	IntegrationActionType_UpdateCharge   domain.IntegrationActionType = "update_charge"
	// Other APIs
	IntegrationActionType_CreateCoupon     domain.IntegrationActionType = "create_coupon"
	IntegrationActionType_GetManyCoupons   domain.IntegrationActionType = "get_many_coupons"
	IntegrationActionType_CreateCustomer   domain.IntegrationActionType = "create_customer"
	IntegrationActionType_DeleteCustomer   domain.IntegrationActionType = "delete_customer"
	IntegrationActionType_GetCustomer      domain.IntegrationActionType = "get_customer"
	IntegrationActionType_GetManyCustomers domain.IntegrationActionType = "get_many_customers"
	IntegrationActionType_UpdateCustomer   domain.IntegrationActionType = "update_customer"
	IntegrationActionType_GetCustomerCard  domain.IntegrationActionType = "get_customer_card"
	IntegrationActionType_CreateSource     domain.IntegrationActionType = "create_source"
	IntegrationActionType_GetSource        domain.IntegrationActionType = "get_source"
)

var (
	Schema = schema

	schema domain.Integration = domain.Integration{
		ID:                "stripe",
		Name:              "Stripe",
		Description:       "Use Stripe integration to manage payments, customers, charges, and more.",
		CanTestConnection: true,
		CredentialProperties: []domain.NodeProperty{
			{
				Key:         "secret_key",
				Name:        "Secret Key",
				Description: "The Stripe secret key for API authentication",
				Required:    true,
				Type:        domain.NodePropertyType_String,
			},
			{
				Key:         "webhook_secret_snapshot",
				Name:        "Webhook Secret (Snapshot)",
				Description: "The webhook secret for snapshot payload webhooks",
				Required:    false,
				Type:        domain.NodePropertyType_String,
			},
			{
				Key:         "webhook_secret_thin",
				Name:        "Webhook Secret (Thin)",
				Description: "The webhook secret for thin payload webhooks",
				Required:    false,
				Type:        domain.NodePropertyType_String,
			},
		},
		Actions: []domain.IntegrationAction{
			// Balance actions
			{
				ID:          "get_balance",
				Name:        "Get Balance",
				ActionType:  IntegrationActionType_GetBalance,
				Description: "Get the current balance of your Stripe account",
				Properties:  []domain.NodeProperty{},
			},
			// Payment Intents API (Modern - Recommended)
			{
				ID:          "create_payment_intent",
				Name:        "Create Payment Intent",
				ActionType:  IntegrationActionType_CreatePaymentIntent,
				Description: "Create a new payment intent (modern payment API)",
				Properties: []domain.NodeProperty{
					{
						Key:         "amount",
						Name:        "Amount",
						Description: "Amount to charge in cents",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "currency",
						Name:        "Currency",
						Description: "Currency code (e.g., usd, eur)",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "payment_method",
						Name:        "Payment Method",
						Description: "Payment method ID (pm_xxxxx or card_xxxxx) - optional for manual confirmation",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "customer",
						Name:        "Customer",
						Description: "Customer ID to associate with the payment",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "description",
						Name:        "Description",
						Description: "Description of the payment",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "auto_confirm",
						Name:        "Auto Confirm",
						Description: "Automatically confirm payment",
						Required:    false,
						Type:        domain.NodePropertyType_Boolean,
					},
					{
						Key:         "return_url",
						Name:        "Return URL",
						Description: "URL to redirect customer after payment (required if redirect-based payment methods enabled in Dashboard)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						DependsOn: &domain.DependsOn{
							PropertyKey: "auto_confirm",
							Value:       true,
						},
					},
					{
						Key:         "disable_redirects",
						Name:        "Disable Redirect Methods",
						Description: "Disable redirect-based payment methods (iDEAL, Giropay, etc.) to avoid return_url requirement",
						Required:    false,
						Type:        domain.NodePropertyType_Boolean,
						DependsOn: &domain.DependsOn{
							PropertyKey: "auto_confirm",
							Value:       true,
						},
					},
				},
			},
			{
				ID:          "confirm_payment_intent",
				Name:        "Confirm Payment Intent",
				ActionType:  IntegrationActionType_ConfirmPaymentIntent,
				Description: "Confirm a payment intent",
				Properties: []domain.NodeProperty{
					{
						Key:         "payment_intent_id",
						Name:        "Payment Intent ID",
						Description: "The ID of the payment intent to confirm",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "payment_method",
						Name:        "Payment Method",
						Description: "Payment method ID (pm_xxxxx or card_xxxxx) if not set during creation",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "return_url",
						Name:        "Return URL",
						Description: "URL to redirect after payment. ⚠️ REQUIRED if Payment Intent was created with automatic_payment_methods[allow_redirects] = 'always' or redirect-based payment methods are enabled.",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "get_payment_intent",
				Name:        "Get Payment Intent",
				ActionType:  IntegrationActionType_GetPaymentIntent,
				Description: "Get a specific payment intent by ID",
				Properties: []domain.NodeProperty{
					{
						Key:         "payment_intent_id",
						Name:        "Payment Intent ID",
						Description: "The ID of the payment intent to retrieve",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "get_many_payment_intents",
				Name:        "Get Many Payment Intents",
				ActionType:  IntegrationActionType_GetManyPaymentIntents,
				Description: "Get multiple payment intents",
				Properties: []domain.NodeProperty{
					{
						Key:         "customer",
						Name:        "Customer",
						Description: "Filter by customer ID",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "after_date",
						Name:        "After Date",
						Description: "Show payment intents created after this date. Use timestamp (1609459200) or ISO date (2021-01-01) or human format (Jan 1, 2021)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "before_date",
						Name:        "Before Date",
						Description: "Show payment intents created before this date. Use timestamp (1609545600) or ISO date (2021-01-02) or human format (Jan 2, 2021)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "update_payment_intent",
				Name:        "Update Payment Intent",
				ActionType:  IntegrationActionType_UpdatePaymentIntent,
				Description: "Update an existing payment intent",
				Properties: []domain.NodeProperty{
					{
						Key:         "payment_intent_id",
						Name:        "Payment Intent ID",
						Description: "The ID of the payment intent to update",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "amount",
						Name:        "Amount",
						Description: "Updated amount in cents",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "description",
						Name:        "Description",
						Description: "Updated description",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "metadata",
						Name:        "Metadata",
						Description: "Updated metadata (JSON string)",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
				},
			},
			{
				ID:          "cancel_payment_intent",
				Name:        "Cancel Payment Intent",
				ActionType:  IntegrationActionType_CancelPaymentIntent,
				Description: "Cancel a payment intent",
				Properties: []domain.NodeProperty{
					{
						Key:         "payment_intent_id",
						Name:        "Payment Intent ID",
						Description: "The ID of the payment intent to cancel",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "cancellation_reason",
						Name:        "Cancellation Reason",
						Description: "Reason for cancellation (duplicate, fraudulent, requested_by_customer, abandoned)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			// Payment Methods API
			{
				ID:          "get_payment_method",
				Name:        "Get Payment Method",
				ActionType:  IntegrationActionType_GetPaymentMethod,
				Description: "Get a specific payment method by ID",
				Properties: []domain.NodeProperty{
					{
						Key:         "payment_method_id",
						Name:        "Payment Method ID",
						Description: "The ID of the payment method to retrieve",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			// Legacy Charge actions (for backward compatibility)
			{
				ID:          "create_charge",
				Name:        "Create Charge (Legacy)",
				ActionType:  IntegrationActionType_CreateCharge,
				Description: "Create a new charge using legacy API - consider using Payment Intents instead",
				Properties: []domain.NodeProperty{
					{
						Key:         "amount",
						Name:        "Amount",
						Description: "Amount to charge in cents",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "currency",
						Name:        "Currency",
						Description: "Currency code (e.g., usd, eur)",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "source",
						Name:        "Source",
						Description: "Payment source (token like tok_visa or legacy card ID starting with card_)",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "description",
						Name:        "Description",
						Description: "Description of the charge",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "customer",
						Name:        "Customer",
						Description: "Customer ID to associate with the charge",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "get_charge",
				Name:        "Get Charge",
				ActionType:  IntegrationActionType_GetCharge,
				Description: "Get a specific charge by ID",
				Properties: []domain.NodeProperty{
					{
						Key:         "charge_id",
						Name:        "Charge ID",
						Description: "The ID of the charge to retrieve",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "get_many_charges",
				Name:        "Get Many Charges",
				ActionType:  IntegrationActionType_GetManyCharges,
				Description: "Get multiple charges",
				Properties: []domain.NodeProperty{
					{
						Key:         "customer",
						Name:        "Customer",
						Description: "Filter by customer ID",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "after_date",
						Name:        "After Date",
						Description: "Show charges created after this date. Use timestamp (1609459200) or ISO date (2021-01-01) or human format (Jan 1, 2021)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "before_date",
						Name:        "Before Date",
						Description: "Show charges created before this date. Use timestamp (1609545600) or ISO date (2021-01-02) or human format (Jan 2, 2021)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "update_charge",
				Name:        "Update Charge",
				ActionType:  IntegrationActionType_UpdateCharge,
				Description: "Update an existing charge",
				Properties: []domain.NodeProperty{
					{
						Key:         "charge_id",
						Name:        "Charge ID",
						Description: "The ID of the charge to update",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "customer",
						Name:        "Customer",
						Description: "Associate charge with existing customer (only if no customer already associated)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "description",
						Name:        "Description",
						Description: "Updated description of the charge",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "receipt_email",
						Name:        "Receipt Email",
						Description: "Email address for charge receipt (triggers new receipt if updated)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "shipping_name",
						Name:        "Shipping Name",
						Description: "Recipient name for shipping",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "shipping_phone",
						Name:        "Shipping Phone",
						Description: "Recipient phone for shipping",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "shipping_address_line1",
						Name:        "Shipping Address Line 1",
						Description: "First line of shipping address",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "shipping_address_line2",
						Name:        "Shipping Address Line 2",
						Description: "Second line of shipping address",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "shipping_address_city",
						Name:        "Shipping City",
						Description: "City for shipping address",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "shipping_address_state",
						Name:        "Shipping State",
						Description: "State/Province for shipping address",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "shipping_address_postal_code",
						Name:        "Shipping Postal Code",
						Description: "Postal/ZIP code for shipping address",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "shipping_address_country",
						Name:        "Shipping Country",
						Description: "Country code for shipping address (e.g., US, GB)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "metadata",
						Name:        "Metadata",
						Description: "Updated metadata (JSON string)",
						Required:    false,
						Type:        domain.NodePropertyType_Text,
					},
				},
			},
			// Coupon actions
			{
				ID:          "create_coupon",
				Name:        "Create Coupon",
				ActionType:  IntegrationActionType_CreateCoupon,
				Description: "Create a new coupon",
				Properties: []domain.NodeProperty{
					{
						Key:         "id",
						Name:        "Coupon ID",
						Description: "Unique identifier for the coupon",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "percent_off",
						Name:        "Percent Off",
						Description: "Percentage discount (1-100)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "amount_off",
						Name:        "Amount Off",
						Description: "Fixed amount discount in cents",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "currency",
						Name:        "Currency",
						Description: "Currency for amount_off (required if amount_off is set)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "duration",
						Name:        "Duration",
						Description: "How long the coupon discount lasts",
						Required:    true,
						Type:        domain.NodePropertyType_String,
						Options: []domain.NodePropertyOption{
							{Label: "Once", Value: "once", Description: "One-time discount that applies to the next invoice"},
							{Label: "Repeating", Value: "repeating", Description: "Discount that applies for a specific number of months"},
							{Label: "Forever", Value: "forever", Description: "Discount that applies forever"},
						},
					},
					{
						Key:         "duration_in_months",
						Name:        "Duration in Months",
						Description: "Number of months for repeating duration (required when duration is 'repeating')",
						Required:    false,
						Type:        domain.NodePropertyType_String,
						DependsOn: &domain.DependsOn{
							PropertyKey: "duration",
							Value:       "repeating",
						},
					},
				},
			},
			{
				ID:          "get_many_coupons",
				Name:        "Get Many Coupons",
				ActionType:  IntegrationActionType_GetManyCoupons,
				Description: "Get multiple coupons",
				Properties: []domain.NodeProperty{
					{
						Key:         "after_date",
						Name:        "After Date",
						Description: "Show coupons created after this date. Use timestamp (1609459200) or ISO date (2021-01-01) or human format (Jan 1, 2021)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "before_date",
						Name:        "Before Date",
						Description: "Show coupons created before this date. Use timestamp (1609545600) or ISO date (2021-01-02) or human format (Jan 2, 2021)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			// Customer actions
			{
				ID:          "create_customer",
				Name:        "Create Customer",
				ActionType:  IntegrationActionType_CreateCustomer,
				Description: "Create a new customer",
				Properties: []domain.NodeProperty{
					{
						Key:         "email",
						Name:        "Email",
						Description: "Customer's email address",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "name",
						Name:        "Name",
						Description: "Customer's name",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "phone",
						Name:        "Phone",
						Description: "Customer's phone number",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "description",
						Name:        "Description",
						Description: "Description of the customer",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "delete_customer",
				Name:        "Delete Customer",
				ActionType:  IntegrationActionType_DeleteCustomer,
				Description: "Delete a customer",
				Properties: []domain.NodeProperty{
					{
						Key:         "customer_id",
						Name:        "Customer ID",
						Description: "The ID of the customer to delete",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "get_customer",
				Name:        "Get Customer",
				ActionType:  IntegrationActionType_GetCustomer,
				Description: "Get a specific customer by ID",
				Properties: []domain.NodeProperty{
					{
						Key:         "customer_id",
						Name:        "Customer ID",
						Description: "The ID of the customer to retrieve",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "get_many_customers",
				Name:        "Get Many Customers",
				ActionType:  IntegrationActionType_GetManyCustomers,
				Description: "Get multiple customers",
				Properties: []domain.NodeProperty{
					{
						Key:         "limit",
						Name:        "Limit",
						Description: "Maximum number of customers to return (default: 10, max: 100)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "email",
						Name:        "Email",
						Description: "Filter by email address",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "after_date",
						Name:        "After Date",
						Description: "Show customers created after this date. Use timestamp (1609459200) or ISO date (2021-01-01) or human format (Jan 1, 2021)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "before_date",
						Name:        "Before Date",
						Description: "Show customers created before this date. Use timestamp (1609545600) or ISO date (2021-01-02) or human format (Jan 2, 2021)",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
			{
				ID:          "update_customer",
				Name:        "Update Customer",
				ActionType:  IntegrationActionType_UpdateCustomer,
				Description: "Update an existing customer",
				Properties: []domain.NodeProperty{
					{
						Key:         "customer_id",
						Name:        "Customer ID",
						Description: "The ID of the customer to update",
						Required:    true,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "email",
						Name:        "Email",
						Description: "Updated email address",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "name",
						Name:        "Name",
						Description: "Updated name",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "phone",
						Name:        "Phone",
						Description: "Updated phone number",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "description",
						Name:        "Description",
						Description: "Updated description",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
					{
						Key:         "default_source",
						Name:        "Default Source",
						Description: "Card ID to set as default payment source",
						Required:    false,
						Type:        domain.NodePropertyType_String,
					},
				},
			},
		},
		Triggers: []domain.IntegrationTrigger{
			{
				ID:          "stripe_event_listener",
				Name:        "Stripe Event Listener",
				EventType:   IntegrationTriggerType_StripeUniversalTrigger,
				Description: "Triggers on selected Stripe events.",
				Properties: []domain.NodeProperty{
					{
						Key:         "path",
						Name:        "Webhook URL",
						Description: "The webhook URL endpoint for Stripe events",
						Required:    true,
						Type:        domain.NodePropertyType_Endpoint,
						EndpointPropertyOpts: &domain.EndpointPropertyOptions{
							AllowedMethods: []string{"POST"},
						},
					},
					{
						Key:         "selected_events",
						Name:        "Stripe Events",
						Description: "Select one or more Stripe events to trigger this flow",
						Required:    true,
						Type:        domain.NodePropertyType_ListTagInput,
						Options: []domain.NodePropertyOption{
							// Account events
							{Label: "On Account Updated", Value: "account.updated", Description: "Triggered when account information is updated"},
							{Label: "On Account Application Authorized", Value: "account.application.authorized", Description: "Triggered when an application is authorized"},
							{Label: "On Account Application Deauthorized", Value: "account.application.deauthorized", Description: "Triggered when an application is deauthorized"},
							{Label: "On Account External Account Created", Value: "account.external_account.created", Description: "Triggered when an external account is created"},
							{Label: "On Account External Account Deleted", Value: "account.external_account.deleted", Description: "Triggered when an external account is deleted"},
							{Label: "On Account External Account Updated", Value: "account.external_account.updated", Description: "Triggered when an external account is updated"},

							// Application fee events
							{Label: "On Application Fee Created", Value: "application_fee.created", Description: "Triggered when an application fee is created"},
							{Label: "On Application Fee Refunded", Value: "application_fee.refunded", Description: "Triggered when an application fee is refunded"},
							{Label: "On Application Fee Refund Updated", Value: "application_fee.refund.updated", Description: "Triggered when an application fee refund is updated"},

							// Balance events
							{Label: "On Balance Available", Value: "balance.available", Description: "Triggered when balance becomes available"},

							// Capability events
							{Label: "On Capability Updated", Value: "capability.updated", Description: "Triggered when a capability is updated"},

							// Charge events
							{Label: "On Charge Captured", Value: "charge.captured", Description: "Triggered when a charge is captured"},
							{Label: "On Charge Expired", Value: "charge.expired", Description: "Triggered when a charge expires"},
							{Label: "On Charge Failed", Value: "charge.failed", Description: "Triggered when a charge fails"},
							{Label: "On Charge Pending", Value: "charge.pending", Description: "Triggered when a charge is pending"},
							{Label: "On Charge Refunded", Value: "charge.refunded", Description: "Triggered when a charge is refunded"},
							{Label: "On Charge Succeeded", Value: "charge.succeeded", Description: "Triggered when a charge succeeds"},
							{Label: "On Charge Updated", Value: "charge.updated", Description: "Triggered when a charge is updated"},

							// Charge dispute events
							{Label: "On Charge Dispute Closed", Value: "charge.dispute.closed", Description: "Triggered when a charge dispute is closed"},
							{Label: "On Charge Dispute Created", Value: "charge.dispute.created", Description: "Triggered when a charge dispute is created"},
							{Label: "On Charge Dispute Funds Reinstated", Value: "charge.dispute.funds_reinstated", Description: "Triggered when charge dispute funds are reinstated"},
							{Label: "On Charge Dispute Funds Withdrawn", Value: "charge.dispute.funds_withdrawn", Description: "Triggered when charge dispute funds are withdrawn"},
							{Label: "On Charge Dispute Updated", Value: "charge.dispute.updated", Description: "Triggered when a charge dispute is updated"},

							// Refund events
							{Label: "On Charge Refund Updated", Value: "charge.refund.updated", Description: "Triggered when a charge refund is updated"},

							// Checkout session events
							{Label: "On Checkout Session Completed", Value: "checkout.session.completed", Description: "Triggered when a checkout session is completed"},

							// Coupon events
							{Label: "On Coupon Created", Value: "coupon.created", Description: "Triggered when a coupon is created"},
							{Label: "On Coupon Deleted", Value: "coupon.deleted", Description: "Triggered when a coupon is deleted"},
							{Label: "On Coupon Updated", Value: "coupon.updated", Description: "Triggered when a coupon is updated"},

							// Credit note events
							{Label: "On Credit Note Created", Value: "credit_note.created", Description: "Triggered when a credit note is created"},
							{Label: "On Credit Note Updated", Value: "credit_note.updated", Description: "Triggered when a credit note is updated"},
							{Label: "On Credit Note Voided", Value: "credit_note.voided", Description: "Triggered when a credit note is voided"},

							// Customer events
							{Label: "On Customer Created", Value: "customer.created", Description: "Triggered when a customer is created"},
							{Label: "On Customer Deleted", Value: "customer.deleted", Description: "Triggered when a customer is deleted"},
							{Label: "On Customer Updated", Value: "customer.updated", Description: "Triggered when a customer is updated"},

							// Customer discount events
							{Label: "On Customer Discount Created", Value: "customer.discount.created", Description: "Triggered when a customer discount is created"},
							{Label: "On Customer Discount Deleted", Value: "customer.discount.deleted", Description: "Triggered when a customer discount is deleted"},
							{Label: "On Customer Discount Updated", Value: "customer.discount.updated", Description: "Triggered when a customer discount is updated"},

							// Customer source events
							{Label: "On Customer Source Created", Value: "customer.source.created", Description: "Triggered when a customer source is created"},
							{Label: "On Customer Source Deleted", Value: "customer.source.deleted", Description: "Triggered when a customer source is deleted"},
							{Label: "On Customer Source Expiring", Value: "customer.source.expiring", Description: "Triggered when a customer source is expiring"},
							{Label: "On Customer Source Updated", Value: "customer.source.updated", Description: "Triggered when a customer source is updated"},

							// Customer subscription events
							{Label: "On Customer Subscription Created", Value: "customer.subscription.created", Description: "Triggered when a customer subscription is created"},
							{Label: "On Customer Subscription Deleted", Value: "customer.subscription.deleted", Description: "Triggered when a customer subscription is deleted"},
							{Label: "On Customer Subscription Trial Will End", Value: "customer.subscription.trial_will_end", Description: "Triggered when a customer subscription trial will end"},
							{Label: "On Customer Subscription Updated", Value: "customer.subscription.updated", Description: "Triggered when a customer subscription is updated"},

							// Customer tax ID events
							{Label: "On Customer Tax ID Created", Value: "customer.tax_id.created", Description: "Triggered when a customer tax ID is created"},
							{Label: "On Customer Tax ID Deleted", Value: "customer.tax_id.deleted", Description: "Triggered when a customer tax ID is deleted"},
							{Label: "On Customer Tax ID Updated", Value: "customer.tax_id.updated", Description: "Triggered when a customer tax ID is updated"},

							// File events
							{Label: "On File Created", Value: "file.created", Description: "Triggered when a file is created"},

							// Invoice events
							{Label: "On Invoice Created", Value: "invoice.created", Description: "Triggered when an invoice is created"},
							{Label: "On Invoice Deleted", Value: "invoice.deleted", Description: "Triggered when an invoice is deleted"},
							{Label: "On Invoice Finalized", Value: "invoice.finalized", Description: "Triggered when an invoice is finalized"},
							{Label: "On Invoice Marked Uncollectible", Value: "invoice.marked_uncollectible", Description: "Triggered when an invoice is marked uncollectible"},
							{Label: "On Invoice Payment Action Required", Value: "invoice.payment_action_required", Description: "Triggered when an invoice payment action is required"},
							{Label: "On Invoice Payment Failed", Value: "invoice.payment_failed", Description: "Triggered when an invoice payment fails"},
							{Label: "On Invoice Payment Succeeded", Value: "invoice.payment_succeeded", Description: "Triggered when an invoice payment succeeds"},
							{Label: "On Invoice Sent", Value: "invoice.sent", Description: "Triggered when an invoice is sent"},
							{Label: "On Invoice Upcoming", Value: "invoice.upcoming", Description: "Triggered when an invoice is upcoming"},
							{Label: "On Invoice Updated", Value: "invoice.updated", Description: "Triggered when an invoice is updated"},
							{Label: "On Invoice Voided", Value: "invoice.voided", Description: "Triggered when an invoice is voided"},

							// Invoice item events
							{Label: "On Invoice Item Created", Value: "invoiceitem.created", Description: "Triggered when an invoice item is created"},
							{Label: "On Invoice Item Deleted", Value: "invoiceitem.deleted", Description: "Triggered when an invoice item is deleted"},
							{Label: "On Invoice Item Updated", Value: "invoiceitem.updated", Description: "Triggered when an invoice item is updated"},

							// Payment intent events
							{Label: "On Payment Intent Amount Capturable Updated", Value: "payment_intent.amount_capturable_updated", Description: "Triggered when a payment intent amount capturable is updated"},
							{Label: "On Payment Intent Canceled", Value: "payment_intent.canceled", Description: "Triggered when a payment intent is canceled"},
							{Label: "On Payment Intent Created", Value: "payment_intent.created", Description: "Triggered when a payment intent is created"},
							{Label: "On Payment Intent Payment Failed", Value: "payment_intent.payment_failed", Description: "Triggered when a payment intent payment fails"},
							{Label: "On Payment Intent Succeeded", Value: "payment_intent.succeeded", Description: "Triggered when a payment intent succeeds"},
							{Label: "On Payment Intent Requires Action", Value: "payment_intent.requires_action", Description: "Triggered when a payment intent requires action"},

							// Payment method events
							{Label: "On Payment Method Attached", Value: "payment_method.attached", Description: "Triggered when a payment method is attached"},
							{Label: "On Payment Method Card Automatically Updated", Value: "payment_method.card_automatically_updated", Description: "Triggered when a payment method card is automatically updated"},
							{Label: "On Payment Method Detached", Value: "payment_method.detached", Description: "Triggered when a payment method is detached"},
							{Label: "On Payment Method Updated", Value: "payment_method.updated", Description: "Triggered when a payment method is updated"},

							// Payout events
							{Label: "On Payout Canceled", Value: "payout.canceled", Description: "Triggered when a payout is canceled"},
							{Label: "On Payout Created", Value: "payout.created", Description: "Triggered when a payout is created"},
							{Label: "On Payout Failed", Value: "payout.failed", Description: "Triggered when a payout fails"},
							{Label: "On Payout Paid", Value: "payout.paid", Description: "Triggered when a payout is paid"},
							{Label: "On Payout Updated", Value: "payout.updated", Description: "Triggered when a payout is updated"},

							// Plan events
							{Label: "On Plan Created", Value: "plan.created", Description: "Triggered when a plan is created"},
							{Label: "On Plan Deleted", Value: "plan.deleted", Description: "Triggered when a plan is deleted"},
							{Label: "On Plan Updated", Value: "plan.updated", Description: "Triggered when a plan is updated"},

							// Product events
							{Label: "On Product Created", Value: "product.created", Description: "Triggered when a product is created"},
							{Label: "On Product Deleted", Value: "product.deleted", Description: "Triggered when a product is deleted"},
							{Label: "On Product Updated", Value: "product.updated", Description: "Triggered when a product is updated"},

							// Setup intent events
							{Label: "On Setup Intent Canceled", Value: "setup_intent.canceled", Description: "Triggered when a setup intent is canceled"},
							{Label: "On Setup Intent Created", Value: "setup_intent.created", Description: "Triggered when a setup intent is created"},
							{Label: "On Setup Intent Setup Failed", Value: "setup_intent.setup_failed", Description: "Triggered when a setup intent setup fails"},
							{Label: "On Setup Intent Succeeded", Value: "setup_intent.succeeded", Description: "Triggered when a setup intent succeeds"},

							// Source events
							{Label: "On Source Canceled", Value: "source.canceled", Description: "Triggered when a source is canceled"},
							{Label: "On Source Chargeable", Value: "source.chargeable", Description: "Triggered when a source becomes chargeable"},
							{Label: "On Source Failed", Value: "source.failed", Description: "Triggered when a source fails"},
							{Label: "On Source Transaction Created", Value: "source.transaction.created", Description: "Triggered when a source transaction is created"},

							// Subscription events
							{Label: "On Subscription Created", Value: "subscription.created", Description: "Triggered when a subscription is created"},
							{Label: "On Subscription Deleted", Value: "subscription.deleted", Description: "Triggered when a subscription is deleted"},
							{Label: "On Subscription Updated", Value: "subscription.updated", Description: "Triggered when a subscription is updated"},

							// Tax rate events
							{Label: "On Tax Rate Created", Value: "tax_rate.created", Description: "Triggered when a tax rate is created"},
							{Label: "On Tax Rate Updated", Value: "tax_rate.updated", Description: "Triggered when a tax rate is updated"},

							// Topup events
							{Label: "On Topup Canceled", Value: "topup.canceled", Description: "Triggered when a topup is canceled"},
							{Label: "On Topup Created", Value: "topup.created", Description: "Triggered when a topup is created"},
							{Label: "On Topup Failed", Value: "topup.failed", Description: "Triggered when a topup fails"},
							{Label: "On Topup Reversed", Value: "topup.reversed", Description: "Triggered when a topup is reversed"},
							{Label: "On Topup Succeeded", Value: "topup.succeeded", Description: "Triggered when a topup succeeds"},

							// Transfer events
							{Label: "On Transfer Created", Value: "transfer.created", Description: "Triggered when a transfer is created"},
							{Label: "On Transfer Failed", Value: "transfer.failed", Description: "Triggered when a transfer fails"},
							{Label: "On Transfer Paid", Value: "transfer.paid", Description: "Triggered when a transfer is paid"},
							{Label: "On Transfer Reversed", Value: "transfer.reversed", Description: "Triggered when a transfer is reversed"},
							{Label: "On Transfer Updated", Value: "transfer.updated", Description: "Triggered when a transfer is updated"},
						},
					},
				},
			},
		},
	}
)
