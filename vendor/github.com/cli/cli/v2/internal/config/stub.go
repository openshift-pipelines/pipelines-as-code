package config

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/cli/cli/v2/internal/keyring"
	ghConfig "github.com/cli/go-gh/v2/pkg/config"
)

func NewBlankConfig() *ConfigMock {
	return NewFromString(defaultConfigStr)
}

func NewFromString(cfgStr string) *ConfigMock {
	c := ghConfig.ReadFromString(cfgStr)
	cfg := cfg{c}
	mock := &ConfigMock{}
	mock.GetOrDefaultFunc = func(host, key string) (string, error) {
		return cfg.GetOrDefault(host, key)
	}
	mock.SetFunc = func(host, key, value string) {
		cfg.Set(host, key, value)
	}
	mock.WriteFunc = func() error {
		return cfg.Write()
	}
	mock.MigrateFunc = func(m Migration) error {
		return cfg.Migrate(m)
	}
	mock.AliasesFunc = func() *AliasConfig {
		return &AliasConfig{cfg: c}
	}
	mock.AuthenticationFunc = func() *AuthConfig {
		return &AuthConfig{
			cfg: c,
			defaultHostOverride: func() (string, string) {
				return "github.com", "default"
			},
			hostsOverride: func() []string {
				keys, _ := c.Keys([]string{hostsKey})
				return keys
			},
			tokenOverride: func(hostname string) (string, string) {
				token, _ := c.Get([]string{hostsKey, hostname, oauthTokenKey})
				return token, oauthTokenKey
			},
		}
	}
	mock.BrowserFunc = func(hostname string) string {
		val, _ := cfg.GetOrDefault(hostname, browserKey)
		return val
	}
	mock.EditorFunc = func(hostname string) string {
		val, _ := cfg.GetOrDefault(hostname, editorKey)
		return val
	}
	mock.GitProtocolFunc = func(hostname string) string {
		val, _ := cfg.GetOrDefault(hostname, gitProtocolKey)
		return val
	}
	mock.HTTPUnixSocketFunc = func(hostname string) string {
		val, _ := cfg.GetOrDefault(hostname, httpUnixSocketKey)
		return val
	}
	mock.PagerFunc = func(hostname string) string {
		val, _ := cfg.GetOrDefault(hostname, pagerKey)
		return val
	}
	mock.PromptFunc = func(hostname string) string {
		val, _ := cfg.GetOrDefault(hostname, promptKey)
		return val
	}
	mock.VersionFunc = func() string {
		val, _ := cfg.GetOrDefault("", versionKey)
		return val
	}
	return mock
}

// NewIsolatedTestConfig sets up a Mock keyring, creates a blank config
// overwrites the ghConfig.Read function that returns a singleton config
// in the real implementation, sets the GH_CONFIG_DIR env var so that
// any call to Write goes to a different location on disk, and then returns
// the blank config and a function that reads any data written to disk.
func NewIsolatedTestConfig(t *testing.T) (Config, func(io.Writer, io.Writer)) {
	keyring.MockInit()

	c := ghConfig.ReadFromString("")
	cfg := cfg{c}

	// The real implementation of config.Read uses a sync.Once
	// to read config files and initialise package level variables
	// that are used from then on.
	//
	// This means that tests can't be isolated from each other, so
	// we swap out the function here to return a new config each time.
	ghConfig.Read = func(_ *ghConfig.Config) (*ghConfig.Config, error) {
		return c, nil
	}

	// The config.Write method isn't defined in the same way as Read to allow
	// the function to be swapped out and it does try to write to disk.
	//
	// We should consider whether it makes sense to change that but in the meantime
	// we can use GH_CONFIG_DIR env var to ensure the tests remain isolated.
	readConfigs := StubWriteConfig(t)

	return &cfg, readConfigs
}

// StubWriteConfig stubs out the filesystem where config file are written.
// It then returns a function that will read in the config files into io.Writers.
// It automatically cleans up environment variables and written files.
func StubWriteConfig(t *testing.T) func(io.Writer, io.Writer) {
	t.Helper()
	tempDir := t.TempDir()
	t.Setenv("GH_CONFIG_DIR", tempDir)
	return func(wc io.Writer, wh io.Writer) {
		config, err := os.Open(filepath.Join(tempDir, "config.yml"))
		if err != nil {
			return
		}
		defer config.Close()
		configData, err := io.ReadAll(config)
		if err != nil {
			return
		}
		_, err = wc.Write(configData)
		if err != nil {
			return
		}

		hosts, err := os.Open(filepath.Join(tempDir, "hosts.yml"))
		if err != nil {
			return
		}
		defer hosts.Close()
		hostsData, err := io.ReadAll(hosts)
		if err != nil {
			return
		}
		_, err = wh.Write(hostsData)
		if err != nil {
			return
		}
	}
}
