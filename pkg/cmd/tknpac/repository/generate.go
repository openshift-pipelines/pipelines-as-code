package repository

// const defaultMainBranch = "main"
// func t() {
// 	if opts.event.BaseBranch == "" {
// 		qs = append(qs, &survey.Question{
// 			Name:   "TargetBranch",
// 			Prompt: &survey.Input{Message: "Enter the target GIT branch (default: main): "},
// 		})
// 	}

// 	if opts.event.EventType == "" {
// 		qs = append(qs, &survey.Question{
// 			Name: "EventType",
// 			Prompt: &survey.Select{
// 				Message: "Enter the Git event type for triggering the pipeline: ",
// 				Options: []string{"pull_request", "push"},
// 				Default: "pull_request",
// 			},
// 		})
// 	}
// 	if opts.event.BaseBranch == "" {
// 		opts.event.BaseBranch = defaultMainBranch
// 	}

// }

// askToCreateSimplePipeline will try to create a basic pipeline in tekton
// // directory.
// func askToCreateSimplePipeline(gitRoot string, opts CreateOptions) error {
// 	fpath := filepath.Join(gitRoot, ".tekton", fmt.Sprintf("%s.yaml", opts.event.EventType))
// 	cwd, _ := os.Getwd()
// 	relpath, _ := filepath.Rel(cwd, fpath)

// 	reply, err := AskYesNo(opts,
// 		fmt.Sprintf("Would you like me to create a basic PipelineRun file into the file %s ?", relpath),
// 		"True")
// 	if err != nil {
// 		return err
// 	}

// 	if !reply {
// 		return nil
// 	}

// 	if _, err = os.Stat(filepath.Join(gitRoot, ".tekton")); os.IsNotExist(err) {
// 		if err := os.MkdirAll(filepath.Join(gitRoot, ".tekton"), 0o755); err != nil {
// 			return err
// 		}
// 	}

// 	if _, err = os.Stat(fpath); !os.IsNotExist(err) {
// 		overwrite, err := AskYesNo(opts,
// 			fmt.Sprintf("There is already a file named: %s would you like me to override it?", fpath), "No")
// 		if err != nil {
// 			return err
// 		}
// 		if !overwrite {
// 			return nil
// 		}
// 	}

// 	tmpl := fmt.Sprintf(`---
// apiVersion: tekton.dev/v1beta1
// kind: PipelineRun
// metadata:
//   name: %s
//   annotations:
//     # The event we are targeting (ie: pull_request, push)
//     pipelinesascode.tekton.dev/on-event: "[%s]"

//     # The branch or tag we are targeting (ie: main, refs/tags/*)
//     pipelinesascode.tekton.dev/on-target-branch: "[%s]"

//     # Fetch the git-clone task from hub, we are able to reference it with taskRef
//     pipelinesascode.tekton.dev/task: "[git-clone]"

//     # You can add more tasks in here to reuse, browse the one you like from here
//     # https://hub.tekton.dev/
//     # example:
//     # pipelinesascode.tekton.dev/task-1: "[maven, buildah]"

//     # How many runs we want to keep attached to this event
//     pipelinesascode.tekton.dev/max-keep-runs: "5"
// spec:
//   params:
//     # The variable with brackets are special to Pipelines as Code
//     # They will automatically be expanded with the events from Github.
//     - name: repo_url
//       value: "{{repo_url}}"
//     - name: revision
//       value: "{{revision}}"
//   pipelineSpec:
//     params:
//       - name: repo_url
//       - name: revision
//     workspaces:
//       - name: source
//       - name: basic-auth
//     tasks:
//       - name: fetch-repository
//         taskRef:
//           name: git-clone
//         workspaces:
//           - name: output
//             workspace: source
//           - name: basic-auth
//             workspace: basic-auth
//         params:
//           - name: url
//             value: $(params.repo_url)
//           - name: revision
//             value: $(params.revision)
//       # Customize this task if you like, or just do a taskRef
//       # to one of the hub task.
//       - name: noop-task
//         runAfter:
//           - fetch-repository
//         workspaces:
//           - name: source
//             workspace: source
//         taskSpec:
//           workspaces:
//             - name: source
//           steps:
//             - name: noop-task
//               image: registry.access.redhat.com/ubi8/ubi-micro:8.4
//               workingDir: $(workspaces.source.path)
//               script: |
//                 exit 0
//   workspaces:
//   - name: source
//     volumeClaimTemplate:
//       spec:
//         accessModes:
//           - ReadWriteOnce
//         resources:
//           requests:
//             storage: 1Gi
//   # This workspace will inject secret to help the git-clone task to be able to
//   # checkout the private repositories
//   - name: basic-auth
//     secret:
//       secretName: "pac-git-basic-auth-{{repo_owner}}-{{repo_name}}"
//       `, opts.repository.Name, opts.event.EventType, opts.event.BaseBranch)
// 	// nolint: gosec
// 	err = ioutil.WriteFile(fpath, []byte(tmpl), 0o644)
// 	if err != nil {
// 		return err
// 	}

