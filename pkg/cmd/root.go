/*
Copyright © 2021 Red Hat

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"github.com/openshift-pipelines/pipelines-as-code/pkg/paac"
	"github.com/spf13/cobra"
)

var payload, token string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "pipelines-as-code",
	Short: "Pipelines as code CLI",
	Run: func(cmd *cobra.Command, args []string) {
		paac.PipelineAsCode(token, payload)
	},
}

func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	rootCmd.PersistentFlags().StringVar(&token, "token", "", "Token used to interact with API")
	rootCmd.PersistentFlags().StringVar(&payload, "payload", "", "Webhook payload")
}
