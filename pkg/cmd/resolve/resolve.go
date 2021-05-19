package resolve

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/flags"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/pipelineascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/resolve"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

var (
	filenames    []string
	skipInlining []string
	generateName bool
	remoteTask   bool
)

func Command(p cli.Params) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve",
		Short: "Resolve a bunch of yaml file in a single PipelineRun",
		RunE: func(cmd *cobra.Command, args []string) error {
			cs, err := p.Clients()
			if err != nil {
				return err
			}
			s, err := resolveFilenames(cs, filenames)
			fmt.Println(s)
			return err
		},
	}
	cmd.Flags().StringSliceVarP(&filenames, "filename", "f", filenames,
		"Filename, directory, or URL to files to use to create the resource")
	cmd.Flags().StringSliceVarP(&skipInlining, "skip", "s", filenames,
		"Which task to skip inlining")
	cmd.Flags().BoolVar(&generateName, "generateName", false,
		"Wether to switch name to a generateName on pipelinerun")
	cmd.Flags().BoolVar(&remoteTask, "remoteTask", true,
		"Wether parse annotation to fetch remote task")
	flags.AddPacOptions(cmd)

	return cmd
}

func resolveFilenames(cs *cli.Clients, filenames []string) (string, error) {
	var ret string

	allTemplates := enumerateFiles(filenames)
	// TODO: flags
	allTemplates = pipelineascode.ReplacePlaceHoldersVariables(allTemplates, map[string]string{
		"revision": "SHA",
		"repo_url": "url",
	})
	ctx := context.Background()
	runinfo := &webvcs.RunInfo{}
	ropt := &resolve.Opts{
		GenerateName: generateName,
		RemoteTasks:  remoteTask,
		SkipInlining: skipInlining,
	}
	prun, err := resolve.Resolve(ctx, cs, runinfo, allTemplates, ropt)
	if err != nil {
		return "", err
	}

	for _, run := range prun {
		d, err := yaml.Marshal(run)
		if err != nil {
			return "", err
		}
		ret += fmt.Sprintf("---\n%s\n", d)
	}
	return ret, nil
}

func appendYaml(filename string) string {
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatal(err)
	}
	s := string(b)
	if strings.HasPrefix(s, "---") {
		return s
	}
	return fmt.Sprintf("---\n%s", s)
}

func enumerateFiles(filenames []string) string {
	var yamlDoc string
	for _, paths := range filenames {
		if stat, err := os.Stat(paths); err == nil && !stat.IsDir() {
			yamlDoc += appendYaml(paths)
			continue
		}

		// walk dir getting all yamls
		err := filepath.Walk(paths, func(path string, fi os.FileInfo, err error) error {
			if filepath.Ext(path) == ".yaml" {
				yamlDoc += appendYaml(path)
			}
			return nil
		})
		if err != nil {
			log.Fatalf("Error enumerating files: %v", err)
		}
	}

	return yamlDoc
}
