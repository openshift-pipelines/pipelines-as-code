package matcher

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/gobwas/glob"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/checker/decls"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/changedfiles"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/triggertype"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider"
)

const (
	reChangedFilesTags = `files\.`
)

func removeQuotes(s string) string {
	var output string
	if strings.HasPrefix(s, `"`) || strings.HasPrefix(s, `'`) {
		// Get the quote character
		quote := string(s[0])
		sWithoutPrefix := strings.TrimPrefix(s, quote)
		idx := strings.LastIndex(sWithoutPrefix, quote)
		if idx != -1 {
			output = sWithoutPrefix[:idx]
		}
	}
	return output
}

// getBranchFromExpression function extracts the value associated with the provided branchKey.
//
// ex: Given an expression like target_branch == "main", getBranchFromExpression returns "main".
func getBranchFromExpression(branchKey string) string {
	parts := strings.Split(branchKey, "==")

	var branch string
	if len(parts) == 2 {
		// Trim any leading or trailing white spaces from the value
		branch = removeQuotes(strings.TrimSpace(parts[1]))
	}
	return branch
}

func celEvaluate(ctx context.Context, expr string, event *info.Event, vcx provider.Interface) (ref.Val, error) {
	eventTitle := event.PullRequestTitle
	var (
		baseBranchValue string
		headBranchValue string
	)
	splitValue := strings.Split(expr, "&&")
	for i := range splitValue {
		if strings.Contains(strings.TrimSpace(splitValue[i]), "target_branch") {
			targetBranchValue := getBranchFromExpression(splitValue[i])
			if branchMatch(targetBranchValue, event.BaseBranch) {
				baseBranchValue = targetBranchValue
			}
		}
		if strings.Contains(strings.TrimSpace(splitValue[i]), "source_branch") {
			sourceBranchValue := getBranchFromExpression(splitValue[i])
			if branchMatch(sourceBranchValue, event.HeadBranch) {
				headBranchValue = sourceBranchValue
			}
		}
	}

	if event.TriggerTarget == triggertype.Push {
		eventTitle = event.SHATitle
	}

	nbody, err := json.Marshal(event.Event)
	if err != nil {
		return nil, err
	}
	var jsonMap map[string]interface{}
	err = json.Unmarshal(nbody, &jsonMap)
	if err != nil {
		return nil, err
	}
	headerMap := make(map[string]string)
	for k, v := range event.Request.Header {
		headerMap[strings.ToLower(k)] = v[0]
	}

	r := regexp.MustCompile(reChangedFilesTags)
	changedFiles := changedfiles.ChangedFiles{}

	if r.MatchString(expr) {
		changedFiles, err = vcx.GetFiles(ctx, event)
		if err != nil {
			return nil, err
		}
	}

	data := map[string]interface{}{
		"event":         event.TriggerTarget.String(),
		"event_title":   eventTitle,
		"target_branch": baseBranchValue,
		"source_branch": headBranchValue,
		"target_url":    event.BaseURL,
		"source_url":    event.HeadURL,
		"body":          jsonMap,
		"headers":       headerMap,
		"files": map[string]interface{}{
			"all":      changedFiles.All,
			"added":    changedFiles.Added,
			"deleted":  changedFiles.Deleted,
			"modified": changedFiles.Modified,
			"renamed":  changedFiles.Renamed,
		},
	}
	env, err := cel.NewEnv(
		cel.Lib(celPac{vcx, ctx, event}),
		cel.Declarations(
			decls.NewVar("event", decls.String),
			decls.NewVar("headers", decls.NewMapType(decls.String, decls.Dyn)),
			decls.NewVar("body", decls.NewMapType(decls.String, decls.Dyn)),
			decls.NewVar("event_title", decls.String),
			decls.NewVar("target_branch", decls.String),
			decls.NewVar("source_branch", decls.String),
			decls.NewVar("target_url", decls.String),
			decls.NewVar("source_url", decls.String),
			decls.NewVar("files", decls.NewMapType(decls.String, decls.Dyn)),
		))
	if err != nil {
		return nil, err
	}

	parsed, issues := env.Parse(expr)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("failed to parse expression %#v: %w", expr, issues.Err())
	}

	checked, issues := env.Check(parsed)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("expression %#v check failed: %w", expr, issues.Err())
	}

	prg, err := env.Program(checked)
	if err != nil {
		return nil, fmt.Errorf("expression %#v failed to create a Program: %w", expr, err)
	}

	out, _, err := prg.Eval(data)
	if err != nil {
		return nil, fmt.Errorf("expression %#v failed to evaluate: %w", expr, err)
	}
	return out, nil
}

type celPac struct {
	vcx   provider.Interface
	ctx   context.Context
	event *info.Event
}

func (t celPac) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

func (t celPac) pathChanged(vals ref.Val) ref.Val {
	var match types.Bool
	changedFiles, err := t.vcx.GetFiles(t.ctx, t.event)
	if err != nil {
		return types.Bool(false)
	}
	for i := range changedFiles.All {
		if v, ok := vals.Value().(string); ok {
			g := glob.MustCompile(v)
			if g.Match(changedFiles.All[i]) {
				return types.Bool(true)
			}
		}
		match = types.Bool(false)
	}

	return match
}

func (t celPac) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("pathChanged",
			cel.MemberOverload("pathChanged", []*cel.Type{cel.StringType}, cel.BoolType,
				cel.UnaryBinding(t.pathChanged))),
	}
}
