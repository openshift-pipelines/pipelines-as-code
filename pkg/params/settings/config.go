package settings

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/mcuadros/go-defaults"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	ApplicationNameKey = "application-name"
	HubURLKey          = "hub-url"
	HubCatalogNameKey  = "hub-catalog-name"
	//nolint: gosec
	MaxKeepRunUpperLimitKey               = "max-keep-run-upper-limit"
	DefaultMaxKeepRunsKey                 = "default-max-keep-runs"
	RemoteTasksKey                        = "remote-tasks"
	BitbucketCloudCheckSourceIPKey        = "bitbucket-cloud-check-source-ip"
	BitbucketCloudAdditionalSourceIPKey   = "bitbucket-cloud-additional-source-ip"
	TektonDashboardURLKey                 = "tekton-dashboard-url"
	AutoConfigureNewGitHubRepoKey         = "auto-configure-new-github-repo"
	AutoConfigureRepoNamespaceTemplateKey = "auto-configure-repo-namespace-template"

	CustomConsoleNameKey      = "custom-console-name"
	CustomConsoleURLKey       = "custom-console-url"
	CustomConsolePRDetailKey  = "custom-console-url-pr-details"
	CustomConsolePRTaskLogKey = "custom-console-url-pr-tasklog"

	SecretAutoCreateKey                          = "secret-auto-create"
	secretAutoCreateDefaultValue                 = "true"
	SecretGhAppTokenRepoScopedKey                = "secret-github-app-token-scoped" //nolint: gosec
	secretGhAppTokenRepoScopedDefaultValue       = "true"
	SecretGhAppTokenScopedExtraReposKey          = "secret-github-app-scope-extra-repos" //nolint: gosec
	secretGhAppTokenScopedExtraReposDefaultValue = ""                                    //nolint: gosec

	remoteTasksDefaultValue                 = "true"
	bitbucketCloudCheckSourceIPDefaultValue = "true"
	PACApplicationNameDefaultValue          = "Pipelines as Code CI"
	HubURLDefaultValue                      = "https://api.hub.tekton.dev/v1"
	HubCatalogNameDefaultValue              = "tekton"
	AutoConfigureNewGitHubRepoDefaultValue  = "false"

	ErrorLogSnippetKey   = "error-log-snippet"
	errorLogSnippetValue = "true"

	ErrorDetectionKey   = "error-detection-from-container-logs"
	errorDetectionValue = "true"

	ErrorDetectionNumberOfLinesKey   = "error-detection-max-number-of-lines"
	errorDetectionNumberOfLinesValue = 50

	ErrorDetectionSimpleRegexpKey   = "error-detection-simple-regexp"
	errorDetectionSimpleRegexpValue = `^(?P<filename>[^:]*):(?P<line>[0-9]+):(?P<column>[0-9]+):([ ]*)?(?P<error>.*)`

	RememberOKToTestKey   = "remember-ok-to-test"
	rememberOKToTestValue = "true"
)

var (
	TknBinaryName       = `tkn`
	hubCatalogNameRegex = regexp.MustCompile(`^catalog-(\d+)-`)
)

type HubCatalog struct {
	ID   string
	Name string
	URL  string
}

type Settings struct {
	ApplicationName string `mapstructure:"application-name" default:"Pipelines as Code CI"`
	// HubURL                             string
	// HubCatalogName                     string
	HubCatalogs           *sync.Map
	RemoteTasks           bool   `mapstructure:"remote-tasks" default:"true"`
	MaxKeepRunsUpperLimit int    `mapstructure:"max-keep-run-upper-limit" `
	DefaultMaxKeepRuns    int    `mapstructure:"default-max-keep-runs" `
	TektonDashboardURL    string `mapstructure:"tekton-dashboard-url"`

	AutoConfigureNewGitHubRepo         bool   `mapstructure:"auto-configure-new-github-repo" default:"false"`
	AutoConfigureRepoNamespaceTemplate string `mapstructure:"auto-configure-repo-namespace-template" `

	BitbucketCloudCheckSourceIP      bool   `mapstructure:"bitbucket-cloud-check-source-ip" default:"true"`
	BitbucketCloudAdditionalSourceIP string `mapstructure:"bitbucket-cloud-additional-source-ip" `

	SecretAutoCreation               bool   `mapstructure:"secret-auto-create" default:"true"`
	SecretGHAppRepoScoped            bool   `mapstructure:"secret-github-app-token-scoped" default:"true"`
	SecretGhAppTokenScopedExtraRepos string `mapstructure:"secret-github-app-scope-extra-repos"`

	ErrorLogSnippet             bool   `mapstructure:"error-log-snippet" default:"true"`
	ErrorDetection              bool   `mapstructure:"error-detection-from-container-logs" default:"true"`
	ErrorDetectionNumberOfLines int    `mapstructure:"error-detection-max-number-of-lines" default:"50"`
	ErrorDetectionSimpleRegexp  string `mapstructure:"error-detection-simple-regexp" default:"^(?P<filename>[^:]*):(?P<line>[0-9]+):(?P<column>[0-9]+):([ ]*)?(?P<error>.*)"`

	CustomConsoleName      string `mapstructure:"custom-console-name" `
	CustomConsoleURL       string `mapstructure:"custom-console-url" `
	CustomConsolePRdetail  string `mapstructure:"custom-console-url-pr-details" `
	CustomConsolePRTaskLog string `mapstructure:"custom-console-url-pr-tasklog" `

	RememberOKToTest bool `mapstructure:"remember-ok-to-test" default:"true"`
}

