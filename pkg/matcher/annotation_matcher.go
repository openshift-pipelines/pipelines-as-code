package matcher

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gobwas/glob"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

const (
	onEventAnnotation        = "on-event"
	onTargetBranchAnnotation = "on-target-branch"
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

func MatchPipelinerunByAnnotation(ctx context.Context, pruns []*v1beta1.PipelineRun, cs *params.Run) (*v1beta1.PipelineRun, *apipac.Repository, map[string]string, error) {
	configurations := map[string]map[string]string{}
	repo := &apipac.Repository{}
	cs.Clients.Log.Infof("matching a pipeline to event: URL=%s, target-branch=%s, target-event=%s",
		cs.Info.Event.URL,
		cs.Info.Event.BaseBranch,
		cs.Info.Event.TriggerTarget)

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
			repo, _ = MatchEventURLRepo(ctx, cs, targetNS)
			if repo == nil {
				cs.Clients.Log.Warnf("could not find Repository CRD in %s while pipelineRun %s targets it", targetNS, prun.GetGenerateName())
				continue
			}
		}

		if targetEvent, ok := prun.GetObjectMeta().GetAnnotations()[pipelinesascode.
			GroupName+"/"+onEventAnnotation]; ok {
			matched, err := matchOnAnnotation(targetEvent, cs.Info.Event.TriggerTarget, false)
			configurations[prun.GetGenerateName()]["target-event"] = targetEvent
			if err != nil {
				return nil, nil, map[string]string{}, err
			}
			if !matched {
				continue
			}
		}

		if targetBranch, ok := prun.GetObjectMeta().GetAnnotations()[pipelinesascode.
			GroupName+"/"+onTargetBranchAnnotation]; ok {
			matched, err := matchOnAnnotation(targetBranch, cs.Info.Event.BaseBranch, true)
			configurations[prun.GetGenerateName()]["target-branch"] = targetBranch
			if err != nil {
				return nil, nil, map[string]string{}, err
			}
			if !matched {
				continue
			}
		}

		cs.Clients.Log.Infof("matched pipelinerun with name: %s, annotation config: %q", prun.GetGenerateName(),
			configurations[prun.GetGenerateName()])
		return prun, repo, configurations[prun.GetGenerateName()], nil
	}

	cs.Clients.Log.Warn("could not find a match to a pipelinerun in the .tekton/ dir")
	cs.Clients.Log.Warn("available configuration in pipelineRuns annotations")
	for name, maps := range configurations {
		cs.Clients.Log.Infof("pipelineRun: %s, target-branch=%s, target-event=%s",
			name, maps["target-branch"], maps["target-event"])
	}

	// TODO: more descriptive error message
	return nil, nil, map[string]string{}, fmt.Errorf("cannot match pipeline from webhook to pipelineruns on event=%s, branch=%s", cs.Info.Event.EventType, cs.Info.Event.BaseBranch)
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
