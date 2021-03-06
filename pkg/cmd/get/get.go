// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

// adopted from https://github.com/kubernetes/kubectl/blob/master/pkg/cmd/get/get.go

/*
Copyright 2014 The Kubernetes Authors.

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

//nolint
package get

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/spf13/cobra"
	pluginutil "github.com/stolostron/hub-of-hubs-cli-plugins/pkg/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1beta1 "k8s.io/apimachinery/pkg/apis/meta/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	watchtools "k8s.io/client-go/tools/watch"
	kubectlget "k8s.io/kubectl/pkg/cmd/get"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/interrupt"
	"k8s.io/kubectl/pkg/util/templates"
	utilpointer "k8s.io/utils/pointer"
)

// Options contains the input to the get command.
type Options struct {
	PrintFlags             *kubectlget.PrintFlags
	ToPrinter              func(*meta.RESTMapping, *bool, bool, bool) (printers.ResourcePrinterFunc, error)
	IsHumanReadablePrinter bool
	PrintWithOpenAPICols   bool

	CmdParent string

	resource.FilenameOptions

	Watch     bool
	ChunkSize int64

	OutputWatchEvents bool

	LabelSelector string
	FieldSelector string

	ServerPrint bool

	NoHeaders      bool
	Sort           bool
	IgnoreNotFound bool

	genericclioptions.IOStreams
	configFlags *genericclioptions.ConfigFlags

	nonk8sAPIURL string
	token        string
	mapping      *meta.RESTMapping
	resourcePath string
}

var (
	getLong = templates.LongDesc(i18n.T(`
		Display one or many managed clusters.

		Prints a table of the most important information about the specified managed clusters.
		You can filter the list using a label selector and the --selector flag.

		By specifying the output as 'template' and providing a Go template as the value
		of the --template flag, you can filter the attributes of the fetched managed clusters.`))

	getExample = templates.Examples(i18n.T(`
		# List all managed clusters in ps output format
		kubectl-mc get

		# List a single managed cluster with specified NAME in ps output format
		kubectl-mc get mycluster

		# List a single managed cluster in JSON output format
		kubectl-mc get -o json mycluster`))

	errStatusNotOK = errors.New("response status not HTTP OK")
)

const (
	useOpenAPIPrintColumnFlagLabel = "use-openapi-print-columns"
	useServerPrintColumns          = "server-print"
)

// NewOptions returns a Options with default chunk size 500.
func NewOptions(parent string, configFlags *genericclioptions.ConfigFlags,
	streams genericclioptions.IOStreams, mapping *meta.RESTMapping,
	resourcePath string) *Options {
	return &Options{
		PrintFlags: kubectlget.NewGetPrintFlags(),
		CmdParent:  parent,

		configFlags:  configFlags,
		IOStreams:    streams,
		ChunkSize:    cmdutil.DefaultChunkSize,
		ServerPrint:  true,
		mapping:      mapping,
		resourcePath: resourcePath,
	}
}

// NewCmd creates a command object for the generic "get" action, which
// retrieves one or more resources from a server.
func NewCmd(parent string, f cmdutil.Factory, configFlags *genericclioptions.ConfigFlags,
	streams genericclioptions.IOStreams, mapping *meta.RESTMapping, resourcePath, resourceNamePlural string) *cobra.Command {
	o := NewOptions(parent, configFlags, streams, mapping, resourcePath)

	cmd := &cobra.Command{
		Use: fmt.Sprintf("get [(-o|--output=)%s] [NAME | -l label] [flags]",
			strings.Join(o.PrintFlags.AllowedFormats(), "|")),
		DisableFlagsInUseLine: true,
		Short:                 i18n.T("Display one or many " + resourceNamePlural),
		Long:                  getLong,
		Example:               getExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Validate(cmd, args))
			cmdutil.CheckErr(o.Run(cmd, args))
		},
		SuggestFor: []string{"list", "ps"},
	}

	o.PrintFlags.AddFlags(cmd)

	cmd.Flags().BoolVarP(&o.Watch, "watch", "w", o.Watch, "After listing/getting the requested object, watch for changes.")
	cmd.Flags().BoolVar(&o.OutputWatchEvents, "output-watch-events", o.OutputWatchEvents, "Output watch event objects when --watch is used. Existing objects are output as initial ADDED events.")
	cmd.Flags().BoolVar(&o.IgnoreNotFound, "ignore-not-found", o.IgnoreNotFound, "If the requested object does not exist the command will return exit code 0.")
	cmd.Flags().StringVar(&o.FieldSelector, "field-selector", o.FieldSelector, "Selector (field query) to filter on, supports '=', '==', and '!='.(e.g. --field-selector key1=value1,key2=value2). The server only supports a limited number of field queries per type.")
	addOpenAPIPrintColumnFlags(cmd, o)
	addServerPrintColumnFlags(cmd, o)
	cmdutil.AddChunkSizeFlag(cmd, &o.ChunkSize)

	// TODO replace with cmdutil.AddLabelSelectorFlagVar() in a latest version
	cmd.Flags().StringVarP(&o.LabelSelector, "selector", "l", o.LabelSelector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	return cmd
}

// Complete takes the command arguments and factory and infers any remaining options.
func (o *Options) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	var err error

	sortBy, err := cmd.Flags().GetString("sort-by")
	if err != nil {
		return err
	}
	o.Sort = len(sortBy) > 0

	o.NoHeaders = cmdutil.GetFlagBool(cmd, "no-headers")

	// TODO (soltysh): currently we don't support custom columns
	// with server side print. So in these cases force the old behavior.
	outputOption := cmd.Flags().Lookup("output").Value.String()
	if strings.Contains(outputOption, "custom-columns") || outputOption == "yaml" || strings.Contains(outputOption, "json") {
		o.ServerPrint = false
	}

	templateArg := ""
	if o.PrintFlags.TemplateFlags != nil && o.PrintFlags.TemplateFlags.TemplateArgument != nil {
		templateArg = *o.PrintFlags.TemplateFlags.TemplateArgument
	}

	// human readable printers have special conversion rules, so we determine if we're using one.
	if (len(*o.PrintFlags.OutputFormat) == 0 && len(templateArg) == 0) || *o.PrintFlags.OutputFormat == "wide" {
		o.IsHumanReadablePrinter = true
	}

	o.ToPrinter = func(mapping *meta.RESTMapping, outputObjects *bool, withNamespace bool, withKind bool) (printers.ResourcePrinterFunc, error) {
		// make a new copy of current flags / opts before mutating
		printFlags := o.PrintFlags.Copy()

		if mapping != nil {
			if !cmdSpecifiesOutputFmt(cmd) && o.PrintWithOpenAPICols {
				if apiSchema, err := f.OpenAPISchema(); err == nil {
					printFlags.UseOpenAPIColumns(apiSchema, mapping)
				}
			}
			printFlags.SetKind(mapping.GroupVersionKind.GroupKind())
		}
		if withNamespace {
			printFlags.EnsureWithNamespace()
		}
		if withKind {
			printFlags.EnsureWithKind()
		}

		printer, err := printFlags.ToPrinter()
		if err != nil {
			return nil, err
		}
		printer, err = printers.NewTypeSetter(scheme.Scheme).WrapToPrinter(printer, nil)
		if err != nil {
			return nil, err
		}

		if o.Sort {
			printer = &kubectlget.SortingPrinter{Delegate: printer, SortField: sortBy}
		}
		if outputObjects != nil {
			printer = &skipPrinter{delegate: printer, output: outputObjects}
		}
		if o.ServerPrint {
			printer = &kubectlget.TablePrinter{Delegate: printer}
		}
		return printer.PrintObj, nil
	}

	switch {
	case o.Watch:
		if o.Sort {
			fmt.Fprintf(o.IOStreams.ErrOut, "warning: --watch requested, --sort-by will be ignored\n")
		}
	}

	// openapi printing is mutually exclusive with server side printing
	if o.PrintWithOpenAPICols && o.ServerPrint {
		fmt.Fprintf(o.IOStreams.ErrOut, "warning: --%s requested, --%s will be ignored\n", useOpenAPIPrintColumnFlagLabel, useServerPrintColumns)
	}

	config, err := o.configFlags.ToRawKubeConfigLoader().RawConfig()
	if err != nil {
		return err
	}

	o.nonk8sAPIURL, err = pluginutil.GetNonK8sAPIURL(config)
	if err != nil {
		return err
	}

	o.token, err = pluginutil.GetToken(config)
	if err != nil {
		return err
	}

	return nil
}

// Validate checks the set of flags provided by the user.
func (o *Options) Validate(cmd *cobra.Command, args []string) error {
	if cmdutil.GetFlagBool(cmd, "show-labels") {
		outputOption := cmd.Flags().Lookup("output").Value.String()
		if outputOption != "" && outputOption != "wide" {
			return fmt.Errorf("--show-labels option cannot be used with %s printer", outputOption)
		}
	}
	if o.OutputWatchEvents && !o.Watch {
		return cmdutil.UsageErrorf(cmd, "--output-watch-events option can only be used with --watch")
	}

	if len(args) > 0 {
		return cmdutil.UsageErrorf(cmd, "currently, only getting all the clusters is supported")
	}
	return nil
}

// OriginalPositioner and NopPositioner is required for swap/sort operations of data in table format
type OriginalPositioner interface {
	OriginalPosition(int) int
}

// NopPositioner and OriginalPositioner is required for swap/sort operations of data in table format
type NopPositioner struct{}

// OriginalPosition returns the original position from NopPositioner object
func (t *NopPositioner) OriginalPosition(ix int) int {
	return ix
}

// RuntimeSorter holds the required objects to perform sorting of runtime objects
type RuntimeSorter struct {
	field      string
	decoder    runtime.Decoder
	objects    []runtime.Object
	positioner OriginalPositioner
}

// Sort performs the sorting of runtime objects
func (r *RuntimeSorter) Sort() error {
	// a list is only considered "sorted" if there are 0 or 1 items in it
	// AND (if 1 item) the item is not a Table object
	if len(r.objects) == 0 {
		return nil
	}
	if len(r.objects) == 1 {
		_, isTable := r.objects[0].(*metav1.Table)
		if !isTable {
			return nil
		}
	}

	includesTable := false
	includesRuntimeObjs := false

	for _, obj := range r.objects {
		switch t := obj.(type) {
		case *metav1.Table:
			includesTable = true

			if sorter, err := kubectlget.NewTableSorter(t, r.field); err != nil {
				return err
			} else if err := sorter.Sort(); err != nil {
				return err
			}
		default:
			includesRuntimeObjs = true
		}
	}

	// we use a NopPositioner when dealing with Table objects
	// because the objects themselves are not swapped, but rather
	// the rows in each object are swapped / sorted.
	r.positioner = &NopPositioner{}

	if includesRuntimeObjs && includesTable {
		return fmt.Errorf("sorting is not supported on mixed Table and non-Table object lists")
	}
	if includesTable {
		return nil
	}

	// if not dealing with a Table response from the server, assume
	// all objects are runtime.Object as usual, and sort using old method.
	var err error
	if r.positioner, err = kubectlget.SortObjects(r.decoder, r.objects, r.field); err != nil {
		return err
	}
	return nil
}

// OriginalPosition returns the original position of a runtime object
func (r *RuntimeSorter) OriginalPosition(ix int) int {
	if r.positioner == nil {
		return 0
	}
	return r.positioner.OriginalPosition(ix)
}

// WithDecoder allows custom decoder to be set for testing
func (r *RuntimeSorter) WithDecoder(decoder runtime.Decoder) *RuntimeSorter {
	r.decoder = decoder
	return r
}

// NewRuntimeSorter returns a new instance of RuntimeSorter
func NewRuntimeSorter(objects []runtime.Object, sortBy string) *RuntimeSorter {
	parsedField, err := kubectlget.RelaxedJSONPathExpression(sortBy)
	if err != nil {
		parsedField = sortBy
	}

	return &RuntimeSorter{
		field:   parsedField,
		decoder: kubernetesscheme.Codecs.UniversalDecoder(),
		objects: objects,
	}
}

func (o *Options) transformRequests(req *rest.Request) {
	// We need full objects if printing with openapi columns
	if o.PrintWithOpenAPICols {
		return
	}
	if !o.ServerPrint || !o.IsHumanReadablePrinter {
		return
	}

	req.SetHeader("Accept", strings.Join([]string{
		fmt.Sprintf("application/json;as=Table;v=%s;g=%s", metav1.SchemeGroupVersion.Version, metav1.GroupName),
		fmt.Sprintf("application/json;as=Table;v=%s;g=%s", metav1beta1.SchemeGroupVersion.Version, metav1beta1.GroupName),
		"application/json",
	}, ","))

	// if sorting, ensure we receive the full object in order to introspect its fields via jsonpath
	if o.Sort {
		req.Param("includeObject", "Object")
	}
}

// Run performs the get operation.
// TODO: remove the need to pass these arguments, like other commands.
func (o *Options) Run(cmd *cobra.Command, args []string) error {
	//TODO
	// if o.Watch {
	//	return o.watch(f, cmd, args)
	//}

	chunkSize := o.ChunkSize
	if o.Sort {
		// TODO(juanvallejo): in the future, we could have the client use chunking
		// to gather all results, then sort them all at the end to reduce server load.
		chunkSize = 0
	}

	// TODO fix
	_ = chunkSize

	client, err := createClient()
	if err != nil {
		return fmt.Errorf("unable to create client: %w", err)
	}

	req, err := http.NewRequestWithContext(context.TODO(), "GET",
		fmt.Sprintf("%s/multicloud/hub-of-hubs-nonk8s-api/%s", o.nonk8sAPIURL, o.resourcePath), nil)
	if err != nil {
		return fmt.Errorf("unable to create request: %w", err)
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", o.token))

	if o.ServerPrint {
		req.Header.Add("Accept", strings.Join([]string{
			fmt.Sprintf("application/json;as=Table;v=%s;g=%s", metav1.SchemeGroupVersion.Version, metav1.GroupName),
			"application/json",
		}, ","))
	} else {
		req.Header.Add("Accept", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("got error: %w", err)
	}
	defer resp.Body.Close()

	if o.IgnoreNotFound && resp.StatusCode != http.StatusNotFound {
		return nil
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %d", errStatusNotOK, resp.StatusCode)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("unable to read response body: %w", err)
	}

	objs, err := getObjects(body)
	if err != nil {
		return fmt.Errorf("unable to get objects from the body: %w", err)
	}

	if !o.IsHumanReadablePrinter {
		return o.printGeneric(objs)
	}

	allErrs := []error{}
	errs := sets.NewString()

	sorting, err := cmd.Flags().GetString("sort-by")
	if err != nil {
		return err
	}

	var positioner OriginalPositioner
	if o.Sort {
		sorter := NewRuntimeSorter(objs, sorting)
		if err := sorter.Sort(); err != nil {
			return err
		}
		positioner = sorter
	}

	var printer printers.ResourcePrinter

	// track if we write any output
	trackingWriter := &trackingWriterWrapper{Delegate: o.Out}
	// output an empty line separating output
	separatorWriter := &separatorWriterWrapper{Delegate: trackingWriter}

	w := printers.GetNewTabWriter(separatorWriter)
	printer, err = o.ToPrinter(o.mapping, nil, false, false)

	if err != nil {
		if !errs.Has(err.Error()) {
			errs.Insert(err.Error())
			allErrs = append(allErrs, err)
		}
	} else {
		for ix := range objs {
			var obj runtime.Object

			if positioner != nil {
				obj = objs[positioner.OriginalPosition(ix)]
			} else {
				obj = objs[ix]
			}

			// ensure a versioned object is passed to the custom-columns printer
			// if we are using OpenAPI columns to print
			if o.PrintWithOpenAPICols {
				printer.PrintObj(obj, w)
				continue
			}

			printer.PrintObj(obj, w)
		}
	}
	w.Flush()
	if trackingWriter.Written == 0 && !o.IgnoreNotFound && len(allErrs) == 0 {
		fmt.Fprintln(o.ErrOut, "No resources found")
	}

	return utilerrors.NewAggregate(allErrs)
}

func getObjects(rawBytes []byte) ([]runtime.Object, error) {
	var results []interface{}

	err := json.Unmarshal(rawBytes, &results)
	if err != nil {
		var result interface{}
		err := json.Unmarshal(rawBytes, &result)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshall json: %w", err)
		}
		results = append(results, result)
	}

	var objects []runtime.Object

	for _, result := range results {
		resultData, err := json.Marshal(result)
		if err != nil {
			return nil, fmt.Errorf("failed to marshall json: %w", err)
		}

		converted, err := runtime.Decode(unstructured.UnstructuredJSONScheme, resultData)
		if err != nil {
			return nil, fmt.Errorf("failed to decode: %w", err)
		}

		objects = append(objects, converted)
	}

	return objects, nil
}

type trackingWriterWrapper struct {
	Delegate io.Writer
	Written  int
}

func (t *trackingWriterWrapper) Write(p []byte) (n int, err error) {
	t.Written += len(p)
	return t.Delegate.Write(p)
}

type separatorWriterWrapper struct {
	Delegate io.Writer
	Ready    bool
}

func (s *separatorWriterWrapper) Write(p []byte) (n int, err error) {
	// If we're about to write non-empty bytes and `s` is ready,
	// we prepend an empty line to `p` and reset `s.Read`.
	if len(p) != 0 && s.Ready {
		fmt.Fprintln(s.Delegate)
		s.Ready = false
	}
	return s.Delegate.Write(p)
}

func (s *separatorWriterWrapper) SetReady(state bool) {
	s.Ready = state
}

// watch starts a client-side watch of one or more resources.
// TODO: remove the need for arguments here.
func (o *Options) watch(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	r := &resource.Result{}

	if err := r.Err(); err != nil {
		return err
	}
	infos, err := r.Infos()
	if err != nil {
		return err
	}
	if multipleGVKsRequested(infos) {
		return i18n.Errorf("watch is only supported on individual resources and resource collections - more than 1 resource was found")
	}

	info := infos[0]
	mapping := info.ResourceMapping()
	outputObjects := utilpointer.BoolPtr(true)
	printer, err := o.ToPrinter(mapping, outputObjects, false, false)
	if err != nil {
		return err
	}
	obj, err := r.Object()
	if err != nil {
		return err
	}

	// watching from resourceVersion 0, starts the watch at ~now and
	// will return an initial watch event.  Starting form ~now, rather
	// the rv of the object will insure that we start the watch from
	// inside the watch window, which the rv of the object might not be.
	rv := "0"
	isList := meta.IsListType(obj)
	if isList {
		// the resourceVersion of list objects is ~now but won't return
		// an initial watch event
		rv, err = meta.NewAccessor().ResourceVersion(obj)
		if err != nil {
			return err
		}
	}

	writer := printers.GetNewTabWriter(o.Out)

	// print the current object
	var objsToPrint []runtime.Object
	if isList {
		objsToPrint, _ = meta.ExtractList(obj)
	} else {
		objsToPrint = append(objsToPrint, obj)
	}
	for _, objToPrint := range objsToPrint {
		if o.OutputWatchEvents {
			objToPrint = &metav1.WatchEvent{Type: string(watch.Added), Object: runtime.RawExtension{Object: objToPrint}}
		}
		if err := printer.PrintObj(objToPrint, writer); err != nil {
			return fmt.Errorf("unable to output the provided object: %v", err)
		}
	}
	writer.Flush()
	if isList {
		// we can start outputting objects now, watches started from lists don't emit synthetic added events
		*outputObjects = true
	} else {
		// suppress output, since watches started for individual items emit a synthetic ADDED event first
		*outputObjects = false
	}

	// print watched changes
	w, err := r.Watch(rv)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	intr := interrupt.New(nil, cancel)
	intr.Run(func() error {
		_, err := watchtools.UntilWithoutRetry(ctx, w, func(e watch.Event) (bool, error) {
			objToPrint := e.Object
			if o.OutputWatchEvents {
				objToPrint = &metav1.WatchEvent{Type: string(e.Type), Object: runtime.RawExtension{Object: objToPrint}}
			}
			if err := printer.PrintObj(objToPrint, writer); err != nil {
				return false, err
			}
			writer.Flush()
			// after processing at least one event, start outputting objects
			*outputObjects = true
			return false, nil
		})
		return err
	})
	return nil
}

func (o *Options) printGeneric(objects []runtime.Object) error {
	var errs []error

	if len(objects) == 0 && o.IgnoreNotFound {
		return utilerrors.Reduce(utilerrors.Flatten(utilerrors.NewAggregate(errs)))
	}

	printer, err := o.ToPrinter(nil, nil, false, false)
	if err != nil {
		return err
	}

	var obj runtime.Object
	if len(objects) != 1 {
		// we have zero or multple items, so coerce all items into a list.
		// we don't want an *unstructured.Unstructured list yet, as we
		// may be dealing with non-unstructured objects. Compose all items
		// into an corev1.List, and then decode using an unstructured scheme.
		list := corev1.List{
			TypeMeta: metav1.TypeMeta{
				Kind:       "List",
				APIVersion: "v1",
			},
			ListMeta: metav1.ListMeta{},
		}
		for _, object := range objects {
			list.Items = append(list.Items, runtime.RawExtension{Object: object})
		}

		listData, err := json.Marshal(list)
		if err != nil {
			return err
		}

		converted, err := runtime.Decode(unstructured.UnstructuredJSONScheme, listData)
		if err != nil {
			return err
		}

		obj = converted
	} else {
		obj = objects[0]
	}

	isList := meta.IsListType(obj)
	if isList {
		items, err := meta.ExtractList(obj)
		if err != nil {
			return err
		}

		// take the items and create a new list for display
		list := &unstructured.UnstructuredList{
			Object: map[string]interface{}{
				"kind":       "List",
				"apiVersion": "v1",
				"metadata":   map[string]interface{}{},
			},
		}
		if listMeta, err := meta.ListAccessor(obj); err == nil {
			list.Object["metadata"] = map[string]interface{}{
				"resourceVersion": listMeta.GetResourceVersion(),
			}
		}

		for _, item := range items {
			list.Items = append(list.Items, *item.(*unstructured.Unstructured))
		}
		if err := printer.PrintObj(list, o.Out); err != nil {
			errs = append(errs, err)
		}
		return utilerrors.Reduce(utilerrors.Flatten(utilerrors.NewAggregate(errs)))
	}

	if printErr := printer.PrintObj(obj, o.Out); printErr != nil {
		errs = append(errs, printErr)
	}

	return utilerrors.Reduce(utilerrors.Flatten(utilerrors.NewAggregate(errs)))
}

func addOpenAPIPrintColumnFlags(cmd *cobra.Command, opt *Options) {
	cmd.Flags().BoolVar(&opt.PrintWithOpenAPICols, useOpenAPIPrintColumnFlagLabel, opt.PrintWithOpenAPICols, "If true, use x-kubernetes-print-column metadata (if present) from the OpenAPI schema for displaying a resource.")
	cmd.Flags().MarkDeprecated(useOpenAPIPrintColumnFlagLabel, "deprecated in favor of server-side printing")
}

func addServerPrintColumnFlags(cmd *cobra.Command, opt *Options) {
	cmd.Flags().BoolVar(&opt.ServerPrint, useServerPrintColumns, opt.ServerPrint, "If true, have the server return the appropriate table output. Supports extension APIs and CRDs.")
}

func shouldGetNewPrinterForMapping(printer printers.ResourcePrinter, lastMapping, mapping *meta.RESTMapping) bool {
	return printer == nil || lastMapping == nil || mapping == nil || mapping.Resource != lastMapping.Resource
}

func cmdSpecifiesOutputFmt(cmd *cobra.Command) bool {
	return cmdutil.GetFlagString(cmd, "output") != ""
}

func multipleGVKsRequested(infos []*resource.Info) bool {
	if len(infos) < 2 {
		return false
	}
	gvk := infos[0].Mapping.GroupVersionKind
	for _, info := range infos {
		if info.Mapping.GroupVersionKind != gvk {
			return true
		}
	}
	return false
}

func multipleGVKsRequestedForObjects(objects []runtime.Object) bool {
	if len(objects) < 2 {
		return false
	}
	gvk := objects[0].GetObjectKind().GroupVersionKind()
	for _, object := range objects {
		if object.GetObjectKind().GroupVersionKind() != gvk {
			return true
		}
	}
	return false
}

func createClient() (*http.Client, error) {
	tlsConfig := &tls.Config{
		//nolint:gosec
		InsecureSkipVerify: true,
	}

	tr := &http.Transport{TLSClientConfig: tlsConfig}
	client := &http.Client{
		Transport:     tr,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	return client, nil
}