func ReadConfig(config *viper.Viper, logger *zap.SugaredLogger) (*Settings, error) {
	setting := &Settings{}

	// Run through defaulting before updating the values
	defaults.SetDefaults(setting)

	err := config.Unmarshal(&setting)
	if err != nil {
		return nil, fmt.Errorf("failed to read the config into settings: %w", err)
	}

	// read hub catalogs
	setting.HubCatalogs = readHubCatalog(config, logger)

	// add validation for configuration values here if required
	if err := validate(config, setting, logger); err != nil {
		return nil, err
	}

	return setting, nil
}

func validate(config *viper.Viper, setting *Settings, logger *zap.SugaredLogger) error {
	if setting.TektonDashboardURL != "" {
		if _, err := url.ParseRequestURI(setting.TektonDashboardURL); err != nil {
			return fmt.Errorf("invalid value %v for key tekton-dashboard-url, invalid url: %w", setting.TektonDashboardURL, err)
		}
	}

	if setting.ErrorDetectionSimpleRegexp != "" {
		if _, err := regexp.Compile(setting.ErrorDetectionSimpleRegexp); err != nil {
			return fmt.Errorf("cannot use %v as regexp for error-detection-simple-regexp: %w", setting.ErrorDetectionSimpleRegexp, err)
		}
	}

	if setting.CustomConsoleURL != "" {
		if _, err := url.ParseRequestURI(setting.CustomConsoleURL); err != nil {
			return fmt.Errorf("invalid value %v for key custom-console-url, invalid url: %w", setting.CustomConsoleURL, err)
		}
	}

	if setting.CustomConsolePRTaskLog != "" {
		// check if custom console start with http:// or https://
		if strings.HasPrefix(setting.CustomConsolePRTaskLog, "http://") || !strings.HasPrefix(setting.CustomConsolePRTaskLog, "https://") {
			return fmt.Errorf("invalid value %v for key custom-console-url-pr-tasklog, must start with http:// or https://", setting.CustomConsolePRTaskLog)
		}
	}

	if setting.CustomConsolePRdetail != "" {
		if strings.HasPrefix(setting.CustomConsolePRdetail, "http://") || !strings.HasPrefix(setting.CustomConsolePRdetail, "https://") {
			return fmt.Errorf("invalid value %v for key custom-console-url-pr-details, must start with http:// or https://", setting.CustomConsolePRdetail)
		}
	}

	value, _ := setting.HubCatalogs.Load("default")
	catalogDefault, ok := value.(HubCatalog)
	if ok {
		if catalogDefault.URL != config.GetString(HubURLKey) {
			logger.Infof("CONFIG: hub URL set to %v", config.GetString(HubURLKey))
			catalogDefault.URL = config.GetString(HubURLKey)
		}
		if catalogDefault.Name != config.GetString(HubCatalogNameKey) {
			logger.Infof("CONFIG: hub catalog name set to %v", config.GetString(HubCatalogNameKey))
			catalogDefault.Name = config.GetString(HubCatalogNameKey)
		}
	}
	setting.HubCatalogs.Store("default", catalogDefault)
	return nil
}

