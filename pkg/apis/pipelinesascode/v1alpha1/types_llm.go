package v1alpha1

const (
	// DefaultContainerLogsMaxLines is the default maximum number of log lines to fetch per container.
	DefaultContainerLogsMaxLines = 50
)

// AIAnalysisConfig defines configuration for AI/LLM-powered analysis of CI/CD pipeline events.
type AIAnalysisConfig struct {
	// Enabled controls whether AI analysis is active for this repository
	// +kubebuilder:validation:Required
	Enabled bool `json:"enabled"`

	// Provider specifies which LLM provider to use for analysis
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=openai;gemini
	Provider string `json:"provider"`

	// TokenSecretRef references the Kubernetes secret containing the LLM provider API token
	// +kubebuilder:validation:Required
	TokenSecretRef *LLMSecret `json:"secret_ref"`

	// TimeoutSeconds sets the maximum time to wait for LLM analysis (default: 30)
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=300
	TimeoutSeconds int `json:"timeout_seconds,omitempty"`

	// MaxTokens limits the response length from the LLM (default: 1000)
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=4000
	MaxTokens int `json:"max_tokens,omitempty"`

	// Roles defines different analysis scenarios and their configurations
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Roles []AnalysisRole `json:"roles"`
}

type LLMSecret struct {
	*Secret `json:",inline"`

	// URL is an optional base URL to override the default API endpoint of the LLM provider
	// +optional
	URL string `json:"url,omitempty"`
}

// AnalysisRole defines a specific analysis scenario with its prompt, conditions, and output configuration.
type AnalysisRole struct {
	// Name is a unique identifier for this analysis role
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Prompt is the base prompt template sent to the LLM for analysis
	// +kubebuilder:validation:Required
	Prompt string `json:"prompt"`

	// OnCEL is a CEL expression that determines when this role should be triggered
	// +optional
	OnCEL string `json:"on_cel,omitempty"`

	// Output specifies where the analysis results should be sent (default: pr-comment)
	// +optional
	// +kubebuilder:default=pr-comment
	// +kubebuilder:validation:Enum=pr-comment
	Output string `json:"output,omitempty"`

	// ContextItems defines what context data to include in the analysis
	// +optional
	ContextItems *ContextConfig `json:"context_items,omitempty"`
}

// ContextConfig defines what contextual information to include in LLM analysis.
type ContextConfig struct {
	// CommitContent includes commit message and diff information
	// +optional
	CommitContent bool `json:"commit_content,omitempty"`

	// PRContent includes pull request title, description, and metadata
	// +optional
	PRContent bool `json:"pr_content,omitempty"`

	// ErrorContent includes error messages and failure summaries
	// +optional
	ErrorContent bool `json:"error_content,omitempty"`

	// ContainerLogs configures inclusion of container/task logs
	// +optional
	ContainerLogs *ContainerLogsConfig `json:"container_logs,omitempty"`
}

// ContainerLogsConfig defines how container logs should be included in analysis.
type ContainerLogsConfig struct {
	// Enabled controls whether container logs are included
	Enabled bool `json:"enabled"`

	// MaxLines limits the number of log lines to include (default: 50)
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1000
	MaxLines int `json:"max_lines,omitempty"`
}

func (c *ContainerLogsConfig) GetMaxLines() int {
	if c == nil || c.MaxLines == 0 {
		return DefaultContainerLogsMaxLines
	}
	return c.MaxLines
}

// GetOutput returns the output destination with a default value if not specified.
func (r *AnalysisRole) GetOutput() string {
	if r.Output == "" {
		return "pr-comment"
	}
	return r.Output
}
