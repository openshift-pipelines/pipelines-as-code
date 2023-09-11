package wait

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	tlogs "github.com/openshift-pipelines/pipelines-as-code/test/pkg/logs"
)

func RegexpMatchingInControllerLog(ctx context.Context, clients *params.Run, reg regexp.Regexp, maxNumberOfLoop int) error {
	labelselector := "app.kubernetes.io/component=controller"
	containerName := "pac-controller"
	for i := 0; i <= maxNumberOfLoop; i++ {
		output, err := tlogs.GetControllerLog(ctx, clients.Clients.Kube.CoreV1(), labelselector, containerName)
		if err != nil {
			return err
		}

		if reg.MatchString(output) {
			clients.Clients.Log.Infof("matched regexp %s in %s:%s labelSelector/pod for regexp: %s", reg.String(), labelselector, containerName)
			return nil
		}
		time.Sleep(5 * time.Second)
	}
	return fmt.Errorf("could not find a match in %s:%s labelSelector/pod for regexp: %s", labelselector, containerName, reg.String())
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
