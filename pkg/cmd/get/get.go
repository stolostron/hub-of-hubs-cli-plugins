// Copyright (c) 2022 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package get

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/cmd/get"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
)

// NewCmdGet creates a command object for the generic "get" action, which
// retrieves one or more resources from a server.
func NewCmdGet(parent string, f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := get.NewCmdGet(parent, f, streams)

	cmd.Use = fmt.Sprintf("get [(-o|--output=)%s] [NAME | -l label] [flags]",
		strings.Join(get.NewGetPrintFlags().AllowedFormats(), "|"))

	cmd.Short = "Display one or many managed clusters"

	cmd.Long = `
		Display one or many resources.
		Prints a table of the most important information about the specified managed clusters.
		You can filter the list using a label selector and the --selector flag.
		By specifying the output as 'template' and providing a Go template as the value
		of the --template flag, you can filter the attributes of the fetched managed clusters.`

	cmd.Example = `
		# List all managed clusters in ps output format
		kubectl mc get

		# List a single managed cluster with specified NAME in ps output format
		kubectl mc get mycluster

		# List a single managed cluster in JSON output format
		kubectl mc get -o json mycluster
	`

	return cmd
}
