package resolve

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/formatting"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/git"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/info"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/provider/github"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/resolve"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/templates"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"sigs.k8s.io/yaml"
)

var (
	filenames      []string
	parameters     []string
	skipInlining   []string
	noGenerateName bool
	remoteTask     bool
	noSecret       bool
	providerToken  string
	output         string
)

var longhelp = fmt.Sprintf(`

resolve - resolve a PipelineRun and all its referenced Pipeline/Tasks embedded.

Resolve the .tekton/pull-request as a single pipelinerun, fetching the remote
tasks according to the annotations in the pipelineRun, apply the parameters
substitutions with -p flags. Output on the standard output or to a file with the
-o flag with the complete PipelineRun resolved.

A simple example that would parse the .tekton/pull-request.yaml with all the
remote task embedded into it applying the parameters substitutions:

%s pac resolve \
		-f .tekton/pull-request.yaml -o output-file.yaml \
		-p revision=main -p repo_url=https://repo_url/

You can specify multiple template files to combine :

%s pac resolve -f .tekton/pull-request.yaml -f task/referenced.yaml

or a directory where it will get all the files ending by .yaml  :

%s pac resolve -f .tekton/

If it detect a {{ git_auth_secret }} in the template it will ask you if you want
to provide a token. You can set the environment variable PAC_PROVIDER_TOKEN to
not have to ask about it.

*It does not support task from local directory referenced in annotations at the
 moment*.`, settings.TknBinaryName, settings.TknBinaryName, settings.TknBinaryName)

func Command(run *params.Run, streams *cli.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve",
		Long:  longhelp,
		Short: "Resolve PipelineRun the same way its run on CI",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			errc := run.Clients.NewClients(ctx, &run.Info)

			// only report error here on CLI
			zaplog, err := zap.NewProduction(
				zap.IncreaseLevel(zap.FatalLevel),
			)
			if err != nil {
				return err
			}
			run.Clients.Log = zaplog.Sugar()

			if errc != nil {
				// this check allows resolve to be run without
				// a kubeconfig so users can verify the tkn version
				noConfigErr := strings.Contains(errc.Error(), "Couldn't get kubeConfiguration namespace")
				if !noConfigErr {
					return errc
				}
			} else {
				// it's OK  if pac is not installed, ignore the error
				_ = run.UpdatePACInfo(ctx)
			}

			if len(filenames) == 0 {
				return fmt.Errorf("you need to at least specify a file with -f")
			}

			if err := settings.ConfigToSettings(run.Clients.Log, run.Info.Pac.Settings, map[string]string{}); err != nil {
				return err
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
				repoOwner, err := formatting.GetRepoOwnerFromURL(gitinfo.URL)
				if err != nil {
					return err
				}
				mapped["repo_owner"] = strings.Split(repoOwner, "/")[0]
				mapped["repo_name"] = strings.Split(repoOwner, "/")[1]
			}

			s, err := resolveFilenames(ctx, run, filenames, mapped)
			if err != nil {
				return err
			}

			if output != "" {
				fmt.Fprintf(streams.Out, "PipelineRun has been written to %s\n", output)
				return os.WriteFile(output, []byte(s), 0o600)
			}

			fmt.Fprintln(streams.Out, s)
			return nil
		},
		Annotations: map[string]string{
			"commandType": "main",
		},
	}
	cmd.Flags().StringSliceVarP(&parameters, "params", "p", filenames,
		"Params to resolve (ie: revision, repo_url)")

	cmd.Flags().StringVarP(&output, "output", "o", "",
		"Params to resolve (ie: revision, repo_url)")

	cmd.Flags().StringSliceVarP(&filenames, "filename", "f", filenames,
		"Filename, directory, or URL to files to use to create the resource")

	cmd.Flags().StringSliceVarP(&skipInlining, "skip", "s", filenames,
		"skip inlining")

	cmd.Flags().BoolVar(&noSecret, "no-secret", false,
		"skip generating or asking for secrets")

	cmd.Flags().BoolVar(&noGenerateName, "no-generate-name", false,
		"don't automatically generate a GenerateName for pipelinerun uniqueness")

	cmd.Flags().BoolVar(&remoteTask, "remoteTask", true,
		"set this to false to avoid fetching and embed remote tasks")

	cmd.Flags().StringVarP(&providerToken, "providerToken", "t", "", "use this token to generate the git-auth secret,\n you can set the environment PAC_PROVIDER_TOKEN to have this set automatically")
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

func resolveFilenames(ctx context.Context, cs *params.Run, filenames []string, params map[string]string) (string, error) {
	var ret string

	ropt := &resolve.Opts{
		GenerateName:  !noGenerateName,
		RemoteTasks:   remoteTask,
		SkipInlining:  skipInlining,
		ProviderToken: providerToken,
	}
	allTemplates := enumerateFiles(filenames)
	if !noSecret {
		outSecret, secretName, err := makeGitAuthSecret(ctx, cs, filenames, ropt.ProviderToken, params)
		if err != nil {
			return "", err
		}
		if secretName != "" {
			params["git_auth_secret"] = secretName
		}
		ret += outSecret
	}

	// TODO: flags
	allTemplates = templates.ReplacePlaceHoldersVariables(allTemplates, params)
	// We use github here but since we don't do remotetask we would not care
	providerintf := github.New()
	event := info.NewEvent()
	prun, err := resolve.Resolve(ctx, cs, cs.Clients.Log, providerintf, event, allTemplates, ropt)
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
	b, err := os.ReadFile(filename)
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
