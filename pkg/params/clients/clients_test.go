package clients

import (
	"testing"

	httptesthelper "github.com/openshift-pipelines/pipelines-as-code/pkg/test/http"
	"gotest.tools/v3/assert"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestClients_GetURL(t *testing.T) {
	tests := []struct {
		name       string
		remoteURLS map[string]map[string]string
		want       string
		wantErr    bool
		url        string
	}{
		{
			name: "good",
			remoteURLS: map[string]map[string]string{
				"http://blahblah": {
					"body": "hellomoto",
					"code": "200",
				},
			},
			want: "hellomoto",
			url:  "http://blahblah",
		},
		{
			name: "bad",
			remoteURLS: map[string]map[string]string{
				"http://blahblah": {
					"code": "404",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			httpTestClient := httptesthelper.MakeHTTPTestClient(tt.remoteURLS)
			c := &Clients{
				HTTP: *httpTestClient,
			}
			got, err := c.GetURL(ctx, tt.url)
			if tt.wantErr {
				assert.Assert(t, err != nil)
				return
			}
			assert.NilError(t, err, "Clients.GetURL() error = %v, wantErr %v", err, tt.wantErr)
			assert.Equal(t, string(got), tt.want)
		})
	}
}
