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
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/mia-platform/jpl/pkg/resource"
	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
)

func TestSetNamespace(t *testing.T) {
	t.Parallel()

	testdataFolder := filepath.Join("..", "..", "testdata", "commons")
	deploymentFilename := filepath.Join(testdataFolder, "deployment.yaml")
	namespaceFilename := filepath.Join(testdataFolder, "namespace.yaml")
	namespacedCRDFilename := filepath.Join(testdataFolder, "namespaced-crd.yaml")
	namespacedCRFilename := filepath.Join(testdataFolder, "namespaced-cr.yaml")
	clusterCRDFilename := filepath.Join(testdataFolder, "cluster-crd.yaml")
	clusterCRFilename := filepath.Join(testdataFolder, "cluster-cr.yaml")
	unknown := filepath.Join(testdataFolder, "unknown.yaml")

	namespacedNamespaceFilename := "testdata/namespaced-ns.yaml"
	secretFilename := "testdata/namespaced-secret.yaml"

	testNamespace := "new-namespace"
	testCases := map[string]struct {
		objs             []*unstructured.Unstructured
		namespace        string
		enforceNamespace bool

		expectedErr        error
		expectedNamespaces []string
	}{
		"empty objects don't do anything": {
			objs:             []*unstructured.Unstructured{},
			namespace:        testNamespace,
			enforceNamespace: true,
		},
		"empty namespace and unenforced namespace don't do anything": {
			objs: []*unstructured.Unstructured{
				pkgtesting.UnstructuredFromFile(t, deploymentFilename),
			},
			namespace:          "",
			enforceNamespace:   false,
			expectedNamespaces: []string{""},
		},
		"change namespace of namespaced objects": {
			objs: []*unstructured.Unstructured{
				pkgtesting.UnstructuredFromFile(t, deploymentFilename),
				pkgtesting.UnstructuredFromFile(t, namespaceFilename),
			},
			namespace:          testNamespace,
			enforceNamespace:   false,
			expectedNamespaces: []string{testNamespace, ""},
		},
		"recognize custom CRD and use them": {
			objs: []*unstructured.Unstructured{
				pkgtesting.UnstructuredFromFile(t, namespacedCRFilename),
				pkgtesting.UnstructuredFromFile(t, namespacedCRDFilename),
				pkgtesting.UnstructuredFromFile(t, clusterCRDFilename),
				pkgtesting.UnstructuredFromFile(t, clusterCRFilename),
			},
			namespace:          testNamespace,
			enforceNamespace:   false,
			expectedNamespaces: []string{testNamespace, "", "", ""},
		},
		"don't change namespace if present": {
			objs: []*unstructured.Unstructured{
				pkgtesting.UnstructuredFromFile(t, secretFilename),
				pkgtesting.UnstructuredFromFile(t, namespacedCRDFilename),
			},
			namespace:          testNamespace,
			enforceNamespace:   false,
			expectedNamespaces: []string{"secret", ""},
		},
		"unknown resource return error": {
			objs: []*unstructured.Unstructured{
				pkgtesting.UnstructuredFromFile(t, unknown),
			},
			namespace:          testNamespace,
			enforceNamespace:   false,
			expectedNamespaces: []string{""},
			expectedErr: resource.UnknownResourceTypeError{
				ResourceGVK: schema.GroupVersionKind{
					Group:   "unknown.example.com",
					Version: "v1",
					Kind:    "Unknown",
				},
			},
		},
		"cluster resource with namespace return error": {
			objs: []*unstructured.Unstructured{
				pkgtesting.UnstructuredFromFile(t, namespacedNamespaceFilename),
			},
			namespace:          testNamespace,
			enforceNamespace:   false,
			expectedNamespaces: []string{"wrong"},
			expectedErr:        fmt.Errorf("resource %q has cluster scope but has namespace set to \"wrong\"", schema.GroupVersionKind{Kind: "Namespace", Version: "v1"}),
		},
		"enforce namespace return error if resource has different namespace": {
			objs: []*unstructured.Unstructured{
				pkgtesting.UnstructuredFromFile(t, secretFilename),
			},
			namespace:          testNamespace,
			enforceNamespace:   true,
			expectedNamespaces: []string{"secret"},
			expectedErr: EnforcedNamespaceError{
				EnforcedNamespace: testNamespace,
				NamespaceFound:    "secret",
				ResourceGVK: schema.GroupVersionKind{
					Version: "v1",
					Kind:    "Secret",
				},
			},
		},
	}

	mapper, err := pkgtesting.NewTestClientFactory().ToRESTMapper()
	require.NoError(t, err)

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			err := setNamespace(mapper, testCase.objs, testCase.namespace, testCase.enforceNamespace)
			assert.Equal(t, testCase.expectedErr, err)

			var namespaces []string
			for _, obj := range testCase.objs {
				namespaces = append(namespaces, obj.GetNamespace())
			}
			assert.Equal(t, testCase.expectedNamespaces, namespaces)
		})
	}
}
