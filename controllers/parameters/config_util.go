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

package parameters

import (
	"context"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	cfgcm "github.com/apecloud/kubeblocks/pkg/configuration/config_manager"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/constant"
	"github.com/apecloud/kubeblocks/pkg/controller/component"
	intctrlutil "github.com/apecloud/kubeblocks/pkg/controllerutil"
)

// type ValidateConfigMap func(configTpl, ns string) (*corev1.ConfigMap, error)
// type ValidateConfigSchema func(tpl *appsv1beta1.ParametersSchema) (bool, error)

func checkConfigLabels(object client.Object, requiredLabs []string) bool {
	labels := object.GetLabels()
	if len(labels) == 0 {
		return false
	}

	for _, label := range requiredLabs {
		if _, ok := labels[label]; !ok {
			return false
		}
	}

	// reconfigure ConfigMap for db instance
	if ins, ok := labels[constant.CMConfigurationTypeLabelKey]; !ok || ins != constant.ConfigInstanceType {
		return false
	}

	return checkEnableCfgUpgrade(object)
}

// func getConfigMapByTemplateName(cli client.Client, ctx intctrlutil.RequestCtx, templateName, ns string) (*corev1.ConfigMap, error) {
// 	if len(templateName) == 0 {
// 		return nil, fmt.Errorf("required configmap reference name is empty! [%v]", templateName)
// 	}
//
// 	configObj := &corev1.ConfigMap{}
// 	if err := cli.Get(ctx.Ctx, client.ObjectKey{
// 		Namespace: ns,
// 		Name:      templateName,
// 	}, configObj); err != nil {
// 		ctx.Log.Error(err, "failed to get config template cm object!", "configMapName", templateName)
// 		return nil, err
// 	}
//
// 	return configObj, nil
// }

// func checkConfigConstraint(ctx intctrlutil.RequestCtx, configConstraint *appsv1beta1.ConfigConstraint) (bool, error) {
// 	// validate configuration template
// 	validateConfigSchema := func(ccSchema *appsv1beta1.ParametersSchema) (bool, error) {
// 		if ccSchema == nil || len(ccSchema.CUE) == 0 {
// 			return true, nil
// 		}
//
// 		err := validate.CueValidate(ccSchema.CUE)
// 		return err == nil, err
// 	}
//
// 	// validate schema
// 	if ok, err := validateConfigSchema(configConstraint.Spec.ParametersSchema); !ok || err != nil {
// 		ctx.Log.Error(err, "failed to validate template schema!", "configMapName", fmt.Sprintf("%v", configConstraint.Spec.ParametersSchema))
// 		return ok, err
// 	}
// 	return true, nil
// }

// func ReconcileTemplateForReferencedCR(client client.Client, ctx intctrlutil.RequestCtx, cmpd *appsv1.ComponentDefinition) error {
// 	if ok, err := checkComponentTemplateCR(client, ctx, obj); !ok || err != nil {
// 		return fmt.Errorf("failed to check config template: %v", err)
// 	}
// 	if ok, err := updateLabelsByConfigSpec(client, ctx, obj); !ok || err != nil {
// 		return fmt.Errorf("failed to update using config template info: %v", err)
// 	}
// 	if _, err := updateConfigMapFinalizer(client, ctx, obj); err != nil {
// 		return fmt.Errorf("failed to update config map finalizer: %v", err)
// 	}
// 	return nil
// }

// func DeleteConfigMapFinalizer(cli client.Client, ctx intctrlutil.RequestCtx, obj client.Object) error {
// 	handler := func(configSpecs []appsv1.ComponentConfigSpec) (bool, error) {
// 		return true, batchDeleteConfigMapFinalizer(cli, ctx, configSpecs, obj)
// 	}
// 	_, err := handleConfigTemplate(obj, handler)
// 	return err
// }

// func validateConfigMapOwners(cli client.Client, ctx intctrlutil.RequestCtx, labels client.MatchingLabels, check func(obj client.Object) bool, objLists ...client.ObjectList) (bool, error) {
// 	for _, objList := range objLists {
// 		if err := cli.List(ctx.Ctx, objList, labels, client.Limit(2)); err != nil {
// 			return false, err
// 		}
// 		v, err := conversion.EnforcePtr(objList)
// 		if err != nil {
// 			return false, err
// 		}
// 		items := v.FieldByName("Items")
// 		if !items.IsValid() || items.Kind() != reflect.Slice || items.Len() > 1 {
// 			return false, nil
// 		}
// 		if items.Len() == 0 {
// 			continue
// 		}
//
// 		val := items.Index(0)
// 		// fetch object pointer
// 		if val.CanAddr() {
// 			val = val.Addr()
// 		}
// 		if !val.CanInterface() || !check(val.Interface().(client.Object)) {
// 			return false, nil
// 		}
// 	}
// 	return true, nil
// }

