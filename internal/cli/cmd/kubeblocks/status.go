/*
Copyright (C) 2022-2023 ApeCloud Co., Ltd

This file is part of KubeBlocks project

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program.  If not, see <http://www.gnu.org/licenses/>.
*/

package kubeblocks

import (
	"context"
	"fmt"
	"strings"

	"github.com/containerd/stargz-snapshotter/estargz/errorutil"
	"github.com/spf13/cobra"

	extensionsv1alpha1 "github.com/apecloud/kubeblocks/apis/extensions/v1alpha1"
	"github.com/apecloud/kubeblocks/internal/cli/printer"
	"github.com/apecloud/kubeblocks/internal/cli/types"
	"github.com/apecloud/kubeblocks/internal/cli/util"
	"github.com/apecloud/kubeblocks/internal/constant"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/util/templates"
	"k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metrics "k8s.io/metrics/pkg/client/clientset/versioned"
)

var (
	infoExample = templates.Examples(`
	# list workloads owned by KubeBlocks
	kbcli kubeblocks status

	# list all resources owned by KubeBlocks, such as workloads, cluster definitions, backup template.
	kbcli kubeblocks status --all`)
)

var (
	selectorList = []metav1.ListOptions{{LabelSelector: types.InstanceLabelSelector}, {LabelSelector: types.ReleaseLabelSelector}}

	kubeBlocksWorkloads = []schema.GroupVersionResource{
		types.DeployGVR(),
		types.StatefulSetGVR(),
	}

	kubeBlocksGlobalCustomResources = []schema.GroupVersionResource{
		types.BackupToolGVR(),
		types.ClusterDefGVR(),
		types.ClusterVersionGVR(),
		types.ConfigConstraintGVR(),
	}

	kubeBlocksConfigurations = []schema.GroupVersionResource{
		types.ConfigmapGVR(),
		types.SecretGVR(),
		types.ServiceGVR(),
	}

	kubeBlocksClusterRBAC = []schema.GroupVersionResource{
		types.ClusterRoleGVR(),
		types.ClusterRoleBindingGVR(),
	}

	kubeBlocksNamespacedRBAC = []schema.GroupVersionResource{
		types.RoleGVR(),
		types.RoleBindingGVR(),
		types.ServiceAccountGVR(),
	}

	kubeBlocksStorages = []schema.GroupVersionResource{
		types.PVCGVR(),
	}

	helmConfigurations = []schema.GroupVersionResource{
		types.ConfigmapGVR(),
		types.SecretGVR(),
	}
	notAvailable = "N/A"
)

type statusOptions struct {
	genericclioptions.IOStreams
	client  kubernetes.Interface
	dynamic dynamic.Interface
	mc      metrics.Interface
	showAll bool
	ns      string
	addons  []*extensionsv1alpha1.Addon
}

func newStatusCmd(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := statusOptions{IOStreams: streams}
	cmd := &cobra.Command{
		Use:     "status",
		Short:   "Show list of resource KubeBlocks uses or owns.",
		Args:    cobra.NoArgs,
		Example: infoExample,
		Run: func(cmd *cobra.Command, args []string) {
			util.CheckErr(o.complete(f))
			util.CheckErr(o.run())
		},
	}
	cmd.Flags().BoolVarP(&o.showAll, "all", "A", false, "Show all resources, including configurations, storages, etc")
	return cmd
}

func (o *statusOptions) complete(f cmdutil.Factory) error {
	var err error

	o.dynamic, err = f.DynamicClient()
	if err != nil {
		return err
	}

	o.client, err = f.KubernetesClientSet()
	if err != nil {
		return err
	}

	config, err := f.ToRESTConfig()
	if err != nil {
		return err
	}

	o.mc, err = metrics.NewForConfig(config)
	if err != nil {
		return err
	}
	o.ns = metav1.NamespaceAll
	return nil
}

