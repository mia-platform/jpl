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
	"path/filepath"
	"testing"

	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestResourceScope(t *testing.T) {
	t.Parallel()

	testdataFolder := filepath.Join("..", "..", "testdata", "commons")
	deploymentFilename := filepath.Join(testdataFolder, "deployment.yaml")
	namespaceFilename := filepath.Join(testdataFolder, "namespace.yaml")
	namespacedCRDFilename := filepath.Join(testdataFolder, "namespaced-crd.yaml")
	namespacedCRFilename := filepath.Join(testdataFolder, "namespaced-cr.yaml")
	clusterCRDFilename := filepath.Join(testdataFolder, "cluster-crd.yaml")
	clusterCRFilename := filepath.Join(testdataFolder, "cluster-cr.yaml")
	unknownFilename := filepath.Join(testdataFolder, "unknown.yaml")

	testCases := map[string]struct {
		resource      *unstructured.Unstructured
		crds          []*unstructured.Unstructured
		expectedScope meta.RESTScope
		expectedErr   error
	}{
		"ConfigMap has scope namespace": {
			resource:      pkgtesting.UnstructuredFromFile(t, deploymentFilename),
			expectedScope: meta.RESTScopeNamespace,
		},
		"Namespace has scope root": {
			resource:      pkgtesting.UnstructuredFromFile(t, namespaceFilename),
			expectedScope: meta.RESTScopeRoot,
		},
		"custom namespaced CRD has scope namespace": {
			resource: pkgtesting.UnstructuredFromFile(t, namespacedCRFilename),
			crds: []*unstructured.Unstructured{
				pkgtesting.UnstructuredFromFile(t, clusterCRDFilename),
				pkgtesting.UnstructuredFromFile(t, namespacedCRDFilename),
			},
			expectedScope: meta.RESTScopeNamespace,
		},
		"custom cluster CRD has scope root": {
			resource: pkgtesting.UnstructuredFromFile(t, clusterCRFilename),
			crds: []*unstructured.Unstructured{
				pkgtesting.UnstructuredFromFile(t, namespacedCRDFilename),
				pkgtesting.UnstructuredFromFile(t, clusterCRDFilename),
			},
			expectedScope: meta.RESTScopeRoot,
		},
		"invalid CRDs are skipped but no error is returned": {
			resource: pkgtesting.UnstructuredFromFile(t, clusterCRDFilename),
			crds: []*unstructured.Unstructured{
				pkgtesting.UnstructuredFromFile(t, deploymentFilename),
				pkgtesting.UnstructuredFromFile(t, clusterCRDFilename),
			},
			expectedScope: meta.RESTScopeRoot,
		},
		"missing resource return error": {
			resource: pkgtesting.UnstructuredFromFile(t, unknownFilename),
			crds: []*unstructured.Unstructured{
				pkgtesting.UnstructuredFromFile(t, namespacedCRDFilename),
			},
			expectedErr: UnknownResourceTypeError{
				ResourceGVK: schema.GroupVersionKind{
					Kind:    "Unknown",
					Group:   "unknown.example.com",
					Version: "v1",
				},
			},
		},
	}

	testFactory := pkgtesting.NewTestClientFactory()
	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			mapper, err := testFactory.ToRESTMapper()
			require.NoError(t, err)

			scope, err := Scope(testCase.resource, mapper, testCase.crds)
			require.ErrorIs(t, err, testCase.expectedErr)
			assert.Equal(t, testCase.expectedScope, scope)
		})
	}
}
