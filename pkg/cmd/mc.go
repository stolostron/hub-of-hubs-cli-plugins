/*
Copyright 2018 The Kubernetes Authors.

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
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stolostron/hub-of-hubs-cli-plugins/pkg/cmd/get"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

var managedClustersExample = `
	# view managed clusters
	%[1]s mc get

	# label a managed cluster
	%[1]s mc label mycluster environment=dev
`

// ManagedClustersOptions provides options for ManagedClusters commands
type ManagedClustersOptions struct {
	genericclioptions.IOStreams
	configFlags *genericclioptions.ConfigFlags
}

// NewManagedClustersOptions provides an instance of ManagedClustersOptions with default values
func NewManagedClustersOptions(streams genericclioptions.IOStreams) *ManagedClustersOptions {
	return &ManagedClustersOptions{
		configFlags: genericclioptions.NewConfigFlags(true),
		IOStreams:   streams,
	}
}

// NewCmdManagedClusters provides a cobra command wrapping ManagedClustersOptions
func NewCmdManagedClusters(streams genericclioptions.IOStreams) *cobra.Command {
	o := NewManagedClustersOptions(streams)

	defaultConfigFlags := genericclioptions.NewConfigFlags(true).
		WithDeprecatedPasswordFlag().WithDiscoveryBurst(300).
		WithDiscoveryQPS(50.0)

	cmd := &cobra.Command{
		Use:     "kubectl mc",
		Short:   "Operate managed clusters for Hub of Hubs",
		Example: fmt.Sprintf(managedClustersExample, "kubectl"),
		Run:     runHelp,
	}

	cmd.CompletionOptions.DisableDefaultCmd = true
	flags := cmd.PersistentFlags()

	kubeConfigFlags := o.configFlags
	if kubeConfigFlags == nil {
		kubeConfigFlags = defaultConfigFlags
	}
	kubeConfigFlags.AddFlags(flags)
	matchVersionKubeConfigFlags := cmdutil.NewMatchVersionFlags(kubeConfigFlags)
	matchVersionKubeConfigFlags.AddFlags(flags)

	f := cmdutil.NewFactory(matchVersionKubeConfigFlags)

	cmd.AddCommand(get.NewCmdGet("kubectl", f, o.IOStreams))

	return cmd
}

func runHelp(cmd *cobra.Command, args []string) {
	//nolint:errcheck
	cmd.Help()
}
