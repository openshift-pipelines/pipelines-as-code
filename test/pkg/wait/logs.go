package wait

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	tlogs "github.com/openshift-pipelines/pipelines-as-code/test/pkg/logs"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
)

func RegexpMatchingInControllerLog(ctx context.Context, clients *params.Run, reg regexp.Regexp, maxNumberOfLoop int, controllerName string) error {
	labelselector := fmt.Sprintf("app.kubernetes.io/name=%s", controllerName)
	containerName := "pac-controller"
	ns := info.GetNS(ctx)
	clients.Clients.Log.Infof("looking for regexp %s in %s for label %s container %s", reg.String(), ns, labelselector, containerName)
	for i := 0; i <= maxNumberOfLoop; i++ {
		output, err := tlogs.GetPodLog(ctx, clients.Clients.Kube.CoreV1(), info.GetNS(ctx), labelselector, containerName)
		if err != nil {
			return err
		}

		if reg.MatchString(output) {
			clients.Clients.Log.Infof("matched regexp %s in %s:%s labelSelector/pod %s for regexp: %s", reg.String(), labelselector, containerName)
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("could not find a match using the labelSelector: %s in container %s for regexp: %s", labelselector, containerName, reg.String())
}

func RegexpMatchingInPodLog(ctx context.Context, clients *params.Run, ns, labelselector, containerName string, reg regexp.Regexp, maxNumberOfLoop int) error {
	var err error
	output := ""
	clients.Clients.Log.Infof("looking for regexp %s in %s:%s labelSelector/pod", reg.String(), labelselector, containerName)
	for i := 0; i <= maxNumberOfLoop; i++ {
		output, err = tlogs.GetPodLog(ctx, clients.Clients.Kube.CoreV1(), ns, labelselector, containerName)
		if err != nil {
			return err
		}

		if reg.MatchString(output) {
			clients.Clients.Log.Infof("matched regexp in labelSelector/container %s:%s",
				labelselector, containerName)
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("could not find a match in %s:%s labelSelector/pod for regexp: '%s' output: '%s'",
		labelselector, containerName, reg.String(), output)
}

// GoldenPodLog is a helper function to get the logs of a pod and compare it to a golden file.
func GoldenPodLog(ctx context.Context, t *testing.T, clients *params.Run, ns, labelselector, containerName, goldenFile string, maxNumberOfLoop int) {
	var err error
	for i := 0; i <= maxNumberOfLoop; i++ {
		var output string
		output, err = tlogs.GetPodLog(ctx, clients.Clients.Kube.CoreV1(), ns, labelselector, containerName)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}
		// Note(chmouel) This is one of the weirdest bug i have seen,
		// only my laptop, the getpodlog outputs things like that
		// files.all: [".tekton/pullrequest.yaml",".tekton/push.yaml","deleted.txt","modified.txt","renamed.txt"]
		// and on CI it has a space
		// files.all: [".tekton/pullrequest.yaml", ".tekton/push.yaml", "deleted.txt", "modified.txt", "renamed.txt"]
		// so we make the output consistent so i can run from my laptop which has probably everything newer than CI
		// anyway this is weird but got the e2e tests working on CI and local dev
		// but something to look out for in the future
		output = strings.ReplaceAll(output, `","`, `", "`)
		golden.Assert(t, output, goldenFile)
	}
	assert.NilError(t, err)
}
