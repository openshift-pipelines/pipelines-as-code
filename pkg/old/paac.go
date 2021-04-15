package paac

import (
	"fmt"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
)

const (
	TEKTON_DIRECTORY = ".tekton"
)

func PipelineAsCode(token, payload string) error {
	webvcs := webvcs.NewGithubVCS(token, payload)
	runinfo, err := webvcs.ParsePayload(payload)
	if err != nil {
		return err
	}
	repoContent, err := webvcs.GetTektonDir(TEKTON_DIRECTORY, runinfo)
	if err != nil {
		return err
	}
	fmt.Println(repoContent)
	// for _, object := range repoContent {
	//	blob, err := webvcs.GetObject(object.GetSHA(), runinfo)
	//	if err != nil {
	//		fmt.Println("Could not find: " + object.GetName()) // Should not happen
	//		continue
	//	}
	// }
	return nil
}