func (o *statusOptions) run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	o.ns, _ = util.GetKubeBlocksNamespace(o.client)
	if o.ns == "" {
		printer.Warning(o.Out, "Failed to find deployed KubeBlocks in any namespace\n")
		printer.Warning(o.Out, "Will check all namespaces for KubeBlocks resources left behind\n")
	} else {
		fmt.Fprintf(o.Out, "Kuberblocks is deployed in namespace: %s\n", o.ns)
	}

	allErrs := make([]error, 0)
	o.buildSelectorList(ctx, &allErrs)
	o.showWorkloads(ctx, &allErrs)
	o.showAddons()

	if o.showAll {
		o.showKubeBlocksResources(ctx, &allErrs)
		o.showKubeBlocksConfig(ctx, &allErrs)
		o.showKubeBlocksRBAC(ctx, &allErrs)
		o.showKubeBlocksStorage(ctx, &allErrs)
		o.showHelmResources(ctx, &allErrs)
	}
	return errorutil.Aggregate(allErrs)
}

func (o *statusOptions) buildSelectorList(ctx context.Context, allErrs *[]error) {
	addons := make([]*extensionsv1alpha1.Addon, 0)
	objs, err := o.dynamic.Resource(types.AddonGVR()).List(ctx, metav1.ListOptions{})
	appendErrIgnoreNotFound(allErrs, err)
	if objs != nil {
		for _, obj := range objs.Items {
			addon := &extensionsv1alpha1.Addon{}
			if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, addon); err != nil {
				appendErrIgnoreNotFound(allErrs, err)
				continue
			}
			addons = append(addons, addon)
		}
	}

	// build addon instance selector
	o.addons = addons

	var selectors []metav1.ListOptions
	for _, selector := range buildResourceLabelSelectors(addons) {
		selectors = append(selectors, metav1.ListOptions{LabelSelector: selector})
	}
	selectorList = selectors
}

func (o *statusOptions) showAddons() {
	fmt.Fprintln(o.Out, "\nKubeBlocks Addons:")
	tbl := printer.NewTablePrinter(o.Out)
	tbl.SetHeader("NAME", "STATUS", "TYPE", "PROVIDER")

	var provider string
	var ok bool
	for _, addon := range o.addons {
		if addon.Labels == nil {
			provider = notAvailable
		} else if provider, ok = addon.Labels[constant.AddonProviderLableKey]; !ok {
			provider = notAvailable
		}
		tbl.AddRow(addon.Name, addon.Status.Phase, addon.Spec.Type, provider)
	}
	tbl.Print()
}

func (o *statusOptions) showKubeBlocksResources(ctx context.Context, allErrs *[]error) {
	fmt.Fprintln(o.Out, "\nKubeBlocks Global Custom Resources:")
	tblPrinter := printer.NewTablePrinter(o.Out)
	tblPrinter.SetHeader("KIND", "NAME")

	unstructuredList := listResourceByGVR(ctx, o.dynamic, metav1.NamespaceAll, kubeBlocksGlobalCustomResources, selectorList, allErrs)
	for _, resourceList := range unstructuredList {
		for _, resource := range resourceList.Items {
			tblPrinter.AddRow(resource.GetKind(), resource.GetName())
		}
	}
	tblPrinter.Print()
}

func (o *statusOptions) showKubeBlocksConfig(ctx context.Context, allErrs *[]error) {
	fmt.Fprintln(o.Out, "\nKubeBlocks Configurations:")
	tblPrinter := printer.NewTablePrinter(o.Out)
	tblPrinter.SetHeader("NAMESPACE", "KIND", "NAME")
	unstructuredList := listResourceByGVR(ctx, o.dynamic, o.ns, kubeBlocksConfigurations, selectorList, allErrs)
	for _, resourceList := range unstructuredList {
		for _, resource := range resourceList.Items {
			tblPrinter.AddRow(resource.GetNamespace(), resource.GetKind(), resource.GetName())
		}
	}
	tblPrinter.Print()
}

