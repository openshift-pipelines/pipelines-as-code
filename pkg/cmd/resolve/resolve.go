package resolve

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/flags"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/pipelineascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/resolve"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

func Command(p cli.Params) *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "resolve",
		Short: "Resolve a bunch of yaml file in a single PipelineRun",
		RunE: func(cmd *cobra.Command, args []string) error {
			cs, err := p.Clients()
			if err != nil {
				return err
			}
			bytes, _ := ioutil.ReadAll(os.Stdin)
			allTemplates := string(bytes)
			// TODO: flags
			allTemplates = pipelineascode.ReplacePlaceHoldersVariables(allTemplates, map[string]string{
				"revision": "SHA",
				"repo_url": "url",
			})
			prun, err := resolve.Resolve(cs, allTemplates, true)
			if err != nil {
				return err
			}
			// TODO multiples?
			d, err := yaml.Marshal(prun[0])
			if err != nil {
				return err
			}

			fmt.Printf("---\n%s\n", d)
			return nil
		},
	}
	flags.AddPacOptions(cmd)

	return cmd
}