func readHubCatalog(config *viper.Viper, logger *zap.SugaredLogger) *sync.Map {
	catalogs := sync.Map{}
	if hubURL := config.GetString(HubURLKey); hubURL == "" {
		config.Set(HubURLKey, HubURLDefaultValue)
		logger.Infof("CONFIG: using default hub url %s", HubURLDefaultValue)
	}

	if hubCatalogName := config.GetString(HubCatalogNameKey); hubCatalogName == "" {
		config.Set(HubCatalogNameKey, HubCatalogNameDefaultValue)
	}
	catalogs.Store("default", HubCatalog{
		ID:   "default",
		Name: config.GetString(HubCatalogNameKey),
		URL:  config.GetString(HubURLKey),
	})

	for _, k := range config.AllKeys() {
		m := hubCatalogNameRegex.FindStringSubmatch(k)
		if len(m) > 0 {
			id := m[1]
			cPrefix := fmt.Sprintf("catalog-%s", id)
			skip := false
			for _, kk := range []string{"id", "name", "url"} {
				cKey := fmt.Sprintf("%s-%s", cPrefix, kk)
				// check if key exist in config
				if key := config.GetString(cKey); key == "" {
					logger.Warnf("CONFIG: hub %v should have the key %s, skipping catalog configuration", id, cKey)
					skip = true
					break
				}
			}
			if !skip {
				catalogID := config.GetString(fmt.Sprintf("%s-id", cPrefix))
				if catalogID == "http" || catalogID == "https" {
					logger.Warnf("CONFIG: custom hub catalog name cannot be %s, skipping catalog configuration", catalogID)
					break
				}
				catalogURL := config.GetString(fmt.Sprintf("%s-url", cPrefix))
				u, err := url.Parse(catalogURL)
				if err != nil || u.Scheme == "" || u.Host == "" {
					logger.Warnf("CONFIG: custom hub %s, catalog url %s is not valid, skipping catalog configuration", catalogID, catalogURL)
					break
				}
				logger.Infof("CONFIG: setting custom hub %s, catalog %s", catalogID, catalogURL)
				catalogs.Store(catalogID, HubCatalog{
					ID:   catalogID,
					Name: config.GetString(fmt.Sprintf("%s-name", cPrefix)),
					URL:  catalogURL,
				})
			}
		}
	}
	return &catalogs
}

