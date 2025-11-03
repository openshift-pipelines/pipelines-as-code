package events

import (
	"context"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestEventEmitter_EmitMessage(t *testing.T) {
	observer, _ := zapobserver.New(zap.InfoLevel)
	fakelogger := zap.New(observer).Sugar()
	tests := []struct {
		name         string
		repo         *v1alpha1.Repository
		message      string
		reason       string
		logLevel     zapcore.Level
		expectEvent  bool
		expectedType string
		useNilLogger bool
		useNilClient bool
	}{
		{
			name: "repo exists",
			repo: &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "test-ns",
				},
				Spec: v1alpha1.RepositorySpec{},
			},
			message:      "info-message",
			logLevel:     zap.InfoLevel,
			expectEvent:  true,
			expectedType: v1.EventTypeNormal,
		},
		{
			name: "event with a reason",
			repo: &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "test-ns",
					UID:       "uid",
				},
				Spec: v1alpha1.RepositorySpec{},
			},
			message:      "info-message",
			logLevel:     zap.InfoLevel,
			expectEvent:  true,
			expectedType: v1.EventTypeNormal,
			reason:       "aintnosunshine",
		},
		{
			name:         "repo doesn't exists",
			repo:         nil,
			message:      "error-message",
			logLevel:     zap.ErrorLevel,
			expectEvent:  false,
			expectedType: v1.EventTypeWarning,
		},
		{
			name: "nil logger doesn't cause panic",
			repo: &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "test-ns",
				},
				Spec: v1alpha1.RepositorySpec{},
			},
			message:      "error-message",
			logLevel:     zap.ErrorLevel,
			expectEvent:  true,
			expectedType: v1.EventTypeWarning,
			useNilLogger: true,
		},
		{
			name: "nil client doesn't cause panic",
			repo: &v1alpha1.Repository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "test-ns",
				},
				Spec: v1alpha1.RepositorySpec{},
			},
			message:      "info-message",
			logLevel:     zap.InfoLevel,
			expectEvent:  false,
			expectedType: v1.EventTypeNormal,
			useNilClient: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			stdata, _ := testclient.SeedTestData(t, ctx, testclient.Data{})

			// Use nil logger for specific test case
			var logger *zap.SugaredLogger
			if tt.useNilLogger {
				logger = nil
			} else {
				logger = fakelogger
			}

			// Use nil client for specific test case
			var client kubernetes.Interface
			if tt.useNilClient {
				client = nil
			} else {
				client = stdata.Kube
			}

			// emit event - this should not panic even with nil logger or nil client
			NewEventEmitter(client, logger).EmitMessage(tt.repo, tt.logLevel, tt.reason, tt.message)

			if tt.expectEvent {
				events, err := stdata.Kube.CoreV1().Events(tt.repo.Namespace).List(context.Background(), metav1.ListOptions{})
				assert.NilError(t, err)
				assert.Equal(t, events.Items[0].Message, tt.message)
				assert.Equal(t, events.Items[0].Type, tt.expectedType)
				assert.Equal(t, events.Items[0].Reason, tt.reason)
				assert.Equal(t, events.Items[0].InvolvedObject.Name, tt.repo.Name)
				assert.Equal(t, events.Items[0].InvolvedObject.Namespace, tt.repo.Namespace)
				assert.Equal(t, events.Items[0].InvolvedObject.UID, tt.repo.UID)
				assert.Equal(t, events.Items[0].InvolvedObject.Kind, pipelinesascode.RepositoryKind)
				assert.Equal(t, events.Items[0].InvolvedObject.APIVersion, pipelinesascode.V1alpha1Version)
				assert.Assert(t, events.Items[0].Source.Component != "")
			}
		})
	}
}
