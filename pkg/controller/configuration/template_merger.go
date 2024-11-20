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
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appsv1 "github.com/apecloud/kubeblocks/apis/apps/v1"
	parametersv1alpha1 "github.com/apecloud/kubeblocks/apis/parameters/v1alpha1"
	"github.com/apecloud/kubeblocks/pkg/configuration/core"
	"github.com/apecloud/kubeblocks/pkg/controller/render"
)

type TemplateMerger interface {

	// Merge merges the baseData with the data from the template.
	Merge(baseData map[string]string, updatedData map[string]string) (map[string]string, error)

	// renderTemplate renders the template and returns the data.
	renderTemplate() (map[string]string, error)
}

type mergeContext struct {
	template     appsv1.ConfigTemplateExtension
	configSpec   appsv1.ComponentTemplateSpec
	paramsDefs   []*parametersv1alpha1.ParametersDefinition
	configRender *parametersv1alpha1.ParameterDrivenConfigRender

	templateRender render.TemplateRender
	ctx            context.Context
	client         client.Client
}

func (m *mergeContext) renderTemplate() (map[string]string, error) {
	templateSpec := appsv1.ComponentTemplateSpec{
		// Name:        m.template.Name,
		Namespace:   m.template.Namespace,
		TemplateRef: m.template.TemplateRef,
	}
	configs, err := render.RenderConfigMapTemplate(m.templateRender, templateSpec, m.ctx, m.client)
	if err != nil {
		return nil, err
	}
	if err := validateRenderedData(configs, m.paramsDefs, m.configRender); err != nil {
		return nil, err
	}
	return configs, nil
}

type noneOp struct {
	*mergeContext
}

func (n noneOp) Merge(_ map[string]string, updatedData map[string]string) (map[string]string, error) {
	return updatedData, nil
}

type configPatcher struct {
	*mergeContext
}

type configReplaceMerger struct {
	*mergeContext
}

type configOnlyAddMerger struct {
	*mergeContext
}

func (c *configPatcher) Merge(baseData map[string]string, updatedData map[string]string) (map[string]string, error) {
	if c.configRender == nil {
		return nil, fmt.Errorf("not support patch merge policy")
	}
	configPatch, err := core.TransformConfigPatchFromData(updatedData, c.configRender.Spec)
	if err != nil {
		return nil, err
	}
	if !configPatch.IsModify {
		return baseData, nil
	}

	params := core.GenerateVisualizedParamsList(configPatch, c.configRender.Spec.Configs)
	mergedData := copyMap(baseData)
	for key, patch := range splitParameters(params) {
		v, ok := baseData[key]
		if !ok {
			mergedData[key] = updatedData[key]
			continue
		}
		newConfig, err := core.ApplyConfigPatch([]byte(v), patch, core.ResolveConfigFormat(c.configRender.Spec.Configs, key))
		if err != nil {
			return nil, err
		}
		mergedData[key] = newConfig
	}

	for key, content := range updatedData {
		if _, ok := mergedData[key]; !ok {
			mergedData[key] = content
		}
	}
	return mergedData, err
}

func (c *configReplaceMerger) Merge(baseData map[string]string, updatedData map[string]string) (map[string]string, error) {
	return core.MergeUpdatedConfig(baseData, updatedData), nil
}

func (c *configOnlyAddMerger) Merge(baseData map[string]string, updatedData map[string]string) (map[string]string, error) {
	return nil, core.MakeError("not implemented")
}

func NewTemplateMerger(template appsv1.ConfigTemplateExtension,
	ctx context.Context,
	cli client.Client,
	templateRender render.TemplateRender,
	configSpec appsv1.ComponentTemplateSpec,
	paramsDefs []*parametersv1alpha1.ParametersDefinition,
	configRender *parametersv1alpha1.ParameterDrivenConfigRender,
) (TemplateMerger, error) {
	templateData := &mergeContext{
		configSpec:     configSpec,
		template:       template,
		ctx:            ctx,
		client:         cli,
		templateRender: templateRender,
		paramsDefs:     paramsDefs,
		configRender:   configRender,
	}

	var merger TemplateMerger
	switch template.Policy {
	default:
		return nil, core.MakeError("unknown template policy: %s", template.Policy)
	case appsv1.NoneMergePolicy:
		merger = &noneOp{templateData}
	case appsv1.PatchPolicy:
		merger = &configPatcher{templateData}
	case appsv1.OnlyAddPolicy:
		merger = &configOnlyAddMerger{templateData}
	case appsv1.ReplacePolicy:
		merger = &configReplaceMerger{templateData}
	}
	return merger, nil
}

func mergerConfigTemplate(template appsv1.ConfigTemplateExtension,
	templateRender render.TemplateRender,
	configSpec appsv1.ComponentTemplateSpec,
	baseData map[string]string,
	paramsDefs []*parametersv1alpha1.ParametersDefinition,
	configRender *parametersv1alpha1.ParameterDrivenConfigRender,
	ctx context.Context, cli client.Client) (map[string]string, error) {
	templateMerger, err := NewTemplateMerger(template, ctx, cli, templateRender, configSpec, paramsDefs, configRender)
	if err != nil {
		return nil, err
	}
	data, err := templateMerger.renderTemplate()
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	return templateMerger.Merge(baseData, data)
}

func splitParameters(params []core.VisualizedParam) map[string]map[string]*string {
	r := make(map[string]map[string]*string)
	for _, param := range params {
		if _, ok := r[param.Key]; !ok {
			r[param.Key] = make(map[string]*string)
		}
		for _, kv := range param.Parameters {
			r[param.Key][kv.Key] = kv.Value
		}
	}
	return r
}
