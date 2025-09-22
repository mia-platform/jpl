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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestFindCRDs(t *testing.T) {
	t.Parallel()

	testdataFolder := filepath.Join("..", "..", "testdata", "commons")
	deploymentFilename := filepath.Join(testdataFolder, "deployment.yaml")
	namespacedCRDFilename := filepath.Join(testdataFolder, "namespaced-crd.yaml")
	namespacedCRFilename := filepath.Join(testdataFolder, "namespaced-cr.yaml")
	clusterCRDFilename := filepath.Join(testdataFolder, "cluster-crd.yaml")
	clusterCRFilename := filepath.Join(testdataFolder, "cluster-cr.yaml")

	testCases := map[string]struct {
		objs         []*unstructured.Unstructured
		expectedCrds int
	}{
		"no crds": {
			objs: []*unstructured.Unstructured{
				pkgtesting.UnstructuredFromFile(t, deploymentFilename),
			},
		},
		"object and concrete crd": {
			objs: []*unstructured.Unstructured{
				pkgtesting.UnstructuredFromFile(t, deploymentFilename),
				pkgtesting.UnstructuredFromFile(t, namespacedCRFilename),
			},
		},
		"crds mixed in": {
			objs: []*unstructured.Unstructured{
				pkgtesting.UnstructuredFromFile(t, namespacedCRFilename),
				pkgtesting.UnstructuredFromFile(t, namespacedCRDFilename),
				pkgtesting.UnstructuredFromFile(t, deploymentFilename),
				pkgtesting.UnstructuredFromFile(t, clusterCRDFilename),
				pkgtesting.UnstructuredFromFile(t, clusterCRFilename),
			},
			expectedCrds: 2,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			crds := FindCRDs(testCase.objs)
			assert.Len(t, crds, testCase.expectedCrds)
		})
	}
}

func TestIsCRD(t *testing.T) {
	t.Parallel()

	testdataFolder := filepath.Join("..", "..", "testdata", "commons")
	deploymentFilename := filepath.Join(testdataFolder, "deployment.yaml")
	namespaceFilename := filepath.Join(testdataFolder, "namespace.yaml")
	namespacedCRDFilename := filepath.Join(testdataFolder, "namespaced-crd.yaml")
	clusterCRDFilename := filepath.Join(testdataFolder, "cluster-crd.yaml")

	testCases := map[string]struct {
		object *unstructured.Unstructured
		isCRD  bool
	}{
		"ConfigMap is not a CRD": {
			object: pkgtesting.UnstructuredFromFile(t, deploymentFilename),
			isCRD:  false,
		},
		"Namespace is not a CRD": {
			object: pkgtesting.UnstructuredFromFile(t, namespaceFilename),
			isCRD:  false,
		},
		"cluster scope CRD is a CRD": {
			object: pkgtesting.UnstructuredFromFile(t, clusterCRDFilename),
			isCRD:  true,
		},
		"namespaced scope CRD is a CRD": {
			object: pkgtesting.UnstructuredFromFile(t, namespacedCRDFilename),
			isCRD:  true,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			isCRD := IsCRD(testCase.object)
			assert.Equal(t, testCase.isCRD, isCRD)
		})
	}
}

func TestInfo(t *testing.T) {
	t.Parallel()

	testdataFolder := filepath.Join("..", "..", "testdata", "commons")
	deploymentFilename := filepath.Join(testdataFolder, "deployment.yaml")

	resource := pkgtesting.UnstructuredFromFile(t, deploymentFilename)
	info := Info(resource)

	assert.NotSame(t, resource, info.Object)
	assert.Equal(t, info.Name, resource.GetName())
	assert.Equal(t, info.Namespace, resource.GetNamespace())
}
