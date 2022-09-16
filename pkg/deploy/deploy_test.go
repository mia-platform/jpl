// Copyright 2022 Mia srl
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

package deploy

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

const testdata = "testdata/utils/"

func TestNewResources(t *testing.T) {
	t.Run("Read a valid kubernetes resource", func(t *testing.T) {
		filePath := filepath.Join(testdata, "kubernetesersource.yaml")
		actual, err := NewResources(filePath, "default")
		require.Nil(t, err)
		expected := map[string]interface{}{"apiVersion": "v1", "data": map[string]interface{}{"dueKey": "deuValue", "unaKey": "unValue"}, "kind": "ConfigMap", "metadata": map[string]interface{}{"name": "literal", "namespace": "default"}}
		require.Nil(t, err, "Reading a valid k8s file err must be nil")
		require.Equal(t, len(actual), 1, "1 Resource")
		require.Equal(t, actual[0].GroupVersionKind.Kind, "ConfigMap")
		require.EqualValues(t, expected, actual[0].Object.Object, "confimap on disk different")
	})
	t.Run("Read 2 valid kubernetes resource", func(t *testing.T) {
		filePath := filepath.Join(testdata, "tworesources.yaml")
		actual, err := NewResources(filePath, "default")
		expected1 := map[string]interface{}{"apiVersion": "v1", "data": map[string]interface{}{"dueKey": "deuValue", "unaKey": "unValue"}, "kind": "ConfigMap", "metadata": map[string]interface{}{"name": "literal", "namespace": "default"}}
		expected2 := map[string]interface{}{"apiVersion": "v1", "data": map[string]interface{}{"dueKey": "deuValue2", "unaKey": "unValue2"}, "kind": "ConfigMap", "metadata": map[string]interface{}{"name": "literal2", "namespace": "default"}}
		require.Nil(t, err, "Reading two valid k8s file err must be nil")
		require.Equal(t, len(actual), 2, "2 Resource")
		require.Equal(t, actual[0].GroupVersionKind.Kind, "ConfigMap")
		require.Equal(t, actual[1].GroupVersionKind.Kind, "ConfigMap")
		require.EqualValues(t, expected1, actual[0].Object.Object, "confimap 1 on disk different")
		require.EqualValues(t, expected2, actual[1].Object.Object, "confimap 2 on disk different")
	})
	t.Run("Read not standard resource", func(t *testing.T) {
		filePath := filepath.Join(testdata, "non-standard-resource.yaml")
		actual, err := NewResources(filePath, "default")
		expected := map[string]interface{}{"apiVersion": "traefik.containo.us/v1alpha1", "kind": "IngressRoute", "metadata": map[string]interface{}{"name": "ingressroute1", "namespace": "default"}, "spec": map[string]interface{}{"entryPoints": []interface{}{"websecure"}, "routes": []interface{}{}}}
		require.Nil(t, err, "Reading non standard k8s file err must be nil")
		require.Equal(t, len(actual), 1, "1 Resource")
		require.EqualValues(t, expected, actual[0].Object.Object, "even a crd is unstructurable")
	})
	t.Run("Read an invalid kubernetes resource", func(t *testing.T) {
		filePath := filepath.Join(testdata, "invalidresource.yaml")
		_, err := NewResources(filePath, "default")
		require.EqualError(t, err, "resource testdata/utils/invalidresource.yaml: error converting YAML to JSON: yaml: line 3: could not find expected ':'")
	})
}

func TestMakeResources(t *testing.T) {
	testCases := []struct {
		desc       string
		inputFiles []string
		expected   int
	}{
		{
			desc:       "3 valid resources in 2 files",
			inputFiles: []string{"kubernetesersource.yaml", "tworesources.yaml"},
			expected:   3,
		},
		{
			desc:       "resource with ---",
			inputFiles: []string{"configmap-with-minus.yaml"},
			expected:   1,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			var filePath []string
			for _, v := range tC.inputFiles {
				filePath = append(filePath, filepath.Join(testdata, v))
			}
			actual, err := MakeResources(filePath, "default")
			require.Nil(t, err)
			require.Equal(t, tC.expected, len(actual))
		})
	}
}
