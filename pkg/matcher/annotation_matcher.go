package matcher

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gobwas/glob"
	"github.com/google/cel-go/common/types"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

const (
	onEventAnnotation        = "on-event"
	onTargetBranchAnnotation = "on-target-branch"
	onCelExpression          = "on-cel-expression"
	onTargetNamespace        = "target-namespace"
	maxKeepRuns              = "max-keep-runs"

	// regex allows array of string or a single string
	// eg. ["foo", "bar"], ["foo"] or "foo"
	reValidateTag = `^\[(.*)\]$|^[^[\]\s]*$`
)

func branchMatch(prunBranch, baseBranch string) bool {
	// If we have targetBranch in annotation and refs/heads/targetBranch from
	// webhook, then allow it.
	if filepath.Base(baseBranch) == prunBranch {
		return true
	}

	// match globs like refs/tags/0.*
	g := glob.MustCompile(prunBranch)
	return g.Match(baseBranch)
}

// TODO: move to another file since it's common to all annotations_* files
func getAnnotationValues(annotation string) ([]string, error) {
	re := regexp.MustCompile(reValidateTag)
	annotation = strings.TrimSpace(annotation)
	match := re.Match([]byte(annotation))
	if !match {
		return nil, errors.New("annotations in pipeline are in wrong format")
	}

	// if it's not an array then it would be a single string
	if !strings.HasPrefix(annotation, "[") {
		return []string{annotation}, nil
	}

	// Split all tasks by comma and make sure to trim spaces in there
	splitted := strings.Split(re.FindStringSubmatch(annotation)[1], ",")
	for i := range splitted {
		splitted[i] = strings.TrimSpace(splitted[i])
	}

	if splitted[0] == "" {
		return nil, errors.New("annotations in pipeline are empty")
	}

	return splitted, nil
}

func getTargetBranch(prun *v1beta1.PipelineRun, cs *params.Run, event *info.Event) (bool, string, string, error) {
	var targetEvent, targetBranch string
	if key, ok := prun.GetObjectMeta().GetAnnotations()[filepath.Join(
		pipelinesascode.GroupName, onEventAnnotation)]; ok {
		matched, err := matchOnAnnotation(key, event.TriggerTarget, false)
		targetEvent = key
		if err != nil {
			return false, "", "", err
		}
		if !matched {
			return false, "", "", nil
		}
	}
	if key, ok := prun.GetObjectMeta().GetAnnotations()[filepath.Join(
		pipelinesascode.GroupName, onTargetBranchAnnotation)]; ok {
		matched, err := matchOnAnnotation(key, event.BaseBranch, true)
		targetBranch = key
		if err != nil {
			return false, "", "", err
		}
		if !matched {
			return false, "", "", nil
		}
	}

	if targetEvent == "" || targetBranch == "" {
		cs.Clients.Log.Infof("skipping pipelinerun %s, no on-target-event or on-target-branch has been set in pipelinerun", prun.GetGenerateName())
		return false, "", "", nil
	}
	return true, targetEvent, targetBranch, nil
}

func MatchPipelinerunByAnnotation(ctx context.Context, pruns []*v1beta1.PipelineRun, cs *params.Run, event *info.Event) (*v1beta1.PipelineRun, *apipac.Repository, map[string]string, error) {
	configurations := map[string]map[string]string{}
	repo := &apipac.Repository{}
	cs.Clients.Log.Infof("matching a pipeline to event: URL=%s, target-branch=%s, source-branch=%s, target-event=%s",
		event.URL,
		event.BaseBranch,
		event.HeadBranch,
		event.TriggerTarget)

	for _, prun := range pruns {
		configurations[prun.GetGenerateName()] = map[string]string{}
		if prun.GetObjectMeta().GetAnnotations() == nil {
			cs.Clients.Log.Warnf("PipelineRun %s does not have any annotations", prun.GetName())
			continue
		}

		if maxPrNumber, ok := prun.GetObjectMeta().GetAnnotations()[pipelinesascode.
			GroupName+"/"+maxKeepRuns]; ok {
			configurations[prun.GetGenerateName()]["max-keep-runs"] = maxPrNumber
		}

		if targetNS, ok := prun.GetObjectMeta().GetAnnotations()[pipelinesascode.
			GroupName+"/"+onTargetNamespace]; ok {
			configurations[prun.GetGenerateName()]["target-namespace"] = targetNS
			repo, _ = MatchEventURLRepo(ctx, cs, event, targetNS)
			if repo == nil {
				cs.Clients.Log.Warnf("could not find Repository CRD in %s while pipelineRun %s targets it", targetNS, prun.GetGenerateName())
				continue
			}
		}

		// nolint: nestif
		if celExpr, ok := prun.GetObjectMeta().GetAnnotations()[filepath.Join(pipelinesascode.GroupName, onCelExpression)]; ok {
			out, err := celEvaluate(celExpr, event)
			if err != nil {
				return nil, nil, map[string]string{}, err
			}
			if out != types.True {
				cs.Clients.Log.Infof("CEL expression is not matching %s, skipping", prun.GetGenerateName())
				continue
			}
			cs.Clients.Log.Infof("CEL expression has been evaluated and matched")
		} else {
			matched, targetEvent, targetBranch, err := getTargetBranch(prun, cs, event)
			if err != nil {
				return nil, nil, map[string]string{}, err
			}
			if !matched {
				continue
			}
			configurations[prun.GetGenerateName()]["target-branch"] = targetBranch
			configurations[prun.GetGenerateName()]["target-event"] = targetEvent
		}

		cs.Clients.Log.Infof("matched pipelinerun with name: %s, annotation config: %q", prun.GetGenerateName(),
			configurations[prun.GetGenerateName()])
		return prun, repo, configurations[prun.GetGenerateName()], nil
	}

	cs.Clients.Log.Warn("could not find a match to a pipelinerun matching payload")
	cs.Clients.Log.Warn("available configuration in pipelineRuns annotations")
	for name, maps := range configurations {
		cs.Clients.Log.Infof("pipelineRun: %s, target-branch=%s, target-event=%s",
			name, maps["target-branch"], maps["target-event"])
	}

	// TODO: more descriptive error message
	return nil, nil, map[string]string{}, fmt.Errorf("cannot match pipeline from webhook to pipelineruns on event=%s, branch=%s", event.EventType, event.BaseBranch)
}

func matchOnAnnotation(annotations string, eventType string, branchMatching bool) (bool, error) {
	targets, err := getAnnotationValues(annotations)
	if err != nil {
		return false, err
	}

	var gotit string
	for _, v := range targets {
		if v == eventType {
			gotit = v
		}
		if branchMatching && branchMatch(v, eventType) {
			gotit = v
		}
	}
	if gotit == "" {
		return false, nil
	}
	return true, nil
}
