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
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSortedObjects(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		objects []*unstructured.Unstructured
		want    []*unstructured.Unstructured
	}{
		"sort an array": {
			objects: []*unstructured.Unstructured{
				configMapObj,
				deploymentObj,
				namespaceObj,
			},
			want: []*unstructured.Unstructured{
				namespaceObj,
				configMapObj,
				deploymentObj,
			},
		},
		"if already sorted don't change it": {
			objects: []*unstructured.Unstructured{
				namespaceObj,
				configMapObj,
				deploymentObj,
			},
			want: []*unstructured.Unstructured{
				namespaceObj,
				configMapObj,
				deploymentObj,
			},
		},
		"sort equal resource GK with namespaces": {
			objects: []*unstructured.Unstructured{
				configMapObj2,
				configMapObj,
			},
			want: []*unstructured.Unstructured{
				configMapObj,
				configMapObj2,
			},
		},
		"sort different unknown GKs by their group or kind": {
			objects: []*unstructured.Unstructured{
				unknownObj,
				unknownObj2,
				unknownObj3,
			},
			want: []*unstructured.Unstructured{
				unknownObj2,
				unknownObj,
				unknownObj3,
			},
		},
		"sort equal resource GK and namespaces with names": {
			objects: []*unstructured.Unstructured{
				deploymentObj2,
				deploymentObj,
			},
			want: []*unstructured.Unstructured{
				deploymentObj,
				deploymentObj2,
			},
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			sort.Sort(SortableObjects(testCase.objects))
			assert.Equal(t, testCase.want, testCase.objects)
		})
	}
}

var configMapObj = &unstructured.Unstructured{
	Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      "the-map",
			"namespace": "testspace",
		},
	},
}

var configMapObj2 = &unstructured.Unstructured{
	Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      "the-map",
			"namespace": "testspace2",
		},
	},
}

var namespaceObj = &unstructured.Unstructured{
	Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata": map[string]interface{}{
			"name": "testspace",
		},
	},
}

var deploymentObj = &unstructured.Unstructured{
	Object: map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      "testdeployment",
			"namespace": "testspace",
		},
	},
}

var deploymentObj2 = &unstructured.Unstructured{
	Object: map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      "testdeployment2",
			"namespace": "testspace",
		},
	},
}

var unknownObj = &unstructured.Unstructured{
	Object: map[string]interface{}{
		"apiVersion": "example.com/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      "unknown",
			"namespace": "testspace",
		},
	},
}

var unknownObj2 = &unstructured.Unstructured{
	Object: map[string]interface{}{
		"apiVersion": "example.com/v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      "unknown2",
			"namespace": "testspace",
		},
	},
}

var unknownObj3 = &unstructured.Unstructured{
	Object: map[string]interface{}{
		"apiVersion": "unknown.com/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      "unknown3",
			"namespace": "testspace",
		},
	},
}
