package resolve

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/git"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/pipelineascode"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/resolve"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formating"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs/github"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

var (
	filenames      []string
	parameters     []string
	skipInlining   []string
	noGenerateName bool
	remoteTask     bool
)

const longhelp = `

tknresolve - resolve a PipelineRun and all its referenced Pipeline/Tasks embedded.

Resolve the .tekton/pull-request as a single pipelinerun, fetching the remote
tasks according to the annotations in the pipelineRun, apply the parameters
substitutions with -p flags. Output on the standard output the full PipelineRun
resolved.

A simple example that would parse the .tekton/pull-request.yaml with all the
remote task embedded into it applying the parameters substitutions: 

pipelines-as-code resolve \
		-f .tekton/pull-request.yaml \
		-p revision=main \
		-p repo_url=https://github.com/openshift-pipelines/pipelines-as-code

You can specify multiple template files to combine :

pipelines-as-code resolve -f .tekton/pull-request.yaml -f task/referenced.yaml

or a directory where it will get all the files ending by .yaml  :

pipelines-as-code resolve -f .tekton/

*It does not support task from local directory referenced in annotations at the
 moment*.`

func Command(run *params.Run) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve",
		Long:  longhelp,
		Short: "Embed PipelineRun references as a single resource.",
		RunE: func(cmd *cobra.Command, args []string) error {
			err := run.Clients.NewClients(&run.Info)
			if err != nil {
				// this check allows resolve to be run without
				// a kubeconfig so users can verify the tkn version
				noConfigErr := strings.Contains(err.Error(), "Couldn't get kubeConfiguration namespace")
				if !noConfigErr {
					return err
				}
			}
			if len(filenames) == 0 {
				return fmt.Errorf("you need to at least specify a file with -f")
			}

			mapped := splitArgsInMap(parameters)

			// ignore error
			gitinfo := git.GetGitInfo(".")
			if _, ok := mapped["repo_url"]; !ok && gitinfo.URL != "" {
				mapped["repo_url"] = gitinfo.URL
			}

			if _, ok := mapped["revision"]; !ok && gitinfo.SHA != "" {
				mapped["revision"] = gitinfo.SHA
			}

			if _, ok := mapped["repo_owner"]; !ok && gitinfo.URL != "" {
				repoOwner, err := formating.GetRepoOwnerFromGHURL(gitinfo.URL)
				if err != nil {
					return err
				}
				mapped["repo_owner"] = strings.Split(repoOwner, "/")[0]
				mapped["repo_name"] = strings.Split(repoOwner, "/")[1]
			}
			s, err := resolveFilenames(run, filenames, mapped)
			// nolint:forbidigo
			fmt.Println(s)
			return err
		},
	}
	cmd.Flags().StringSliceVarP(&parameters, "params", "p", filenames,
		"Params to resolve (ie: revision, repo_url)")
	cmd.Flags().StringSliceVarP(&filenames, "filename", "f", filenames,
		"Filename, directory, or URL to files to use to create the resource")
	cmd.Flags().StringSliceVarP(&skipInlining, "skip", "s", filenames,
		"Which task to skip inlining")
	cmd.Flags().BoolVar(&noGenerateName, "no-generate-name", false,
		"Don't automatically generate a GenerateName for pipelinerun uniqueness")
	cmd.Flags().BoolVar(&remoteTask, "remoteTask", true,
		"Wether parse annotation to fetch remote task")
	err := run.Info.Pac.AddFlags(cmd)
	if err != nil {
		log.Fatal(err)
	}

	return cmd
}

func splitArgsInMap(args []string) map[string]string {
	m := make(map[string]string)
	for _, e := range args {
		parts := strings.Split(e, "=")
		m[parts[0]] = parts[1]
	}
	return m
}

func resolveFilenames(cs *params.Run, filenames []string, params map[string]string) (string, error) {
	var ret string

	allTemplates := enumerateFiles(filenames)
	// TODO: flags
	allTemplates = pipelineascode.ReplacePlaceHoldersVariables(allTemplates, params)
	ctx := context.Background()
	ropt := &resolve.Opts{
		GenerateName: !noGenerateName,
		RemoteTasks:  remoteTask,
		SkipInlining: skipInlining,
	}
	// We use github here but since we don't do remotetask we would not care
	vcsintf := &github.VCS{}
	prun, err := resolve.Resolve(ctx, cs, vcsintf, allTemplates, ropt)
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