// func batchDeleteConfigMapFinalizer(cli client.Client, ctx intctrlutil.RequestCtx, configSpecs []appsv1.ComponentConfigSpec, cr client.Object) error {
// 	validator := func(obj client.Object) bool {
// 		return obj.GetName() == cr.GetName() && obj.GetNamespace() == cr.GetNamespace()
// 	}
// 	for _, configSpec := range configSpecs {
// 		labels := client.MatchingLabels{
// 			core.GenerateTPLUniqLabelKeyWithConfig(configSpec.Name): configSpec.TemplateRef,
// 		}
// 		if ok, err := validateConfigMapOwners(cli, ctx, labels, validator, &appsv1.ClusterDefinitionList{}, &appsv1.ComponentDefinitionList{}); err != nil {
// 			return err
// 		} else if !ok {
// 			continue
// 		}
// 		if err := deleteConfigMapFinalizer(cli, ctx, configSpec); err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }

// func updateConfigMapFinalizer(client client.Client, ctx intctrlutil.RequestCtx, obj client.Object) (bool, error) {
// 	handler := func(configSpecs []appsv1.ComponentConfigSpec) (bool, error) {
// 		return true, batchUpdateConfigMapFinalizer(client, ctx, configSpecs)
// 	}
// 	return handleConfigTemplate(obj, handler)
// }

// func batchUpdateConfigMapFinalizer(cli client.Client, ctx intctrlutil.RequestCtx, configSpecs []appsv1.ComponentConfigSpec) error {
// 	for _, configSpec := range configSpecs {
// 		if err := updateConfigMapFinalizerImpl(cli, ctx, configSpec); err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }

// func updateConfigMapFinalizerImpl(cli client.Client, ctx intctrlutil.RequestCtx, configSpec appsv1.ComponentConfigSpec) error {
// 	// step1: add finalizer
// 	// step2: add labels: CMConfigurationTypeLabelKey
// 	// step3: update immutable
//
// 	cmObj, err := getConfigMapByTemplateName(cli, ctx, configSpec.TemplateRef, configSpec.Namespace)
// 	if err != nil {
// 		ctx.Log.Error(err, "failed to get template cm object!", "configMapName", cmObj.Name)
// 		return err
// 	}
//
// 	if controllerutil.ContainsFinalizer(cmObj, constant.ConfigFinalizerName) {
// 		return nil
// 	}
//
// 	patch := client.MergeFrom(cmObj.DeepCopy())
//
// 	if cmObj.Labels == nil {
// 		cmObj.Labels = map[string]string{}
// 	}
// 	cmObj.Labels[constant.CMConfigurationTypeLabelKey] = constant.ConfigTemplateType
// 	controllerutil.AddFinalizer(cmObj, constant.ConfigFinalizerName)
//
// 	// cmObj.Immutable = &tpl.Spec.Immutable
// 	return cli.Patch(ctx.Ctx, cmObj, patch)
// }

// func deleteConfigMapFinalizer(cli client.Client, ctx intctrlutil.RequestCtx, configSpec appsv1.ComponentConfigSpec) error {
// 	cmObj, err := getConfigMapByTemplateName(cli, ctx, configSpec.TemplateRef, configSpec.Namespace)
// 	if err != nil && apierrors.IsNotFound(err) {
// 		return nil
// 	} else if err != nil {
// 		ctx.Log.Error(err, "failed to get config template cm object!", "configMapName", configSpec.TemplateRef)
// 		return err
// 	}
//
// 	if !controllerutil.ContainsFinalizer(cmObj, constant.ConfigFinalizerName) {
// 		return nil
// 	}
//
// 	patch := client.MergeFrom(cmObj.DeepCopy())
// 	controllerutil.RemoveFinalizer(cmObj, constant.ConfigFinalizerName)
// 	return cli.Patch(ctx.Ctx, cmObj, patch)
// }

// type ConfigTemplateHandler func([]appsv1.ComponentTemplateSpec) (bool, error)
// type componentValidateHandler func(*appsv1.ComponentDefinition) error

// func handleConfigTemplate(object client.Object, handler ConfigTemplateHandler, handler2 ...componentValidateHandler) (bool, error) {
// 	var (
// 		err             error
// 		configTemplates []appsv1.ComponentConfigSpec
// 	)
// 	switch cr := object.(type) {
// 	case *appsv1.ComponentDefinition:
// 		configTemplates, err = getConfigTemplateFromComponentDef(cr, handler2...)
// 	default:
// 		return false, core.MakeError("not support CR type: %v", cr)
// 	}
//
// 	switch {
// 	case err != nil:
// 		return false, err
// 	case len(configTemplates) > 0:
// 		return handler(configTemplates)
// 	default:
// 		return true, nil
// 	}
// }

