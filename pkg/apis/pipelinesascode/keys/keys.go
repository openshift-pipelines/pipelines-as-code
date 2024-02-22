/*
CoCopyright 2022 Red Hat

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package keys

import (
	"regexp"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
)

const (
	ControllerInfo  = pipelinesascode.GroupName + "/controller-info"
	Task            = pipelinesascode.GroupName + "/task"
	Pipeline        = pipelinesascode.GroupName + "/pipeline"
	URLOrg          = pipelinesascode.GroupName + "/url-org"
	URLRepository   = pipelinesascode.GroupName + "/url-repository"
	SHA             = pipelinesascode.GroupName + "/sha"
	Sender          = pipelinesascode.GroupName + "/sender"
	EventType       = pipelinesascode.GroupName + "/event-type"
	Branch          = pipelinesascode.GroupName + "/branch"
	Repository      = pipelinesascode.GroupName + "/repository"
	GitProvider     = pipelinesascode.GroupName + "/git-provider"
	State           = pipelinesascode.GroupName + "/state"
	ShaTitle        = pipelinesascode.GroupName + "/sha-title"
	ShaURL          = pipelinesascode.GroupName + "/sha-url"
	RepoURL         = pipelinesascode.GroupName + "/repo-url"
	PullRequest     = pipelinesascode.GroupName + "/pull-request"
	InstallationID  = pipelinesascode.GroupName + "/installation-id"
	GHEURL          = pipelinesascode.GroupName + "/ghe-url"
	SourceProjectID = pipelinesascode.GroupName + "/source-project-id"
	TargetProjectID = pipelinesascode.GroupName + "/target-project-id"
	OriginalPRName  = pipelinesascode.GroupName + "/original-prname"
	GitAuthSecret   = pipelinesascode.GroupName + "/git-auth-secret"
	CheckRunID      = pipelinesascode.GroupName + "/check-run-id"
	OnEvent         = pipelinesascode.GroupName + "/on-event"
	OnComment       = pipelinesascode.GroupName + "/on-comment"
	OnTargetBranch  = pipelinesascode.GroupName + "/on-target-branch"
	OnCelExpression = pipelinesascode.GroupName + "/on-cel-expression"
	TargetNamespace = pipelinesascode.GroupName + "/target-namespace"
	MaxKeepRuns     = pipelinesascode.GroupName + "/max-keep-runs"
	LogURL          = pipelinesascode.GroupName + "/log-url"
	ExecutionOrder  = pipelinesascode.GroupName + "/execution-order"
	// PublicGithubAPIURL default is "https://api.github.com" but it can be overridden by X-GitHub-Enterprise-Host header.
	PublicGithubAPIURL = "https://api.github.com"
	// InstallationURL gives us the Installation ID for the GitHub Application.
	InstallationURL      = "/app/installations"
	GithubApplicationID  = "github-application-id"
	GithubPrivateKey     = "github-private-key"
	ResultsRecordSummary = "results.tekton.dev/recordSummaryAnnotations"
)

var ParamsRe = regexp.MustCompile(`{{([^}]{2,})}}`)
