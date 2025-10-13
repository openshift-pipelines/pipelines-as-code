package info

import (
	"testing"

	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
)

func TestUserHomeDir(t *testing.T) {
	t.Setenv("HOME", "/home/user")
	assert.Equal(t, "/home/user", userHomeDir())
	// can't fake GOOS
}

func TestKubeOptsWithEnv(t *testing.T) {
	testcases := []struct {
		name     string
		env      map[string]string
		expected *KubeOpts
	}{
		{
			name: "with env",
			env: map[string]string{
				"KUBECONFIG": "/home/user/.kube/config",
			},
			expected: &KubeOpts{
				ConfigPath: "/home/user/.kube/config",
			},
		},
		{
			name: "with env",
			env: map[string]string{
				"HOME": "/home/user",
			},
			expected: &KubeOpts{
				ConfigPath: "/home/user/.kube/config",
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.env {
				t.Setenv(k, v)
			}
			k := &KubeOpts{}
			cmd := &cobra.Command{}
			k.AddFlags(cmd)
			assert.NilError(t, cmd.ParseFlags([]string{"-k /home/user/.kube/config"}))
		})
	}
}

func TestKubeOptsFlags(t *testing.T) {
	t.Setenv("HOME", "/home/user")
	t.Setenv("KUBECONFIG", "")
	defaultKubeconfigPath := "/home/user/.kube/config"
	customKubeconfigPath := "/custom/kube/config"

	testcases := []struct {
		name      string
		flags     []string
		expectNS  string
		expectCfg string
	}{
		{
			name:      "namespace flag only",
			flags:     []string{"--namespace", "test-ns"},
			expectNS:  "test-ns",
			expectCfg: defaultKubeconfigPath,
		},
		{
			name:      "namespace flag short form",
			flags:     []string{"-n", "test-ns"},
			expectNS:  "test-ns",
			expectCfg: defaultKubeconfigPath,
		},
		{
			name:      "kubeconfig flag only",
			flags:     []string{"--kubeconfig", customKubeconfigPath},
			expectNS:  "",
			expectCfg: customKubeconfigPath,
		},
		{
			name:      "both flags together",
			flags:     []string{"--namespace", "test-ns", "--kubeconfig", customKubeconfigPath},
			expectNS:  "test-ns",
			expectCfg: customKubeconfigPath,
		},
		{
			name:      "both flags short form",
			flags:     []string{"-n", "test-ns", "-k", customKubeconfigPath},
			expectNS:  "test-ns",
			expectCfg: customKubeconfigPath,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			k := &KubeOpts{}
			cmd := &cobra.Command{}
			k.AddFlags(cmd)
			assert.NilError(t, cmd.ParseFlags(tc.flags))
			assert.Equal(t, k.Namespace, tc.expectNS, "namespace mismatch")
			assert.Equal(t, k.ConfigPath, tc.expectCfg, "config path mismatch")
		})
	}
}