// func getConfigTemplateFromComponentDef(componentDef *appsv1.ComponentDefinition,
// 	validators ...componentValidateHandler) ([]appsv1.ComponentTemplateSpec, error) {
// 	configTemplates := make([]appsv1.ComponentTemplateSpec, 0)
// 	// For compatibility with the previous lifecycle management of configurationSpec.TemplateRef,
// 	// it is necessary to convert ScriptSpecs to ConfigSpecs,
// 	// ensuring that the script-related configmap is not allowed to be deleted.
// 	configTemplates = append(configTemplates, componentDef.Spec.Scripts...)
// 	if len(componentDef.Spec.Configs) > 0 {
// 		// Check reload configure config template
// 		for _, validator := range validators {
// 			if err := validator(componentDef); err != nil {
// 				return nil, err
// 			}
// 		}
// 	}
// 	return append(configTemplates, componentDef.Spec.Configs...), nil
// }

// func checkComponentTemplateCR(client client.Client, ctx intctrlutil.RequestCtx, obj client.Object) (bool, error) {
// 	handler := func(configSpecs []appsv1.ComponentConfigSpec) (bool, error) {
// 		return validateConfigTemplate(client, ctx, configSpecs)
// 	}
// 	return handleConfigTemplate(obj, handler)
// }

// func updateLabelsByConfigSpec[T generics.Object, PT generics.PObject[T]](cli client.Client, ctx intctrlutil.RequestCtx, obj PT) (bool, error) {
// 	handler := func(configSpecs []appsv1.ComponentConfigSpec) (bool, error) {
// 		patch := client.MergeFrom(PT(obj.DeepCopy()))
// 		configuration.BuildConfigConstraintLabels(obj, configSpecs)
// 		return true, cli.Patch(ctx.Ctx, obj, patch)
// 	}
// 	return handleConfigTemplate(obj, handler)
// }

// func validateConfigTemplate(cli client.Client, ctx intctrlutil.RequestCtx, configSpecs []appsv1.ComponentConfigSpec) (bool, error) {
// 	// validate ConfigTemplate
// 	foundAndCheckConfigSpec := func(configSpec appsv1.ComponentConfigSpec, logger logr.Logger) (*appsv1beta1.ConfigConstraint, error) {
// 		if _, err := getConfigMapByTemplateName(cli, ctx, configSpec.TemplateRef, configSpec.Namespace); err != nil {
// 			logger.Error(err, "failed to get config template cm object!")
// 			return nil, err
// 		}
// 		if configSpec.VolumeName == "" && configuration.InjectEnvEnabled(configSpec) {
// 			return nil, core.MakeError("config template volume name and envFrom is empty!")
// 		}
// 		if configSpec.ConfigConstraintRef == "" {
// 			return nil, nil
// 		}
// 		configKey := client.ObjectKey{
// 			Namespace: "",
// 			Name:      configSpec.ConfigConstraintRef,
// 		}
// 		configObj := &appsv1beta1.ConfigConstraint{}
// 		if err := cli.Get(ctx.Ctx, configKey, configObj); err != nil {
// 			logger.Error(err, "failed to get template cm object!")
// 			return nil, err
// 		}
// 		return configObj, nil
// 	}
//
// 	for _, templateRef := range configSpecs {
// 		logger := ctx.Log.WithValues("templateName", templateRef.Name).WithValues("configMapName", templateRef.TemplateRef)
// 		configConstraint, err := foundAndCheckConfigSpec(templateRef, logger)
// 		if err != nil {
// 			logger.Error(err, "failed to validate config template!")
// 			return false, err
// 		}
// 		if configConstraint == nil || configConstraint.Spec.ReloadAction == nil {
// 			continue
// 		}
// 		if err := cfgcm.ValidateReloadOptions(configConstraint.Spec.ReloadAction, cli, ctx.Ctx); err != nil {
// 			return false, err
// 		}
// 		if !validateConfigConstraintStatus(configConstraint.Status) {
// 			errMsg := fmt.Sprintf("Configuration template CR[%s] status not ready! current status: %s", configConstraint.Name, configConstraint.Status.Phase)
// 			logger.V(1).Info(errMsg)
// 			return false, fmt.Errorf("%s", errMsg)
// 		}
// 	}
// 	return true, nil
// }

// func validateConfigConstraintStatus(ccStatus appsv1beta1.ConfigConstraintStatus) bool {
// 	return ccStatus.Phase == appsv1beta1.CCAvailablePhase
// }

