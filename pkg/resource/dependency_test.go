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
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestObjectExplicitDependencies(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		object        *unstructured.Unstructured
		expectedSet   []ObjectMetadata
		expectedError string
	}{
		"object without annotations": {
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{},
				},
			},
			expectedSet: make([]ObjectMetadata, 0),
		},
		"object with other annotations": {
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"annotation":                     "value",
							"value":                          DependsOnAnnotation,
							"config.kubernetes.io/depend-on": "value",
						},
					},
				},
			},
			expectedSet: make([]ObjectMetadata, 0),
		},
		"object with annotations": {
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							DependsOnAnnotation: "/namespaces/namespace/kind/name, group/kind/name",
						},
					},
				},
			},
			expectedSet: []ObjectMetadata{
				{
					Kind:      "kind",
					Group:     "",
					Namespace: "namespace",
					Name:      "name",
				},
				{
					Kind:      "kind",
					Group:     "group",
					Namespace: "",
					Name:      "name",
				},
			},
		},
		"object with malformed string": {
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							DependsOnAnnotation: "/namespaces/namespace/kind/name, group/string/namespace/kind/name",
						},
					},
				},
			},
			expectedSet: []ObjectMetadata{
				{
					Kind:      "kind",
					Group:     "",
					Namespace: "namespace",
					Name:      "name",
				},
			},
			expectedError: "unexpected string as namespaced resource: group/string/namespace/kind/name",
		},
		"object with wrong length": {
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							DependsOnAnnotation: "value",
						},
					},
				},
			},
			expectedSet:   make([]ObjectMetadata, 0),
			expectedError: "unexpected field composition: value",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			dependencies, err := ObjectExplicitDependencies(test.object)
			switch len(test.expectedError) {
			case 0:
				assert.NoError(t, err)
			default:
				assert.ErrorContains(t, err, test.expectedError)
			}

			assert.Equal(t, test.expectedSet, dependencies)
		})
	}
}

func TestSetObjectExplicitDependencies(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		object             *unstructured.Unstructured
		dependencies       []ObjectMetadata
		expectedAnnotation string
		expectedError      string
	}{
		"empty dependency set": {
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
		},
		"single metadata": {
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							DependsOnAnnotation: "old value",
						},
					},
				},
			},
			dependencies: []ObjectMetadata{
				{
					Kind:  "kind",
					Group: "group",
					Name:  "name",
				},
			},
			expectedAnnotation: "group/kind/name",
		},
		"multiple metadata": {
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"metadata": map[string]interface{}{
						"annotations": map[string]interface{}{
							"key": "value",
						},
					},
				},
			},
			dependencies: []ObjectMetadata{
				{
					Kind:  "kind",
					Group: "group",
					Name:  "name",
				},
				{
					Kind:      "namespaced",
					Name:      "name",
					Namespace: "test",
				},
			},
			expectedAnnotation: "group/kind/name,/namespaces/test/namespaced/name",
		},
		"malformed metadata missing kind": {
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			dependencies: []ObjectMetadata{
				{
					Kind:  "kind",
					Group: "group",
					Name:  "name",
				},
				{
					Group:     "namespaced",
					Name:      "name",
					Namespace: "test",
				},
			},
			expectedError: "invalid object metadata: missing resource kind",
		},
		"malformed metadata missing name": {
			object: &unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			dependencies: []ObjectMetadata{
				{
					Kind:  "kind",
					Group: "group",
				},
			},
			expectedError: "invalid object metadata: missing resource name",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			err := SetObjectExplicitDependencies(test.object, test.dependencies)
			switch len(test.expectedError) {
			case 0:
				assert.NoError(t, err)
			default:
				assert.ErrorContains(t, err, test.expectedError)
			}
			value, found := test.object.GetAnnotations()[DependsOnAnnotation]
			assert.Equal(t, len(test.expectedAnnotation) != 0, found)
			assert.Equal(t, test.expectedAnnotation, value)
		})
	}
}
