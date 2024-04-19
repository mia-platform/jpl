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
	"fmt"
	"os"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Scope will lookup the resource against the provided mapper for getting its scope. If the mapper return
// a resource type not found error, we will search inside the additionalCRDs Unstructured objects.
// If no addtionalCRDs are passed or the type is not found even in those objects we return an UnknownResourceTypesError.
func Scope(obj *unstructured.Unstructured, mapper meta.RESTMapper, addtionalCRDs []*unstructured.Unstructured) (meta.RESTScope, error) {
	gvk := obj.GroupVersionKind()

	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err == nil {
		return mapping.Scope, nil
	}

	if !meta.IsNoMatchError(err) {
		return nil, err
	}

	// if remote lookup return withour result, try to found on additionalCRDs resources
	for _, crd := range addtionalCRDs {
		var typedCRD apiextensionsv1.CustomResourceDefinition
		if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(crd.Object, &typedCRD, true); err != nil {
			fmt.Fprintf(os.Stderr, "skipping invalid CRD found: %q", err)
			continue
		}

		crdGK := schema.GroupKind{Group: typedCRD.Spec.Group, Kind: typedCRD.Spec.Names.Kind}
		for _, version := range typedCRD.Spec.Versions {
			if gvk == crdGK.WithVersion(version.Name) {
				switch typedCRD.Spec.Scope {
				case apiextensionsv1.ClusterScoped:
					return meta.RESTScopeRoot, nil
				case apiextensionsv1.NamespaceScoped:
					return meta.RESTScopeNamespace, nil
				}
			}
		}
	}

	return nil, UnknownResourceTypeError{ResourceGVK: gvk}
}