func ConfigToSettings(logger *zap.SugaredLogger, setting *Settings, config map[string]string) error {
	// pass through defaulting
	SetDefaults(config)
	setting.HubCatalogs = getHubCatalogs(logger, setting.HubCatalogs, config)

	// validate fields
	if err := Validate(config); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	if setting.ApplicationName != config[ApplicationNameKey] {
		logger.Infof("CONFIG: application name set to %v", config[ApplicationNameKey])
		setting.ApplicationName = config[ApplicationNameKey]
	}

	secretAutoCreate := StringToBool(config[SecretAutoCreateKey])
	if setting.SecretAutoCreation != secretAutoCreate {
		logger.Infof("CONFIG: secret auto create set to %v", secretAutoCreate)
		setting.SecretAutoCreation = secretAutoCreate
	}

	secretGHAppRepoScoped := StringToBool(config[SecretGhAppTokenRepoScopedKey])
	if setting.SecretGHAppRepoScoped != secretGHAppRepoScoped {
		logger.Infof("CONFIG: not scoping the token generated from gh %v", secretGHAppRepoScoped)
		setting.SecretGHAppRepoScoped = secretGHAppRepoScoped
	}

	secretGHAppScopedExtraRepos := config[SecretGhAppTokenScopedExtraReposKey]
	if setting.SecretGhAppTokenScopedExtraRepos != secretGHAppScopedExtraRepos {
		logger.Infof("CONFIG: adding extra repositories for github app token scope %v", secretGHAppRepoScoped)
		setting.SecretGhAppTokenScopedExtraRepos = secretGHAppScopedExtraRepos
	}

	value, _ := setting.HubCatalogs.Load("default")
	catalogDefault, ok := value.(HubCatalog)
	if ok {
		if catalogDefault.URL != config[HubURLKey] {
			logger.Infof("CONFIG: hub URL set to %v", config[HubURLKey])
			catalogDefault.URL = config[HubURLKey]
		}
		if catalogDefault.Name != config[HubCatalogNameKey] {
			logger.Infof("CONFIG: hub catalog name set to %v", config[HubCatalogNameKey])
			catalogDefault.Name = config[HubCatalogNameKey]
		}
	}
	setting.HubCatalogs.Store("default", catalogDefault)
	// TODO: detect changes in extra hub catalogs

	remoteTask := StringToBool(config[RemoteTasksKey])
	if setting.RemoteTasks != remoteTask {
		logger.Infof("CONFIG: remote tasks setting set to %v", remoteTask)
		setting.RemoteTasks = remoteTask
	}
	maxKeepRunUpperLimit, _ := strconv.Atoi(config[MaxKeepRunUpperLimitKey])
	if setting.MaxKeepRunsUpperLimit != maxKeepRunUpperLimit {
		logger.Infof("CONFIG: max keep runs upper limit set to %v", maxKeepRunUpperLimit)
		setting.MaxKeepRunsUpperLimit = maxKeepRunUpperLimit
	}
	defaultMaxKeepRun, _ := strconv.Atoi(config[DefaultMaxKeepRunsKey])
	if setting.DefaultMaxKeepRuns != defaultMaxKeepRun {
		logger.Infof("CONFIG: default keep runs set to %v", defaultMaxKeepRun)
		setting.DefaultMaxKeepRuns = defaultMaxKeepRun
	}
	check := StringToBool(config[BitbucketCloudCheckSourceIPKey])
	if setting.BitbucketCloudCheckSourceIP != check {
		logger.Infof("CONFIG: bitbucket cloud check source ip setting set to %v", check)
		setting.BitbucketCloudCheckSourceIP = check
	}
	if setting.BitbucketCloudAdditionalSourceIP != config[BitbucketCloudAdditionalSourceIPKey] {
		logger.Infof("CONFIG: bitbucket cloud additional source ip set to %v", config[BitbucketCloudAdditionalSourceIPKey])
		setting.BitbucketCloudAdditionalSourceIP = config[BitbucketCloudAdditionalSourceIPKey]
	}
	if setting.TektonDashboardURL != config[TektonDashboardURLKey] {
		logger.Infof("CONFIG: tekton dashboard url set to %v", config[TektonDashboardURLKey])
		setting.TektonDashboardURL = config[TektonDashboardURLKey]
	}
	autoConfigure := StringToBool(config[AutoConfigureNewGitHubRepoKey])
	if setting.AutoConfigureNewGitHubRepo != autoConfigure {
		logger.Infof("CONFIG: auto configure GitHub repo setting set to %v", autoConfigure)
		setting.AutoConfigureNewGitHubRepo = autoConfigure
	}
	if setting.AutoConfigureRepoNamespaceTemplate != config[AutoConfigureRepoNamespaceTemplateKey] {
		logger.Infof("CONFIG: auto configure repo namespace template set to %v", config[AutoConfigureRepoNamespaceTemplateKey])
		setting.AutoConfigureRepoNamespaceTemplate = config[AutoConfigureRepoNamespaceTemplateKey]
	}

	errorLogSnippet := StringToBool(config[ErrorLogSnippetKey])
	if setting.ErrorLogSnippet != errorLogSnippet {
		logger.Infof("CONFIG: setting log snippet on error to %v", errorLogSnippet)
		setting.ErrorLogSnippet = errorLogSnippet
	}

	errorDetection := StringToBool(config[ErrorDetectionKey])
	if setting.ErrorDetection != errorDetection {
		logger.Infof("CONFIG: setting error detection to %v", errorDetection)
		setting.ErrorDetection = errorDetection
	}

	errorDetectNumberOfLines, _ := strconv.Atoi(config[ErrorDetectionNumberOfLinesKey])
	if setting.ErrorDetection && setting.ErrorDetectionNumberOfLines != errorDetectNumberOfLines {
		logger.Infof("CONFIG: setting error detection limit of container log to %v", errorDetectNumberOfLines)
		setting.ErrorDetectionNumberOfLines = errorDetectNumberOfLines
	}

	if setting.ErrorDetection && setting.ErrorDetectionSimpleRegexp != strings.TrimSpace(config[ErrorDetectionSimpleRegexpKey]) {
		// replace double backslash with single backslash because kube configmap is giving us things double backslashes
		logger.Infof("CONFIG: setting error detection regexp to %v", strings.TrimSpace(config[ErrorDetectionSimpleRegexpKey]))
		setting.ErrorDetectionSimpleRegexp = strings.TrimSpace(config[ErrorDetectionSimpleRegexpKey])
	}

	if setting.CustomConsoleName != config[CustomConsoleNameKey] {
		logger.Infof("CONFIG: setting custom console name to %v", config[CustomConsoleNameKey])
		setting.CustomConsoleName = config[CustomConsoleNameKey]
	}

	if setting.CustomConsoleURL != config[CustomConsoleURLKey] {
		logger.Infof("CONFIG: setting custom console url to %v", config[CustomConsoleURLKey])
		setting.CustomConsoleURL = config[CustomConsoleURLKey]
	}

	if setting.CustomConsolePRdetail != config[CustomConsolePRDetailKey] {
		logger.Infof("CONFIG: setting custom console pr detail URL to %v", config[CustomConsolePRDetailKey])
		setting.CustomConsolePRdetail = config[CustomConsolePRDetailKey]
	}

	if setting.CustomConsolePRTaskLog != config[CustomConsolePRTaskLogKey] {
		logger.Infof("CONFIG: setting custom console pr task log URL to %v", config[CustomConsolePRTaskLogKey])
		setting.CustomConsolePRTaskLog = config[CustomConsolePRTaskLogKey]
	}

	rememberOKToTest := StringToBool(config[RememberOKToTestKey])
	if setting.RememberOKToTest != rememberOKToTest {
		logger.Infof("CONFIG: setting remember ok-to-test to %v", rememberOKToTest)
		setting.RememberOKToTest = rememberOKToTest
	}

	return nil
}

func StringToBool(s string) bool {
	if strings.ToLower(s) == "true" ||
		strings.ToLower(s) == "yes" || s == "1" {
		return true
	}
	return false
}
