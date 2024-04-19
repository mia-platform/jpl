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

package resourcereader

import (
	"encoding/json"
	"fmt"

	"github.com/mia-platform/jpl/pkg/resource"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/filters"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// objectsFromReader will create a kio.Pipeline for reading data from a Reader and cast it to a series of Resources
func objectsFromReader(reader kio.Reader) ([]*unstructured.Unstructured, error) {
	var objs []*unstructured.Unstructured

	pipeline := kio.Pipeline{
		Inputs:  []kio.Reader{reader},
		Filters: []kio.Filter{filters.StripCommentsFilter{}, &filters.IsLocalConfig{}},
		Outputs: []kio.Writer{kio.WriterFunc(func(nodes []*yaml.RNode) error {
			for _, node := range nodes {
				data, err := node.MarshalJSON()
				if err != nil {
					return err
				}

				var object map[string]interface{}
				if err = json.Unmarshal(data, &object); err != nil {
					return err
				}

				obj := &unstructured.Unstructured{
					Object: object,
				}
				objs = append(objs, obj)
			}

			return nil
		})},
	}

	if err := pipeline.Execute(); err != nil {
		return nil, err
	}

	return objs, nil
}

// setNamespace will set the namespace property for every Namespaced resource to the value provided
// if is not already set.
// If enforce is set to true we will return an error containing all the resources with mismatched namespace.
func setNamespace(mapper meta.RESTMapper, objs []*unstructured.Unstructured, namespace string, enforce bool) error {
	// if no namespace is passed and we don't have to enforce it or there are no objects,
	// return early and avoid any computation
	if (!enforce && len(namespace) == 0) || len(objs) == 0 {
		return nil
	}

	crds := resource.FindCRDs(objs)
	for _, res := range objs {
		scope, err := resource.Scope(res, mapper, crds)
		if err != nil {
			return err
		}

		objNamespace := res.GetNamespace()
		switch scope {
		case meta.RESTScopeNamespace:
			switch {
			case enforce && objNamespace != "" && objNamespace != namespace:
				return EnforcedNamespaceError{
					EnforcedNamespace: namespace,
					NamespaceFound:    objNamespace,
					ResourceGVK:       res.GroupVersionKind(),
				}
			case objNamespace == "":
				res.SetNamespace(namespace)
			}
		case meta.RESTScopeRoot:
			if objNamespace != "" {
				return fmt.Errorf("resource %q has cluster scope but has namespace set to %q", res.GroupVersionKind(), objNamespace)
			}
		}
	}

	return nil
}