func (o *statusOptions) showKubeBlocksRBAC(ctx context.Context, allErrs *[]error) {
	fmt.Fprintln(o.Out, "\nKubeBlocks Global RBAC:")
	tblPrinter := printer.NewTablePrinter(o.Out)
	tblPrinter.SetHeader("KIND", "NAME")
	unstructuredList := listResourceByGVR(ctx, o.dynamic, metav1.NamespaceAll, kubeBlocksClusterRBAC, selectorList, allErrs)
	for _, resourceList := range unstructuredList {
		for _, resource := range resourceList.Items {
			tblPrinter.AddRow(resource.GetKind(), resource.GetName())
		}
	}

	tblPrinter.Print()

	fmt.Fprintln(o.Out, "\nKubeBlocks Namespaced RBAC:")
	tblPrinter = printer.NewTablePrinter(o.Out)
	tblPrinter.SetHeader("NAMESPACE", "KIND", "NAME")
	unstructuredList = listResourceByGVR(ctx, o.dynamic, o.ns, kubeBlocksNamespacedRBAC, selectorList, allErrs)
	for _, resourceList := range unstructuredList {
		for _, resource := range resourceList.Items {
			tblPrinter.AddRow(resource.GetNamespace(), resource.GetKind(), resource.GetName())
		}
	}

	tblPrinter.Print()
}

