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

	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
)

var crdGK = apiextv1.SchemeGroupVersion.WithKind(reflect.TypeOf(apiextv1.CustomResourceDefinition{}).Name()).GroupKind()
var namespaceGK = corev1.SchemeGroupVersion.WithKind(reflect.TypeOf(corev1.Namespace{}).Name()).GroupKind()

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

// Info return a kubernetes resource Info backed by a copy of the unstructured
func Info(obj *unstructured.Unstructured) resource.Info {
	obj = obj.DeepCopy()

	return resource.Info{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Object:    obj,
	}
}
