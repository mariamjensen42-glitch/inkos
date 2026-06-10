package model

// InputGovernanceMode controls how chapter input is composed.
type InputGovernanceMode string

const (
	InputGovernanceV2     InputGovernanceMode = "v2"
	InputGovernanceLegacy InputGovernanceMode = "legacy"
)

// ChapterReviewMode controls how the writer pipeline finalizes chapters.
type ChapterReviewMode string

const (
	ChapterReviewAuto   ChapterReviewMode = "auto"
	ChapterReviewManual ChapterReviewMode = "manual"
)

// LLMConfig is the active LLM configuration for the project.
type LLMConfig struct {
	Provider         string                 `json:"provider"`
	BaseURL          string                 `json:"baseUrl,omitempty"`
	APIKey           string                 `json:"apiKey,omitempty"`
	APIKeyEnv        string                 `json:"apiKeyEnv,omitempty"`
	Model            string                 `json:"model"`
	Service          string                 `json:"service,omitempty"`
	Temperature      *float64               `json:"temperature,omitempty"`
	MaxTokens        *int                   `json:"maxTokens,omitempty"`
	ThinkingBudget   *int                   `json:"thinkingBudget,omitempty"`
	Stream           *bool                  `json:"stream,omitempty"`
	Extra            map[string]interface{} `json:"extra,omitempty"`
}

// AgentLLMOverride routes a specific agent to a different model.
type AgentLLMOverride struct {
	Model     string `json:"model,omitempty"`
	Provider  string `json:"provider,omitempty"`
	BaseURL   string `json:"baseUrl,omitempty"`
	APIKeyEnv string `json:"apiKeyEnv,omitempty"`
}

// DetectionConfig governs AIGC detection runs.
type DetectionConfig struct {
	Enabled  *bool    `json:"enabled,omitempty"`
	Provider string   `json:"provider,omitempty"`
	Model    string   `json:"model,omitempty"`
	Dimensions []string `json:"dimensions,omitempty"`
}

// QualityGates sets per-dimension quality thresholds.
type QualityGates struct {
	MinOverallPass float64            `json:"minOverallPass,omitempty"`
	DimensionMin   map[string]float64 `json:"dimensionMin,omitempty"`
}

// FoundationConfig controls book-foundation (architect) generation.
type FoundationConfig struct {
	ReviewRetries int `json:"reviewRetries,omitempty"`
}

// WritingConfig controls chapter writing behavior.
type WritingConfig struct {
	ReviewRetries int               `json:"reviewRetries,omitempty"`
	ReviewMode    ChapterReviewMode `json:"reviewMode,omitempty"`
	WordsTarget   int               `json:"wordsTarget,omitempty"`
	WordsRange    []int             `json:"wordsRange,omitempty"`
}

// NotifyChannel is one push-notification destination.
type NotifyChannel struct {
	Type    string                 `json:"type"` // telegram / feishu / wechat-work / webhook
	Enabled bool                   `json:"enabled"`
	Config  map[string]interface{} `json:"config,omitempty"`
}

// DaemonConfig controls the background daemon.
type DaemonConfig struct {
	Schedule struct {
		RadarCron string `json:"radarCron"`
		WriteCron string `json:"writeCron"`
	} `json:"schedule"`
	MaxConcurrentBooks int `json:"maxConcurrentBooks,omitempty"`
}

// ProjectConfig is the persisted project configuration (inkos.json).
type ProjectConfig struct {
	Name                string                    `json:"name"`
	Version             string                    `json:"version"`
	Language            string                    `json:"language"`
	Notify              []NotifyChannel           `json:"notify"`
	Daemon              DaemonConfig              `json:"daemon"`
	LLM                 LLMConfig                 `json:"llm"`
	Services            map[string]LLMConfig      `json:"services,omitempty"`
	CurrentService      string                    `json:"currentService,omitempty"`
	DefaultModel        string                    `json:"defaultModel,omitempty"`
	InputGovernanceMode InputGovernanceMode       `json:"inputGovernanceMode,omitempty"`
	Detection           DetectionConfig           `json:"detection,omitempty"`
	QualityGates        QualityGates              `json:"qualityGates,omitempty"`
	Foundation          FoundationConfig          `json:"foundation,omitempty"`
	Writing             WritingConfig             `json:"writing,omitempty"`
	ModelOverrides      map[string]AgentLLMOverride `json:"modelOverrides,omitempty"`
	Cover               *LLMConfig                `json:"cover,omitempty"`
}

// SecretEntry is one API key row in .inkos/secrets.json.
type SecretEntry struct {
	APIKey string `json:"apiKey"`
}

// NewSecretEntry is a constructor for clarity at call sites.
func NewSecretEntry(apiKey string) SecretEntry { return SecretEntry{APIKey: apiKey} }

// NewSecretMap returns an empty service->entry map.
func NewSecretMap() map[string]SecretEntry { return map[string]SecretEntry{} }

// Secrets is the persisted API-key vault (.inkos/secrets.json).
type Secrets struct {
	Services map[string]SecretEntry `json:"services"`
}
