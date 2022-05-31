package webhook

import (
	"encoding/json"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	testnewrepo "github.com/openshift-pipelines/pipelines-as-code/pkg/test/repository"
	"gotest.tools/v3/assert"
	v1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestReconciler_Admit(t *testing.T) {
	tests := []struct {
		name    string
		repo    *v1alpha1.Repository
		allowed bool
		result  string
	}{
		{
			name: "allow",
			repo: testnewrepo.NewRepo(testnewrepo.RepoTestcreationOpts{
				Name:             "test-run",
				InstallNamespace: "namespace",
				URL:              "https://github.com/openshift-pipelines/pipelines-as-code",
			}),
			allowed: true,
			result:  "",
		},
		{
			name: "reject",
			repo: testnewrepo.NewRepo(testnewrepo.RepoTestcreationOpts{
				Name:             "test-run",
				InstallNamespace: "namespace",
				URL:              "https://pac.test/already/installed",
			}),
			allowed: false,
			result:  "repository already exist with url: https://pac.test/already/installed",
		},
		{
			name: "allow as it is be update to existing repo",
			repo: testnewrepo.NewRepo(testnewrepo.RepoTestcreationOpts{
				Name:             "test-repo-already-installed",
				InstallNamespace: "namespace",
				URL:              "https://pac.test/already/installed",
			}),
			allowed: true,
		},
		{
			name: "reject as repo namespace different",
			repo: testnewrepo.NewRepo(testnewrepo.RepoTestcreationOpts{
				Name:             "test-repo-already-installed",
				InstallNamespace: "test",
				URL:              "https://pac.test/already/installed",
			}),
			allowed: false,
			result:  "repository already exist with url: https://pac.test/already/installed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)

			alreadyInstalledRepo := testnewrepo.NewRepo(testnewrepo.RepoTestcreationOpts{
				Name:             "test-repo-already-installed",
				InstallNamespace: "namespace",
				URL:              "https://pac.test/already/installed",
			})
			tdata := testclient.Data{Repositories: []*v1alpha1.Repository{alreadyInstalledRepo}}
			stdata, _ := testclient.SeedTestData(t, ctx, tdata)

			r := reconciler{
				pacLister: stdata.RepositoryLister,
			}

			userRepo, err := json.Marshal(tt.repo)
			assert.NilError(t, err)
			req := &v1.AdmissionRequest{Object: runtime.RawExtension{Raw: userRepo}}
			res := r.Admit(ctx, req)

			assert.Equal(t, res.Allowed, tt.allowed)
			if !res.Allowed {
				assert.Equal(t, res.Result.Message, tt.result)
			}
		})
	}
}
