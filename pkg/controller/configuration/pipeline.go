/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

package configuration

import (
	"strconv"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	appsv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	cfgutil "github.com/apecloud/kubeblocks/pkg/configuration/util"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/builder"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

type ReconcileCtx struct {
	*ResourceCtx

	Cluster              *appsv1.Cluster
	Component            *appsv1.Component
	SynthesizedComponent *component.SynthesizedComponent
	PodSpec              *corev1.PodSpec
	ComponentParameter   *parametersv1alpha1.ComponentParameter

	Cache []client.Object
}

type reloadActionBuilderHelper struct {
	// configuration *appsv1alpha1.Configuration
	renderWrapper renderWrapper

	ctx ReconcileCtx
	ResourceFetcher[reloadActionBuilderHelper]
}

type updatePipeline struct {
	reconcile     bool
	renderWrapper renderWrapper

	item       appsv1alpha1.ConfigurationItemDetail
	itemStatus *appsv1alpha1.ConfigurationItemDetailStatus
	configSpec *appsv1.ComponentConfigSpec
	// replace of ConfigMapObj
	// originalCM  *corev1.ConfigMap
	newCM       *corev1.ConfigMap
	configPatch *core.ConfigPatchInfo

	ctx ReconcileCtx
	ResourceFetcher[updatePipeline]
}

func NewReloadActionBuilderHelper(ctx ReconcileCtx) *reloadActionBuilderHelper {
	p := &reloadActionBuilderHelper{ctx: ctx}
	return p.Init(ctx.ResourceCtx, p)
}

func NewReconcilePipeline(ctx ReconcileCtx, item appsv1alpha1.ConfigurationItemDetail, itemStatus *appsv1alpha1.ConfigurationItemDetailStatus, configSpec *appsv1.ComponentConfigSpec) *updatePipeline {
	p := &updatePipeline{
		reconcile:  true,
		item:       item,
		itemStatus: itemStatus,
		ctx:        ctx,
		configSpec: configSpec,
	}
	return p.Init(ctx.ResourceCtx, p)
}

func (p *reloadActionBuilderHelper) Prepare() *reloadActionBuilderHelper {
	buildTemplate := func() (err error) {
		ctx := p.ctx
		templateBuilder := newTemplateBuilder(p.ClusterName, p.Namespace, p.Context, p.Client)
		// Prepare built-in objects and built-in functions
		templateBuilder.injectBuiltInObjectsAndFunctions(ctx.PodSpec, ctx.SynthesizedComponent, ctx.Cache, ctx.Cluster)
		p.renderWrapper = newTemplateRenderWrapper(p.Context, ctx.Client, templateBuilder, ctx.Cluster, ctx.Component)
		return
	}

	return p.Wrap(buildTemplate)
}

func (p *reloadActionBuilderHelper) RenderScriptTemplate() *reloadActionBuilderHelper {
	return p.Wrap(func() error {
		ctx := p.ctx
		return p.renderWrapper.renderScriptTemplate(ctx.Cluster, ctx.SynthesizedComponent, ctx.Cache)
	})
}

func (p *reloadActionBuilderHelper) UpdateConfiguration() *reloadActionBuilderHelper {
	buildConfiguration := func() (err error) {
		expectedConfiguration := p.createConfiguration()
		if intctrlutil.SetControllerReference(p.ctx.Component, expectedConfiguration) != nil {
			return
		}
		_, _ = UpdateConfigPayload(&expectedConfiguration.Spec, p.ctx.SynthesizedComponent)

		existingConfiguration := appsv1alpha1.Configuration{}
		err = p.ResourceFetcher.Client.Get(p.Context, client.ObjectKeyFromObject(expectedConfiguration), &existingConfiguration)
		switch {
		case err == nil:
			return p.updateConfiguration(expectedConfiguration, &existingConfiguration)
		case apierrors.IsNotFound(err):
			return p.ResourceFetcher.Client.Create(p.Context, expectedConfiguration)
		default:
			return err
		}
	}
	return p.Wrap(buildConfiguration)
}

func (p *reloadActionBuilderHelper) CreateConfigTemplate() *reloadActionBuilderHelper {
	return p.Wrap(func() error {
		ctx := p.ctx
		return p.renderWrapper.renderConfigTemplate(ctx.Cluster, ctx.SynthesizedComponent, ctx.Cache, p.ComponentParameterObj)
	})
}

func (p *reloadActionBuilderHelper) UpdateConfigurationStatus() *reloadActionBuilderHelper {
	return p.Wrap(func() error {
		if p.ComponentParameterObj == nil {
			return nil
		}

		existing := p.ComponentParameterObj
		reversion := fromConfiguration(existing)
		patch := client.MergeFrom(existing)
		updated := existing.DeepCopy()
		for _, item := range existing.Spec.ConfigItemDetails {
			CheckAndUpdateItemStatus(updated, item, reversion)
		}
		return p.ResourceFetcher.Client.Status().Patch(p.Context, updated, patch)
	})
}

func CheckAndUpdateItemStatus(updated *appsv1alpha1.Configuration, item appsv1alpha1.ConfigurationItemDetail, reversion string) {
	foundStatus := func(name string) *appsv1alpha1.ConfigurationItemDetailStatus {
		for i := range updated.Status.ConfigurationItemStatus {
			status := &updated.Status.ConfigurationItemStatus[i]
			if status.Name == name {
				return status
			}
		}
		return nil
	}

	status := foundStatus(item.Name)
	if status != nil && status.Phase == "" {
		status.Phase = appsv1alpha1.CInitPhase
	}
	if status == nil {
		updated.Status.ConfigurationItemStatus = append(updated.Status.ConfigurationItemStatus,
			appsv1alpha1.ConfigurationItemDetailStatus{
				Name:           item.Name,
				Phase:          appsv1alpha1.CInitPhase,
				UpdateRevision: reversion,
			})
	}
}

func (p *reloadActionBuilderHelper) UpdatePodVolumes() *reloadActionBuilderHelper {
	return p.Wrap(func() error {
		return intctrlutil.CreateOrUpdatePodVolumes(p.ctx.PodSpec,
			p.renderWrapper.volumes,
			configSetFromComponent(p.ctx.SynthesizedComponent.ConfigTemplates))
	})
}

func (p *reloadActionBuilderHelper) BuildConfigManagerSidecar() *reloadActionBuilderHelper {
	return p.Wrap(func() error {
		return buildConfigManagerWithComponent(p.ctx.PodSpec, p.ctx.SynthesizedComponent.ConfigTemplates, p.Context, p.Client, p.ctx.Cluster, p.ctx.SynthesizedComponent)
	})
}

func (p *reloadActionBuilderHelper) UpdateConfigRelatedObject() *reloadActionBuilderHelper {
	updateMeta := func() error {
		if err := injectTemplateEnvFrom(p.ctx.Cluster, p.ctx.SynthesizedComponent, p.ctx.PodSpec, p.Client, p.Context, p.renderWrapper.renderedObjs); err != nil {
			return err
		}
		return createConfigObjects(p.Client, p.Context, p.renderWrapper.renderedObjs, p.renderWrapper.renderedSecretObjs)
	}

	return p.Wrap(updateMeta)
}

func (p *reloadActionBuilderHelper) createConfiguration() *appsv1alpha1.Configuration {
	builder := builder.NewConfigurationBuilder(p.Namespace,
		core.GenerateComponentConfigurationName(p.ClusterName, p.ComponentName),
	)
	for _, template := range p.ctx.SynthesizedComponent.ConfigTemplates {
		builder.AddConfigurationItem(template)
	}
	return builder.Component(p.ComponentName).
		ClusterRef(p.ClusterName).
		AddLabelsInMap(constant.GetCompLabels(p.ClusterName, p.ComponentName)).
		GetObject()
}

func (p *reloadActionBuilderHelper) updateConfiguration(expected *appsv1alpha1.Configuration, existing *appsv1alpha1.Configuration) error {
	fromMap := func(items []appsv1alpha1.ConfigurationItemDetail) *cfgutil.Sets {
		sets := cfgutil.NewSet()
		for _, item := range items {
			sets.Add(item.Name)
		}
		return sets
	}

	updateConfigSpec := func(item appsv1alpha1.ConfigurationItemDetail) appsv1alpha1.ConfigurationItemDetail {
		if newItem := expected.Spec.GetConfigurationItem(item.Name); newItem != nil {
			item.ConfigSpec = newItem.ConfigSpec
		}
		return item
	}

	oldSets := fromMap(existing.Spec.ConfigItemDetails)
	newSets := fromMap(expected.Spec.ConfigItemDetails)

	addSets := cfgutil.Difference(newSets, oldSets)
	delSets := cfgutil.Difference(oldSets, newSets)

	newConfigItems := make([]appsv1alpha1.ConfigurationItemDetail, 0)
	for _, item := range existing.Spec.ConfigItemDetails {
		if !delSets.InArray(item.Name) {
			newConfigItems = append(newConfigItems, updateConfigSpec(item))
		}
	}
	for _, item := range expected.Spec.ConfigItemDetails {
		if addSets.InArray(item.Name) {
			newConfigItems = append(newConfigItems, item)
		}
	}

	patch := client.MergeFrom(existing)
	updated := existing.DeepCopy()
	updated.Spec.ConfigItemDetails = newConfigItems
	return p.Client.Patch(p.Context, updated, patch)
}

func (p *updatePipeline) isDone() bool {
	return !p.reconcile
}

func (p *updatePipeline) PrepareForTemplate() *updatePipeline {
	buildTemplate := func() (err error) {
		p.reconcile = !intctrlutil.IsApplyConfigChanged(p.ConfigMapObj, p.item)
		if p.isDone() {
			return
		}
		templateBuilder := newTemplateBuilder(p.ClusterName, p.Namespace, p.Context, p.Client)
		// Prepare built-in objects and built-in functions
		templateBuilder.injectBuiltInObjectsAndFunctions(p.ctx.PodSpec, p.ctx.SynthesizedComponent, p.ctx.Cache, p.ctx.Cluster)
		p.renderWrapper = newTemplateRenderWrapper(p.Context, p.Client, templateBuilder, p.ctx.Cluster, p.ctx.Component)
		return
	}
	return p.Wrap(buildTemplate)
}

func (p *updatePipeline) ConfigSpec() *appsv1.ComponentConfigSpec {
	return p.configSpec
}

func (p *updatePipeline) InitConfigSpec() *updatePipeline {
	return p.Wrap(func() (err error) {
		if p.configSpec == nil {
			p.configSpec = component.GetConfigSpecByName(p.ctx.SynthesizedComponent, p.item.Name)
			if p.configSpec == nil {
				return core.MakeError("not found config spec: %s", p.item.Name)
			}
		}
		return
	})
}

func (p *updatePipeline) RerenderTemplate() *updatePipeline {
	return p.Wrap(func() (err error) {
		if p.isDone() {
			return
		}
		if intctrlutil.IsRerender(p.ConfigMapObj, p.item) {
			p.newCM, err = p.renderWrapper.rerenderConfigTemplate(p.ctx.Cluster, p.ctx.SynthesizedComponent, *p.configSpec, &p.item)
		} else {
			p.newCM = p.ConfigMapObj.DeepCopy()
		}
		return
	})
}

func (p *updatePipeline) ApplyParameters() *updatePipeline {
	patchMerge := func(p *updatePipeline, spec appsv1.ComponentConfigSpec, cm *corev1.ConfigMap, item appsv1alpha1.ConfigurationItemDetail) error {
		if p.isDone() || len(item.ConfigFileParams) == 0 {
			return nil
		}
		newData, err := DoMerge(cm.Data, item.ConfigFileParams, p.ConfigConstraintObj, spec)
		if err != nil {
			return err
		}
		if p.ConfigConstraintObj == nil {
			cm.Data = newData
			return nil
		}

		p.configPatch, _, err = core.CreateConfigPatch(cm.Data,
			newData,
			p.ConfigConstraintObj.Spec.FileFormatConfig.Format,
			p.configSpec.Keys,
			false)
		if err != nil {
			return err
		}
		cm.Data = newData
		return nil
	}

	return p.Wrap(func() error {
		if p.isDone() {
			return nil
		}
		return patchMerge(p, *p.configSpec, p.newCM, p.item)
	})
}

func (p *updatePipeline) UpdateConfigVersion(revision string) *updatePipeline {
	return p.Wrap(func() error {
		if p.isDone() {
			return nil
		}

		if err := updateConfigMetaForCM(p.newCM, &p.item, revision); err != nil {
			return err
		}
		annotations := p.newCM.Annotations
		if annotations == nil {
			annotations = make(map[string]string)
		}

		// delete disable reconcile annotation
		if _, ok := annotations[constant.DisableUpgradeInsConfigurationAnnotationKey]; ok {
			annotations[constant.DisableUpgradeInsConfigurationAnnotationKey] = strconv.FormatBool(false)
		}
		p.newCM.Annotations = annotations
		// p.itemStatus.UpdateRevision = revision
		return nil
	})
}

// TODO(leon)
func (p *updatePipeline) Sync() *updatePipeline {
	return p.Wrap(func() error {
		if p.ConfigConstraintObj != nil && !p.isDone() {
			if err := SyncEnvSourceObject(*p.configSpec, p.newCM, &p.ConfigConstraintObj.Spec, p.Client, p.Context, p.ctx.Cluster, p.ctx.SynthesizedComponent); err != nil {
				return err
			}
		}
		if InjectEnvEnabled(*p.configSpec) && toSecret(*p.configSpec) {
			return nil
		}
		switch {
		case p.isDone():
			return nil
		case p.ConfigMapObj == nil && p.newCM != nil:
			return p.Client.Create(p.Context, p.newCM)
		case p.ConfigMapObj != nil:
			patch := client.MergeFrom(p.ConfigMapObj)
			if p.ConfigMapObj != nil {
				p.newCM.Labels = intctrlutil.MergeMetadataMaps(p.newCM.Labels, p.ConfigMapObj.Labels)
				p.newCM.Annotations = intctrlutil.MergeMetadataMaps(p.newCM.Annotations, p.ConfigMapObj.Annotations)
			}
			return p.Client.Patch(p.Context, p.newCM, patch)
		}
		return core.MakeError("unexpected condition")
	})
}

func (p *updatePipeline) SyncStatus() *updatePipeline {
	return p.Wrap(func() (err error) {
		if p.isDone() {
			return
		}
		if p.configSpec == nil || p.itemStatus == nil {
			return
		}
		p.itemStatus.Phase = appsv1alpha1.CMergedPhase
		return
	})
}