func (o *statusOptions) showKubeBlocksStorage(ctx context.Context, allErrs *[]error) {
	fmt.Fprintln(o.Out, "\nKubeBlocks Storage:")
	tblPrinter := printer.NewTablePrinter(o.Out)
	tblPrinter.SetHeader("NAMESPACE", "KIND", "NAME", "CAPACITY")

	renderPVC := func(raw *unstructured.Unstructured) {
		pvc := &corev1.PersistentVolumeClaim{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(raw.Object, pvc)
		if err != nil {
			appendErrIgnoreNotFound(allErrs, err)
			return
		}
		tblPrinter.AddRow(pvc.GetNamespace(), pvc.Kind, pvc.GetName(), pvc.Status.Capacity.Storage())
	}

	unstructuredList := listResourceByGVR(ctx, o.dynamic, o.ns, kubeBlocksStorages, selectorList, allErrs)
	for _, resourceList := range unstructuredList {
		for _, resource := range resourceList.Items {
			switch resource.GetKind() {
			case constant.PersistentVolumeClaimKind:
				renderPVC(&resource)
			default:
				err := fmt.Errorf("unsupported resources: %s", resource.GetKind())
				appendErrIgnoreNotFound(allErrs, err)
			}
		}
	}
	tblPrinter.Print()
}

func (o *statusOptions) showHelmResources(ctx context.Context, allErrs *[]error) {
	fmt.Fprintln(o.Out, "\nHelm Resources:")
	tblPrinter := printer.NewTablePrinter(o.Out)
	tblPrinter.SetHeader("NAMESPACE", "KIND", "NAME", "STATUS")

	helmLabel := func(name []string) string {
		return fmt.Sprintf("%s in (%s),%s=%s", "name", strings.Join(name, ","), "owner", "helm")
	}
	// init helm release list with 'kubeblocks'
	helmReleaseList := []string{types.KubeBlocksChartName}
	// add add one names name = $kubeblocks-addons$
	for _, addon := range o.addons {
		helmReleaseList = append(helmReleaseList, util.BuildAddonReleaseName(addon.Name))
	}
	// label selector 'owner=helm,name in (kubeblocks,kb-addon-mongodb,kb-addon-redis...)'
	selectors := []metav1.ListOptions{{LabelSelector: helmLabel(helmReleaseList)}}
	unstructuredList := listResourceByGVR(ctx, o.dynamic, o.ns, helmConfigurations, selectors, allErrs)
	for _, resourceList := range unstructuredList {
		for _, resource := range resourceList.Items {
			deployedStatus := resource.GetLabels()["status"]
			tblPrinter.AddRow(resource.GetNamespace(), resource.GetKind(), resource.GetName(), deployedStatus)
		}
	}
	tblPrinter.Print()
}

func (o *statusOptions) showWorkloads(ctx context.Context, allErrs *[]error) {
	fmt.Fprintln(o.Out, "\nKubeBlocks Workloads:")
	tblPrinter := printer.NewTablePrinter(o.Out)
	tblPrinter.SetHeader("NAMESPACE", "KIND", "NAME", "READY PODS", "CPU(cores)", "MEMORY(bytes)")

	unstructuredList := listResourceByGVR(ctx, o.dynamic, o.ns, kubeBlocksWorkloads, selectorList, allErrs)

	cpuMap, memMap := computeMetricByWorkloads(ctx, o.ns, unstructuredList, o.mc, allErrs)

	renderDeploy := func(raw *unstructured.Unstructured) {
		deploy := &appsv1.Deployment{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(raw.Object, deploy)
		if err != nil {
			appendErrIgnoreNotFound(allErrs, err)
			return
		}
		name := deploy.GetName()
		tblPrinter.AddRow(deploy.GetNamespace(), deploy.Kind, deploy.GetName(),
			fmt.Sprintf("%d/%d", deploy.Status.ReadyReplicas, deploy.Status.Replicas),
			cpuMap[name], memMap[name])
	}

	renderStatefulSet := func(raw *unstructured.Unstructured) {
		sts := &appsv1.StatefulSet{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(raw.Object, sts)
		if err != nil {
			appendErrIgnoreNotFound(allErrs, err)
			return
		}
		name := sts.GetName()
		tblPrinter.AddRow(sts.GetNamespace(), sts.Kind, sts.GetName(),
			fmt.Sprintf("%d/%d", sts.Status.ReadyReplicas, sts.Status.Replicas),
			cpuMap[name], memMap[name])
	}

	for _, workload := range unstructuredList {
		for _, resource := range workload.Items {
			switch resource.GetKind() {
			case constant.DeploymentKind:
				renderDeploy(&resource)
			case constant.StatefulSetKind:
				renderStatefulSet(&resource)
			default:
				err := fmt.Errorf("unsupported worklkoad type: %s", resource.GetKind())
				appendErrIgnoreNotFound(allErrs, err)
			}
		}
	}
	tblPrinter.Print()
}

func computeMetricByWorkloads(ctx context.Context, ns string, workloads []*unstructured.UnstructuredList, mc metrics.Interface, allErrs *[]error) (cpuMetricMap, memMetricMap map[string]string) {
	cpuMetricMap = make(map[string]string)
	memMetricMap = make(map[string]string)

	podsMetrics, err := mc.MetricsV1beta1().PodMetricses(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		appendErrIgnoreNotFound(allErrs, err)
		return
	}

	computeResources := func(name string, podsMetrics *v1beta1.PodMetricsList) {
		cpuUsage, memUsage := int64(0), int64(0)
		for _, pod := range podsMetrics.Items {
			if strings.HasPrefix(pod.Name, name) {
				for _, container := range pod.Containers {
					cpuUsage += container.Usage.Cpu().MilliValue()
					memUsage += container.Usage.Memory().Value() / (1024 * 1024)
				}
			}
		}
		cpuMetricMap[name] = fmt.Sprintf("%dm", cpuUsage)
		memMetricMap[name] = fmt.Sprintf("%dMi", memUsage)
	}

	for _, workload := range workloads {
		for _, resource := range workload.Items {
			name := resource.GetName()
			if podsMetrics == nil {
				cpuMetricMap[name] = notAvailable
				memMetricMap[name] = notAvailable
				continue
			}
			computeResources(name, podsMetrics)
		}
	}
	return cpuMetricMap, memMetricMap
}

func listResourceByGVR(ctx context.Context, client dynamic.Interface, namespace string, gvrlist []schema.GroupVersionResource, selector []metav1.ListOptions, allErrs *[]error) []*unstructured.UnstructuredList {
	unstructuredList := make([]*unstructured.UnstructuredList, 0)
	for _, gvr := range gvrlist {
		for _, labelSelector := range selector {
			klog.V(1).Infof("listResourceByGVR: namespace=%s, gvrlist=%v, selector=%v", namespace, gvr, labelSelector)
			resource, err := client.Resource(gvr).Namespace(namespace).List(ctx, labelSelector)
			if err != nil {
				appendErrIgnoreNotFound(allErrs, err)
				continue
			}
			unstructuredList = append(unstructuredList, resource)
		}
	}
	return unstructuredList
}

func appendErrIgnoreNotFound(allErrs *[]error, err error) {
	if err == nil || apierrors.IsNotFound(err) {
		return
	}
	*allErrs = append(*allErrs, err)
}
