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
)

func TestObjectMetadataFromString(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		str                 string
		expectedFound       bool
		expectedObjMetadata ObjectMetadata
	}{
		"correct string": {
			str:           "test-namespace_test-name_example.com_Example",
			expectedFound: true,
			expectedObjMetadata: ObjectMetadata{
				Name:      "test-name",
				Namespace: "test-namespace",
				Kind:      "Example",
				Group:     "example.com",
			},
		},
		"colon in name and dashes in group": {
			str:           "test-namespace_test__name_dash-example.com_Example",
			expectedFound: true,
			expectedObjMetadata: ObjectMetadata{
				Name:      "test:name",
				Namespace: "test-namespace",
				Kind:      "Example",
				Group:     "dash-example.com",
			},
		},
		"dashes in namespace": {
			str:           "test__namespace_test-name_example.com_Example",
			expectedFound: false,
			expectedObjMetadata: ObjectMetadata{
				Name:      "",
				Namespace: "",
				Kind:      "",
				Group:     "",
			},
		},
		"random string": {
			str:           "wrong key",
			expectedFound: false,
			expectedObjMetadata: ObjectMetadata{
				Name:      "",
				Namespace: "",
				Kind:      "",
				Group:     "",
			},
		},
		"cluster resource namespace": {
			str:           "_system__controller__namespace-controller_rbac.authorization.k8s.io_ClusterRole",
			expectedFound: true,
			expectedObjMetadata: ObjectMetadata{
				Name:      "system:controller:namespace-controller",
				Namespace: "",
				Kind:      "ClusterRole",
				Group:     "rbac.authorization.k8s.io",
			},
		},
		"number in kind": {
			str:           "test-namespace_test-name_cilium.io_CiliumL2AnnouncementPolicy",
			expectedFound: true,
			expectedObjMetadata: ObjectMetadata{
				Name:      "test-name",
				Namespace: "test-namespace",
				Kind:      "CiliumL2AnnouncementPolicy",
				Group:     "cilium.io",
			},
		},
		"core group": {
			str:           "test-namespace_test-name__ConfigMap",
			expectedFound: true,
			expectedObjMetadata: ObjectMetadata{
				Name:      "test-name",
				Namespace: "test-namespace",
				Kind:      "ConfigMap",
				Group:     "",
			},
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			ok, resMeta := ObjectMetadataFromString(testCase.str)
			assert.Equal(t, testCase.expectedFound, ok)
			assert.Equal(t, testCase.expectedObjMetadata, resMeta)
		})
	}
}

func TestToString(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		objMetadata ObjectMetadata
		expectedKey string
	}{
		"complete metadata": {
			objMetadata: ObjectMetadata{
				Name:      "test-name",
				Namespace: "test-namespace",
				Group:     "example.com",
				Kind:      "Example",
			},
			expectedKey: "test-namespace_test-name_example.com_Example",
		},
		"core group": {
			objMetadata: ObjectMetadata{
				Name:      "test-name",
				Namespace: "default",
				Group:     "",
				Kind:      "ConfigMap",
			},
			expectedKey: "default_test-name__ConfigMap",
		},
		"cluster resource": {
			objMetadata: ObjectMetadata{
				Name:      "test-name",
				Namespace: "",
				Group:     "apiextensions.k8s.io",
				Kind:      "CustomResourceDefinition",
			},
			expectedKey: "_test-name_apiextensions.k8s.io_CustomResourceDefinition",
		},
		"RBAC resource": {
			objMetadata: ObjectMetadata{
				Name:      "system:controller:namespace-controller",
				Namespace: "",
				Kind:      "ClusterRole",
				Group:     "rbac.authorization.k8s.io",
			},
			expectedKey: "_system__controller__namespace-controller_rbac.authorization.k8s.io_ClusterRole",
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			key := testCase.objMetadata.ToString()
			assert.Equal(t, testCase.expectedKey, key)
		})
	}
}

func TestMetadataFromUnstructured(t *testing.T) {
	t.Parallel()

	deploymentPath := filepath.Join("..", "..", "testdata", "commons", "deployment.yaml")
	metadata := ObjectMetadataFromUnstructured(pkgtesting.UnstructuredFromFile(t, deploymentPath))

	assert.NotEqual(t, emptyMetadata, metadata)
	assert.Equal(t, "Deployment", metadata.Kind)
	assert.Equal(t, "apps", metadata.Group)
	assert.Equal(t, "nginx", metadata.Name)
	assert.Empty(t, metadata.Namespace)
}
