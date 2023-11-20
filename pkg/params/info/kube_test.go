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
