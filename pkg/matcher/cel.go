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
	// Utilizing this regex, the GetFiles function will be selectively executed exclusively when the "file."
	// property is specified within the CEL expression.
	changedFilesTags = "files."
)

// evaluateBranchExpression function evaluates the branch expression for the given branch key from the provided expression.
// Examples of expressions:
//   - target_branch == "refs/heads*"
//   - source_branch == "refs/heads*"
//
// If source_branch and target_branch contain anything other than "refs/heads*", the function won't perform any action.
func evaluateBranchExpression(expression, branchKey string) bool {
	env, err := cel.NewEnv(cel.Declarations(
		decls.NewVar(branchKey, decls.String),
	))
	if err != nil {
		return false
	}

	// Define data map with placeholder values.
	data := map[string]interface{}{
		branchKey: "refs/heads/*",
	}

	out, err := evaluateCELExpression(env, expression, data)
	if err != nil {
		return false
	}
	// Return evaluation result as bool.
	if boolVal, ok := out.Value().(bool); ok {
		return boolVal
	}
	return false
}

func handleBranchCondition(event *info.Event, data map[string]interface{}, splitExprData, branchKey, branchFromEvent string) {
	// For push event the target_branch & source_branch info coming from payload have refs/heads/
	// but user may or mayn't provide refs/heads/ info while giving target_branch or source_branch in CEL expression
	// ex:  pipelinesascode.tekton.dev/on-cel-expression: |
	//        event == "push" && target_branch == "main" && "frontend/***".pathChanged()
	// This logic will handle such case.
	if event.TriggerTarget == "push" {
		if !strings.Contains(splitExprData, "refs/heads/") {
			data[branchKey] = strings.TrimPrefix(branchFromEvent, "refs/heads/")
		}
	}
	if evaluateBranchExpression(splitExprData, branchKey) {
		data[branchKey] = "refs/heads/*"
	}
}

// handleTargetAndSourceBranches ensures the matches of the target and source branch and update the map with correct values
// It serves two primary purposes:
//
//  1. For push events, the target_branch & source_branch information from the payload have "refs/heads/" ,
//     but users may or mayn't provide refs/heads/ prefix for target_branch or source_branch in the CEL expression.
//     This logic handles such cases.
//     For example:
//     pipelinesascode.tekton.dev/on-cel-expression: |
//     event == "push" && target_branch == "main" && "frontend/***".pathChanged()
//
//  2. For both push and pull_request events, if users specify target_branch and source_branch as "refs/heads/*",
//     indicating acceptance of any branch, but the actual target_branch and source_branch values from the payload differ,
//     it results in failure. Therefore, if users provide branch information as "refs/heads/*",
//     the data map is updated accordingly as it indicates a positive scenario.
//     For example:
//
//  1. pipelinesascode.tekton.dev/on-cel-expression: |
//     ( event == "push" && target_branch == "refs/heads/*" && source_branch == "refs/heads/*" ) && "frontend/***".pathChanged()
//
//  2. pipelinesascode.tekton.dev/on-cel-expression: |
//     event == "pull_request" && target_branch == "refs/heads/*" && source_branch == "refs/heads/*" && "frontend/***".pathChanged()

func handleTargetAndSourceBranches(expr string, event *info.Event, data map[string]interface{}) {
	splitFunc := func(c rune) bool {
		return c == '&' || c == '(' || c == ')'
	}
	splittedValues := strings.FieldsFunc(expr, splitFunc)
	for i := range splittedValues {
		if strings.Contains(splittedValues[i], "target_branch") {
			handleBranchCondition(event, data, splittedValues[i], "target_branch", event.BaseBranch)
		}
		if strings.Contains(splittedValues[i], "source_branch") {
			handleBranchCondition(event, data, splittedValues[i], "source_branch", event.HeadBranch)
		}
	}
}

func celEvaluate(ctx context.Context, expr string, event *info.Event, vcx provider.Interface) (ref.Val, error) {
	eventTitle := event.PullRequestTitle
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

	r := regexp.MustCompile(changedFilesTags)
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
		"target_branch": event.BaseBranch,
		"source_branch": event.HeadBranch,
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
	handleTargetAndSourceBranches(expr, event, data)
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

	return evaluateCELExpression(env, expr, data)
}

func evaluateCELExpression(env *cel.Env, expr string, data map[string]interface{}) (ref.Val, error) {
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
