package domain

import "fmt"

type NodePropertyType string

const (
	NodePropertyType_String        NodePropertyType = "string"
	NodePropertyType_Text          NodePropertyType = "text"
	NodePropertyType_TagInput      NodePropertyType = "tag_input"
	NodePropertyType_Integer       NodePropertyType = "integer"
	NodePropertyType_Number        NodePropertyType = "number"
	NodePropertyType_Float         NodePropertyType = "float"
	NodePropertyType_Boolean       NodePropertyType = "boolean"
	NodePropertyType_Array         NodePropertyType = "array"
	NodePropertyType_Map           NodePropertyType = "map"
	NodePropertyType_Date          NodePropertyType = "date"
	NodePropertyType_File          NodePropertyType = "file"
	NodePropertyType_CodeEditor    NodePropertyType = "code_editor"
	NodePropertyType_Query         NodePropertyType = "query"
	NodePropertyType_OAuth         NodePropertyType = "oauth"
	NodePropertyType_Endpoint      NodePropertyType = "endpoint"
	NodePropertyType_KeyValueTable NodePropertyType = "key_value_table"
	NodePropertyType_ListTagInput  NodePropertyType = "list_tag_input"
)

const (
	PropertySyntaxExtensionType_SQL  = "sql"
	PropertySyntaxExtensionType_JSON = "json"
)

type CodeLanguageType string

const (
	CodeLanguageType_JSON CodeLanguageType = "json"
	CodeLanguageType_SQL  CodeLanguageType = "sql"
)

const (
	PropertySyntaxDialectType_PostgreSQL = "postgresql"
	PropertySyntaxDialectType_MongoDB    = "mongodb"
)

type DragAndDropBehavior string

const (
	DragAndDropBehavior_Expression DragAndDropBehavior = "expression"
	DragAndDropBehavior_BasicPath  DragAndDropBehavior = "basic_path"
)

type OAuthType string

var (
	OAuthTypeGoogle         OAuthType = "google"
	OAuthTypeSlack          OAuthType = "slack"
	OAuthTypeDropbox        OAuthType = "dropbox"
	OAuthTypeGitHub         OAuthType = "github"
	OAuthTypeLinear         OAuthType = "linear"
	OAuthTypeJira           OAuthType = "jira"
	OAuthTypeMicrosoftTeams OAuthType = "microsoft_teams"
)

type NodeProperty struct {
	Key               string           `json:"key"`
	Name              string           `json:"name"`
	Description       string           `json:"description"`
	Required          bool             `json:"required"`
	Hidden            bool             `json:"hidden"`
	Advanced          bool             `json:"advanced"` // For advanced options that should be hidden by default
	Disabled          bool             `json:"disabled"` // For properties that should be visible but not editable
	Type              NodePropertyType `json:"type"`
	SubNodeProperties []NodeProperty   `json:"sub_node_properties,omitempty"`
	OAuthType         OAuthType        `json:"oauth_type"`
	IsSecret          bool             `json:"is_secret,omitempty"` // Whether this field is a secret

	// Validation
	Pattern   string `json:"pattern,omitempty"`    // Regex pattern for string validation
	MinLength int    `json:"min_length,omitempty"` // For string/text
	MaxLength int    `json:"max_length,omitempty"` // For string/text

	// Drag and drop
	DragAndDropBehavior DragAndDropBehavior `json:"drag_and_drop_behavior,omitempty"`

	// Dynamic behavior
	Dependent []string   `json:"dependent,omitempty"`  // List of properties this field depends on
	DependsOn *DependsOn `json:"depends_on,omitempty"` // Condition for when this field should be shown
	HideIf    *HideIf    `json:"hide_if,omitempty"`    // Conditions for when this field should be hidden
	ShowIf    *ShowIf    `json:"show_if,omitempty"`    // Conditions for when this field should be shown

	// UI Display
	Group       string `json:"group,omitempty"`       // For grouping related properties
	Placeholder string `json:"placeholder,omitempty"` // Placeholder text
	Help        string `json:"help,omitempty"`        // Extended help text
	RegexKey    string `json:"regex_key,omitempty"`   // Key for regex validation

	// Options based on type
	Options                 []NodePropertyOption         `json:"options,omitempty"`                   // For selectable options
	MultipleOpts            []MultipleNodePropertyOption `json:"multiple_opts,omitempty"`             // For multiple selectable options
	NumberOpts              *NumberPropertyOptions       `json:"number_opts,omitempty"`               // For number types
	ArrayOpts               *ArrayPropertyOptions        `json:"array_opts,omitempty"`                // For array type
	MapOpts                 *MapPropertyOptions          `json:"map_opts,omitempty"`                  // For map type
	CredentialSelectionOpts *CredentialSelectionOptions  `json:"credential_selection_opts,omitempty"` // For credential selection type
	EndpointPropertyOpts    *EndpointPropertyOptions     `json:"endpoint_property_opts,omitempty"`    // For endpoint type

	// Syntax highlighting
	SyntaxHighlightingOpts SyntaxHighlightingOpts `json:"syntax_highlighting_opts"`

	// Code editor specific settings
	CodeLanguage CodeLanguageType `json:"code_language,omitempty"` // Language for code editor type (e.g., "json", "sql")

	// Dynamic data loading
	Peekable                    bool                              `json:"peekable"`                                // Whether this field can load options dynamically
	PeekableType                IntegrationPeekableType           `json:"peekable_type,omitempty"`                 // Type of peekable data
	PeekablePaginationType      IntegrationPeekablePaginationType `json:"peekable_pagination_type,omitempty"`      // Type of pagination for peekable data
	PeekableDependentProperties []PeekableDependentProperty       `json:"peekable_dependent_properties,omitempty"` // Properties that this field depends on
	IsNonCredentialPeekable     bool                              `json:"is_non_credential_peekable,omitempty"`    // Whether this field can be peeked without credentials

	ExpressionChoice bool `json:"expression_choice"` // Whether this field can be set using expressions

	ValidDraggableTypes []string `json:"valid_draggable_types,omitempty"` // Types that can be dragged and dropped into this field

	IsApplicableToHTTP bool `json:"is_applicable_to_http"` // Whether this field is applicable to HTTP requests
	IsCustomOAuthable  bool `json:"is_custom_oauthable"`   // Whether this field is custom oauthable

	// Dynamic handle generation
	GeneratesHandles     bool                     `json:"generates_handles,omitempty"`      // Whether this property generates handles
	HandleGenerationOpts *HandleGenerationOptions `json:"handle_generation_opts,omitempty"` // Options for handle generation
}

