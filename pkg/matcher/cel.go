package matcher

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/gobwas/glob"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/decls"
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

func celEvaluate(ctx context.Context, expr string, event *info.Event, vcx provider.Interface) (ref.Val, error) {
	eventTitle := event.PullRequestTitle
	if event.TriggerTarget == triggertype.Push {
		eventTitle = event.SHATitle
		// For push event the target_branch & source_branch info coming from payload have refs/heads/
		// but user may or mayn't provide refs/heads/ info while giving target_branch or source_branch in CEL expression
		// ex:  pipelinesascode.tekton.dev/on-cel-expression: |
		//        event == "push" && target_branch == "main" && "frontend/***".pathChanged()
		// This logic will handle such case.
		splittedValue := strings.Split(expr, "&&")
		for i := range splittedValue {
			if strings.Contains(splittedValue[i], "target_branch") {
				if !strings.Contains(splittedValue[i], "refs/heads/") {
					event.BaseBranch = strings.TrimPrefix(event.BaseBranch, "refs/heads/")
				}
			}
			if strings.Contains(splittedValue[i], "source_branch") {
				if !strings.Contains(splittedValue[i], "refs/heads/") {
					event.HeadBranch = strings.TrimPrefix(event.HeadBranch, "refs/heads/")
				}
			}
		}
	}

	nbody, err := json.Marshal(event.Event)
	if err != nil {
		return nil, err
	}
	var jsonMap map[string]any
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

	data := map[string]any{
		"event":         event.TriggerTarget.String(),
		"event_title":   eventTitle,
		"target_branch": event.BaseBranch,
		"source_branch": event.HeadBranch,
		"target_url":    event.BaseURL,
		"source_url":    event.HeadURL,
		"body":          jsonMap,
		"headers":       headerMap,
		"files": map[string]any{
			"all":      changedFiles.All,
			"added":    changedFiles.Added,
			"deleted":  changedFiles.Deleted,
			"modified": changedFiles.Modified,
			"renamed":  changedFiles.Renamed,
		},
	}
	env, err := cel.NewEnv(
		cel.Lib(celPac{vcx, ctx, event}),
		cel.VariableDecls(
			decls.NewVariable("event", types.StringType),
			decls.NewVariable("headers", types.NewMapType(types.StringType, types.DynType)),
			decls.NewVariable("body", types.NewMapType(types.StringType, types.DynType)),
			decls.NewVariable("event_title", types.StringType),
			decls.NewVariable("target_branch", types.StringType),
			decls.NewVariable("source_branch", types.StringType),
			decls.NewVariable("target_url", types.StringType),
			decls.NewVariable("source_url", types.StringType),
			decls.NewVariable("files", types.NewMapType(types.StringType, types.DynType)),
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
