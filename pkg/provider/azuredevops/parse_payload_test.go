package azuredevops

import (
	"context"
	"net/http"
	"testing"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/stretchr/testify/assert"
)

func TestParsePayload(t *testing.T) {
	// Mock request setup
	mockRequest := &http.Request{}

	// Mock context
	ctx := context.Background()

	tests := []struct {
		name      string
		eventType string
		payload   string
		wantErr   bool
		wantEvent *info.Event
	}{
		{
			name:      "Git Push Event",
			eventType: "git.push",
			payload: `{
				"eventType": "git.push",
				"resource": {
					"commits": [
						{
							"commitId": "71ee795c0163eedd69bdec49c1fad2403a49dd25",
							"author": {
								"name": "John"
							},
							"comment": "Renamed pull_request.yaml to pullrequest.yaml",
							"url": "https://dev.azure.com/xyz/_apis/git/repositories/f0f58388-1646-4faa-887f-15334faa07b6/commits/71ee795c0163eedd69bdec49c1fad2403a49dd25"
						}
					],
					"refUpdates": [
						{
							"name": "refs/heads/main",
							"oldObjectId": "2215f73bc8b77a5a62cf9d833ce411f243d26b0b",
							"newObjectId": "71ee795c0163eedd69bdec49c1fad2403a49dd25"
						}
					],
					"repository": {
						"id": "f0f58388-1646-4faa-887f-15334faa07b6",
						"name": "TestProject",
						"url": "https://dev.azure.com/xyz/_apis/git/repositories/f0f58388-1646-4faa-887f-15334faa07b6",
						"project": {
							"id": "31488f02-aad9-2222-4139-5a1f24bbbb86",
							"name": "TestProject"
						},
						"defaultBranch": "refs/heads/main",
						"remoteUrl": "https://dev.azure.com/xyz/TestProject/_git/TestProject"
					},
					"pushedBy": {
						"displayName": "John"
					}
				}
			}`,
			wantErr: false,
			wantEvent: &info.Event{
				SHA:          "71ee795c0163eedd69bdec49c1fad2403a49dd25",
				SHAURL:       "https://dev.azure.com/xyz/_apis/git/repositories/f0f58388-1646-4faa-887f-15334faa07b6/commits/71ee795c0163eedd69bdec49c1fad2403a49dd25",
				SHATitle:     "Renamed pull_request.yaml to pullrequest.yaml",
				Sender:       "John",
				EventType:    "git.push",
				Repository:   "https://dev.azure.com/xyz/TestProject/_git/TestProject",
				Organization: "TestProject",
			},
		},
		{
			name:      "Git Pull Request Created Event",
			eventType: "git.pullrequest.created",
			payload: `{
				"eventType": "git.pullrequest.created",
				"resource": {
					"repository": {
						"id": "f0f58388-1646-4faa-887f-15334faa07b6",
						"name": "TestProject",
						"url": "https://dev.azure.com/xyz/_apis/git/repositories/f0f58388-1646-4faa-887f-15334faa07b6",
						"project": {
							"id": "31488f02-aad9-2222-4139-5a1f24bbbb86",
							"name": "TestProject"
						},
						"webUrl": "https://dev.azure.com/xyz/TestProject/_git/TestProject"
						},
					"pullRequestId": 2,
					"title": "test",
					"sourceRefName": "refs/heads/test",
					"targetRefName": "refs/heads/main",
					"createdBy": {
						"displayName": "John"
					}
				}
			}`,
			wantErr: false,
			wantEvent: &info.Event{
				PullRequestNumber: 2,
				PullRequestTitle:  "test",
				BaseBranch:        "main",
				HeadBranch:        "test",
				EventType:         "git.pullrequest.created",
				Sender:            "John",
				Repository:        "https://dev.azure.com/xyz/TestProject/_git/TestProject",
				Organization:      "TestProject",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := Provider{} // Assuming this is your Azure DevOps provider
			run := &params.Run{}

			gotEvent, err := v.ParsePayload(ctx, run, mockRequest, tt.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParsePayload() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			assert.Equal(t, tt.wantEvent.EventType, gotEvent.EventType)
			assert.Equal(t, tt.wantEvent.SHA, gotEvent.SHA)
			// Add more assertions as needed
		})
	}
}
func TestParsePayload_Errors(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		payload   string
		wantErr   bool
	}{
		{
			name:      "Invalid JSON payload",
			eventType: "git.push",
			payload:   `{"eventType": "git.push", "resource": {`,
			wantErr:   true,
		},
		{
			name:      "Unsupported event type",
			eventType: "build.completed",
			payload: `{
				"eventType": "build.completed",
				"resource": {}
			}`,
			wantErr: true,
		},
		{
			name:      "Missing event type",
			eventType: "",
			payload: `{
				"resource": {
					"commits": [
						{"commitId": "someId"}
					]
				}
			}`,
			wantErr: true,
		},
		{
			name:      "Bad event type field",
			eventType: "git.push",
			payload: `{
				"eventType": 123,
				"resource": {}
			}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			req := &http.Request{}
			v := Provider{}

			_, err := v.ParsePayload(ctx, &params.Run{}, req, tt.payload)
			if tt.wantErr {
				assert.Error(t, err, "ParsePayload() was supposed to error")
			} else {
				assert.NoError(t, err, "ParsePayload() was not supposed to error")
			}
		})
	}
}

func TestExtractBranchName(t *testing.T) {
	tests := []struct {
		name       string
		refName    string
		wantBranch string
	}{
		{"Standard ref name", "refs/heads/master", "master"},
		{"Nested branch name", "refs/heads/feature/new-feature", "new-feature"},
		{"Non-standard format", "master", "master"},
		{"Empty ref name", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractBranchName(tt.refName); got != tt.wantBranch {
				t.Errorf("ExtractBranchName(%q) = %q, want %q", tt.refName, got, tt.wantBranch)
			}
		})
	}
}

func TestExtractBaseURL(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		wantBaseURL string
		wantErr     bool
	}{
		{"Valid Azure URL", "https://dev.azure.com/exampleOrg/exampleProject", "https://dev.azure.com/exampleOrg", false},
		{"Invalid Azure URL", "https://dev.azure.com/", "", true},
		{"Non-Azure URL", "https://example.com", "", true},
		{"Empty URL", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractBaseURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractBaseURL(%q) expected error? %v, got error? %v", tt.url, tt.wantErr, err != nil)
			}
			if got != tt.wantBaseURL {
				t.Errorf("extractBaseURL(%q) = %q, want %q", tt.url, got, tt.wantBaseURL)
			}
		})
	}
}
