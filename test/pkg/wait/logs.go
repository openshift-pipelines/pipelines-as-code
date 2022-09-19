package wait

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	tlogs "github.com/openshift-pipelines/pipelines-as-code/test/pkg/logs"
)

func RegexpMatchingInPodLog(ctx context.Context, clients *params.Run, labelselector, containerName string, reg regexp.Regexp, maxNumberOfLoop int) error {
	for i := 0; i <= maxNumberOfLoop; i++ {
		output, err := tlogs.GetPodLog(ctx, clients.Clients.Kube.CoreV1(), labelselector, containerName)
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
