/*
Copyright (C) 2022-2024 ApeCloud Co., Ltd

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

// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1beta1 "github.com/apecloud/kubeblocks/apis/apps/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeParametersDescriptions implements ParametersDescriptionInterface
type FakeParametersDescriptions struct {
	Fake *FakeAppsV1beta1
}

var parametersdescriptionsResource = v1beta1.SchemeGroupVersion.WithResource("parametersdescriptions")

var parametersdescriptionsKind = v1beta1.SchemeGroupVersion.WithKind("ParametersDescription")

// Get takes name of the parametersDescription, and returns the corresponding parametersDescription object, and an error if there is any.
func (c *FakeParametersDescriptions) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1beta1.ParametersDescription, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootGetAction(parametersdescriptionsResource, name), &v1beta1.ParametersDescription{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ParametersDescription), err
}

// List takes label and field selectors, and returns the list of ParametersDescriptions that match those selectors.
func (c *FakeParametersDescriptions) List(ctx context.Context, opts v1.ListOptions) (result *v1beta1.ParametersDescriptionList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootListAction(parametersdescriptionsResource, parametersdescriptionsKind, opts), &v1beta1.ParametersDescriptionList{})
	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1beta1.ParametersDescriptionList{ListMeta: obj.(*v1beta1.ParametersDescriptionList).ListMeta}
	for _, item := range obj.(*v1beta1.ParametersDescriptionList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested parametersDescriptions.
func (c *FakeParametersDescriptions) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewRootWatchAction(parametersdescriptionsResource, opts))
}

// Create takes the representation of a parametersDescription and creates it.  Returns the server's representation of the parametersDescription, and an error, if there is any.
func (c *FakeParametersDescriptions) Create(ctx context.Context, parametersDescription *v1beta1.ParametersDescription, opts v1.CreateOptions) (result *v1beta1.ParametersDescription, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootCreateAction(parametersdescriptionsResource, parametersDescription), &v1beta1.ParametersDescription{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ParametersDescription), err
}

// Update takes the representation of a parametersDescription and updates it. Returns the server's representation of the parametersDescription, and an error, if there is any.
func (c *FakeParametersDescriptions) Update(ctx context.Context, parametersDescription *v1beta1.ParametersDescription, opts v1.UpdateOptions) (result *v1beta1.ParametersDescription, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateAction(parametersdescriptionsResource, parametersDescription), &v1beta1.ParametersDescription{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ParametersDescription), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeParametersDescriptions) UpdateStatus(ctx context.Context, parametersDescription *v1beta1.ParametersDescription, opts v1.UpdateOptions) (*v1beta1.ParametersDescription, error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootUpdateSubresourceAction(parametersdescriptionsResource, "status", parametersDescription), &v1beta1.ParametersDescription{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ParametersDescription), err
}

// Delete takes name of the parametersDescription and deletes it. Returns an error if one occurs.
func (c *FakeParametersDescriptions) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewRootDeleteActionWithOptions(parametersdescriptionsResource, name, opts), &v1beta1.ParametersDescription{})
	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeParametersDescriptions) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewRootDeleteCollectionAction(parametersdescriptionsResource, listOpts)

	_, err := c.Fake.Invokes(action, &v1beta1.ParametersDescriptionList{})
	return err
}

// Patch applies the patch and returns the patched parametersDescription.
func (c *FakeParametersDescriptions) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.ParametersDescription, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewRootPatchSubresourceAction(parametersdescriptionsResource, name, pt, data, subresources...), &v1beta1.ParametersDescription{})
	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.ParametersDescription), err
}