// 	cs := opts.IOStreams.ColorScheme()
// 	fmt.Fprintf(opts.IOStreams.Out, "%s A basic template has been created in %s, feel free to customize it.\n",
// 		cs.SuccessIcon(),
// 		cs.Bold(fpath),
// 	)
// 	fmt.Fprintf(opts.IOStreams.Out, "%s You can test your pipeline manually with: ", cs.InfoIcon())
// 	fmt.Fprintf(opts.IOStreams.Out, "tkn-pac resolve -f %s | kubectl create -f-\n", relpath)

// 	return nil
// }

// // create ...
// func create(ctx context.Context, gitdir string, opts CreateOptions) error {
// 	var qs []*survey.Question
// 	var err error

// 	gitinfo := git.GetGitInfo(gitdir)

// 	if opts.AssumeYes && opts.repository.GetNamespace() == "" {
// 		opts.repository.Namespace = opts.CurrentNS
// 	}
// 	if opts.AssumeYes && opts.event.URL == "" {
// 		opts.event.URL = gitinfo.TargetURL
// 	}
// 	if opts.AssumeYes && opts.event.BaseBranch == "" {
// 		opts.event.BaseBranch = defaultMainBranch
// 	}
// 	if opts.AssumeYes && opts.event.EventType == "" {
// 		opts.event.EventType = "pull_request"
// 	}

// 	if opts.repository.GetNamespace() == "" {
// 		qs = append(qs, &survey.Question{
// 			Name:   "Namespace",
// 			Prompt: &survey.Input{Message: fmt.Sprintf("Enter the namespace where the pipeline should run (default: %s): ", opts.CurrentNS)},
// 		})
// 	}
// 	if opts.event.URL == "" {
// 		prompt := "Enter the target url: "
// 		if gitinfo.TargetURL != "" {
// 			prompt = fmt.Sprintf("Enter the Git repository url containing the pipelines (default: %s): ", gitinfo.TargetURL)
// 		}
// 		qs = append(qs, &survey.Question{
// 			Name:   "TargetURL",
// 			Prompt: &survey.Input{Message: prompt},
// 		})
// 	}

// 	if qs != nil {
// 		err := opts.CLIOpts.Ask(qs, &opts)
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	if opts.repository.GetNamespace() == "" {
// 		opts.repository.Namespace = opts.CurrentNS
// 	}

// 	if opts.repository.GetNamespace() == "" {
// 		opts.repository.Namespace, err = askNameForResource(opts, "Enter the repository name")
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	if opts.event.URL == "" && gitinfo.TargetURL != "" {
// 		opts.event.URL = gitinfo.TargetURL
// 	} else if opts.event.URL == "" {
// 		return fmt.Errorf("we didn't get a target URL")
// 	}

// 	cs := opts.IOStreams.ColorScheme()
// 	if opts.repository.GetNamespace() != opts.CurrentNS {
// 		if err := askCreateNamespace(ctx, opts, cs); err != nil {
// 			return err
// 		}
// 	}
// 	opts.repository.Spec = apipac.RepositorySpec{
// 		URL: opts.event.URL,
// 	}

// 	_, err = opts.Run.Clients.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(opts.repository.GetNamespace()).Create(ctx,
// 		opts.repository,
// 		metav1.CreateOptions{})
// 	if err != nil {
// 		return err
// 	}
// 	fmt.Fprintf(opts.IOStreams.Out, "%s Repository %s has been created in %s namespace\n",
// 		cs.SuccessIconWithColor(cs.Green),
// 		opts.repository.GetName(),
// 		opts.repository.GetNamespace(),
// 	)

// 	if err := askToCreateSimplePipeline(gitinfo.TopLevelPath, opts); err != nil {
// 		return err
// 	}

// 	fmt.Fprintf(opts.IOStreams.Out, "%s Don't forget to install the GitHub application into your repo %s\n",
// 		cs.InfoIcon(),
// 		opts.event.URL,
// 	)
// 	fmt.Fprintf(opts.IOStreams.Out, "%s and we are done! enjoy :)))\n", cs.SuccessIcon())

// 	return nil
// }

// func askNameForResource(opts CreateOptions, question string) (string, error) {
// 	s, err := ui.GetRepoOwnerFromGHURL(opts.event.URL)
// 	repo := fmt.Sprintf("%s-%s", filepath.Base(s), strings.ReplaceAll(opts.event.EventType, "_", "-"))
// 	// Don't ask question if we auto generated
// 	if opts.AssumeYes {
// 		return repo, nil
// 	}

// 	if err == nil {
// 		// No assume yes but generated a name properly so let's return that
// 		return repo, nil
// 	}

// 	repo = ""
// 	err = opts.CLIOpts.Ask([]*survey.Question{
// 		{
// 			Prompt: &survey.Input{Message: question},
// 		},
// 	}, &repo)
// 	if err != nil {
// 		return "", err
// 	}
// 	if repo == "" {
// 		return "", fmt.Errorf("no name has been set")
// 	}
// 	return repo, nil
// }