type SyntaxHighlightingOpts struct {
	Extension        string `json:"extension"`
	Dialect          string `json:"dialect"`
	EnableParameters bool   `json:"enable_parameters"`
}

type PeekableDependentProperty struct {
	PropertyKey string `json:"property_key"`
	ValueKey    string `json:"value_key"`
}

type NodePropertyOption struct {
	Label       string `json:"label"`
	Value       any    `json:"value"`
	Description string `json:"description"`
}

type DependsOn struct {
	PropertyKey string `json:"property_key"`
	Value       any    `json:"value"`
}

type HideIf struct {
	PropertyKey string `json:"property_key"`
	Values      []any  `json:"values"`
}

type ShowIf struct {
	PropertyKey string `json:"property_key"`
	Values      []any  `json:"values"`
}

type NumberPropertyOptions struct {
	Min     float64 `json:"min,omitempty"`
	Max     float64 `json:"max,omitempty"`
	Default float64 `json:"default,omitempty"`
	Step    float64 `json:"step,omitempty"`
}

type ArrayPropertyOptions struct {
	MinItems       int              `json:"min_items,omitempty"`
	MaxItems       int              `json:"max_items,omitempty"`
	ItemType       NodePropertyType `json:"item_type"`
	ItemProperties []NodeProperty   `json:"item_properties,omitempty"`
}

type CredentialSelectionOptions struct {
	IntegrationPropertyKey string `json:"integration_property_key"`
}

type MapPropertyOptions struct {
	Properties []NodeProperty `json:"properties"`
}

type MultipleNodePropertyOption struct {
	Label             string               `json:"label"`
	Value             string               `json:"value"`
	SubNodeProperties []NodePropertyOption `json:"sub_node_properties,omitempty"`
}

type EndpointPropertyOptions struct {
	AllowedMethods []string `json:"allowed_methods,omitempty"`
}

type EndpointPropertyData struct {
	TestingURL    string `json:"testing_url"`
	ProductionURL string `json:"production_url"`
	Method        string `json:"method"`
}

func NewEndpointPropertDataFromMap(from any) (EndpointPropertyData, error) {
	m, ok := from.(map[string]any)
	if !ok {
		return EndpointPropertyData{}, fmt.Errorf("path is not a map")
	}

	testingURL, ok := m["testing_url"].(string)
	if !ok {
		return EndpointPropertyData{}, fmt.Errorf("testing_url is required in webhook route")
	}

	productionURL, ok := m["production_url"].(string)
	if !ok {
		return EndpointPropertyData{}, fmt.Errorf("production_url is required in webhook route")
	}

	method, ok := m["method"].(string)
	if !ok {
		return EndpointPropertyData{}, fmt.Errorf("method is required in webhook route")
	}

	return EndpointPropertyData{
		TestingURL:    testingURL,
		ProductionURL: productionURL,
		Method:        method,
	}, nil
}

type HandleGenerationOptions struct {
	HandleType        string             `json:"handle_type"`         // "output" or "input"
	NameFromProperty  string             `json:"name_from_property"`  // Property key to get handle name from
	TypeFromProperty  string             `json:"type_from_property"`  // Property key to get handle type from (optional)
	DefaultHandleType NodeHandleType     `json:"default_handle_type"` // Default handle type if type_from_property is empty
	Position          NodeHandlePosition `json:"position"`            // Handle position
}
