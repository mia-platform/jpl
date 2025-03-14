// Copyright Mia srl
// SPDX-License-Identifier: Apache-2.0
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resource

import (
	"reflect"

	admregv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
)

var crdGK = apiextv1.SchemeGroupVersion.WithKind(reflect.TypeOf(apiextv1.CustomResourceDefinition{}).Name()).GroupKind()
var namespaceGK = corev1.SchemeGroupVersion.WithKind(reflect.TypeOf(corev1.Namespace{}).Name()).GroupKind()
var validatingGK = admregv1.SchemeGroupVersion.WithKind(reflect.TypeOf(admregv1.ValidatingWebhookConfiguration{}).Name()).GroupKind()
var mutatingGK = admregv1.SchemeGroupVersion.WithKind(reflect.TypeOf(admregv1.MutatingWebhookConfiguration{}).Name()).GroupKind()
var serviceGK = corev1.SchemeGroupVersion.WithKind(reflect.TypeOf(corev1.Service{}).Name()).GroupKind()

// FindCRDs return a new slice containing unstructured data for CRDs contained inside a Resource slice
func FindCRDs(objs []*unstructured.Unstructured) []*unstructured.Unstructured {
	// extract all crds available in the slice to control if any custom resource present is Namespaced, even if
	// the actual definition is not already present on remote
	var crds []*unstructured.Unstructured
	for _, resource := range objs {
		if IsCRD(resource) {
			crds = append(crds, resource)
		}
	}

	return crds
}

// IsCRD return true if the Unstructured contains a CRD kubernetes resource, the check is done via equality
// on GroupKind, it will not validate that the resource is actually a CRD or its version
func IsCRD(obj *unstructured.Unstructured) bool {
	resourceGK := obj.GroupVersionKind().GroupKind()
	return resourceGK == crdGK
}

// MetadataIsCRD return true if the ObjectMetadata contains a CRD kubernetes resource, the check is done via equality
// on GroupKind, it will not validate that the resource is actually a CRD or its version
func MetadataIsCRD(obj ObjectMetadata) bool {
	resourceGK := schema.GroupKind{Group: obj.Group, Kind: obj.Kind}
	return resourceGK == crdGK
}

// IsNamespace return true if the Unstructured contains a Namespace kubernetes resource, the check is done via equality
// on GroupKind, it will not validate that the resource is actually a Namespace or its version
func IsNamespace(obj *unstructured.Unstructured) bool {
	resourceGK := obj.GroupVersionKind().GroupKind()
	return resourceGK == namespaceGK
}

// IsRegistrationWebhook return true if the Unstructured contains a ValidatingWebhookConfiguration
// or MutatingWebhookConfiguration kubernetes resource, the check is done via equality on GroupKind,
// it will not validate that the resource is a valid webhook configuration or its version
func IsRegistrationWebhook(obj *unstructured.Unstructured) bool {
	resourceGK := obj.GroupVersionKind().GroupKind()
	return resourceGK == validatingGK || resourceGK == mutatingGK
}

// servicesMetadataFromWebhook return all the services metadata present inside the webhook configuration
func servicesMetadataFromWebhook(webhook *unstructured.Unstructured) []ObjectMetadata {
	services := make([]ObjectMetadata, 0)
	if webhooks, found, _ := unstructured.NestedSlice(webhook.Object, "webhooks"); found {
		for _, webhook := range webhooks {
			serviceRef, found, _ := unstructured.NestedMap(webhook.(map[string]interface{}), "clientConfig")
			if !found {
				continue
			}
			var clientConfig admregv1.WebhookClientConfig
			if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(serviceRef, &clientConfig, true); err != nil || clientConfig.Service == nil {
				continue
			}

			services = append(services, ObjectMetadata{
				Kind:      serviceGK.Kind,
				Group:     serviceGK.Group,
				Name:      clientConfig.Service.Name,
				Namespace: clientConfig.Service.Namespace,
			})
		}
	}

	return services
}

// Info return a kubernetes resource Info backed by a copy of the unstructured
func Info(obj *unstructured.Unstructured) *resource.Info {
	obj = obj.DeepCopy()

	return &resource.Info{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Object:    obj,
	}
}