// func updateConfigConstraintStatus(cli client.Client, ctx intctrlutil.RequestCtx, configConstraint *appsv1beta1.ConfigConstraint, phase appsv1beta1.ConfigConstraintPhase) error {
// 	patch := client.MergeFrom(configConstraint.DeepCopy())
// 	configConstraint.Status.Phase = phase
// 	configConstraint.Status.ObservedGeneration = configConstraint.Generation
// 	return cli.Status().Patch(ctx.Ctx, configConstraint, patch)
// }

func createConfigPatch(cfg *corev1.ConfigMap, configRender *parametersv1alpha1.ParameterDrivenConfigRender, paramsDefs map[string]*parametersv1alpha1.ParametersDefinition) (*core.ConfigPatchInfo, bool, error) {
	if configRender == nil || len(configRender.Spec.Configs) == 0 {
		return nil, true, nil
	}
	lastConfig, err := getLastVersionConfig(cfg)
	if err != nil {
		return nil, false, core.WrapError(err, "failed to get last version data. config[%v]", client.ObjectKeyFromObject(cfg))
	}

	patch, restart, err := core.CreateConfigPatch(lastConfig, cfg.Data, configRender.Spec, true)
	if err != nil {
		return nil, false, err
	}
	if !restart {
		restart = cfgcm.NeedRestart(paramsDefs, patch)
	}
	return patch, restart, nil
}

// func updateConfigSchema(cc *appsv1beta1.ConfigConstraint, cli client.Client, ctx context.Context) error {
// 	schema := cc.Spec.ParametersSchema
// 	if schema == nil || schema.CUE == "" {
// 		return nil
// 	}
//
// 	// Because the conversion of cue to openAPISchema is restricted, and the definition of some cue may not be converted into openAPISchema, and won't return error.
// 	openAPISchema, err := openapi.GenerateOpenAPISchema(schema.CUE, schema.TopLevelKey)
// 	if err != nil {
// 		return err
// 	}
// 	if openAPISchema == nil {
// 		return nil
// 	}
// 	if reflect.DeepEqual(openAPISchema, schema.SchemaInJSON) {
// 		return nil
// 	}
//
// 	ccPatch := client.MergeFrom(cc.DeepCopy())
// 	cc.Spec.ParametersSchema.SchemaInJSON = openAPISchema
// 	return cli.Patch(ctx, cc, ccPatch)
// }

func generateReconcileTasks(reqCtx intctrlutil.RequestCtx, componentParameter *parametersv1alpha1.ComponentParameter) []Task {
	tasks := make([]Task, 0, len(componentParameter.Spec.ConfigItemDetails))
	for _, item := range componentParameter.Spec.ConfigItemDetails {
		if status := fromItemStatus(reqCtx, &componentParameter.Status, item, componentParameter.GetGeneration()); status != nil {
			tasks = append(tasks, NewTask(item, status))
		}
	}
	return tasks
}

func fromItemStatus(ctx intctrlutil.RequestCtx, status *parametersv1alpha1.ComponentParameterStatus, item parametersv1alpha1.ConfigTemplateItemDetail, generation int64) *parametersv1alpha1.ConfigTemplateItemDetailStatus {
	if item.ConfigSpec == nil {
		ctx.Log.V(1).WithName(item.Name).Info(fmt.Sprintf("configuration is creating and pass: %s", item.Name))
		return nil
	}
	itemStatus := intctrlutil.GetItemStatus(status, item.Name)
	if itemStatus == nil || itemStatus.Phase == "" {
		ctx.Log.WithName(item.Name).Info(fmt.Sprintf("ComponentParameters cr is creating: %v", item))
		status.ConfigurationItemStatus = append(status.ConfigurationItemStatus, parametersv1alpha1.ConfigTemplateItemDetailStatus{
			Name:           item.Name,
			Phase:          parametersv1alpha1.CInitPhase,
			UpdateRevision: strconv.FormatInt(generation, 10),
		})
		itemStatus = intctrlutil.GetItemStatus(status, item.Name)
	}
	if !isReconcileStatus(itemStatus.Phase) {
		ctx.Log.V(1).WithName(item.Name).Info(fmt.Sprintf("configuration cr is creating or deleting and pass: %v", itemStatus))
		return nil
	}
	return itemStatus
}

func isReconcileStatus(phase parametersv1alpha1.ParameterPhase) bool {
	return phase != "" &&
		phase != parametersv1alpha1.CCreatingPhase &&
		phase != parametersv1alpha1.CDeletingPhase
}

func buildTemplateVars(ctx context.Context, cli client.Reader,
	compDef *appsv1.ComponentDefinition, synthesizedComp *component.SynthesizedComponent) error {
	if compDef != nil && len(compDef.Spec.Vars) > 0 {
		templateVars, _, err := component.ResolveTemplateNEnvVars(ctx, cli, synthesizedComp, compDef.Spec.Vars)
		if err != nil {
			return err
		}
		synthesizedComp.TemplateVars = templateVars
	}
	return nil
}
