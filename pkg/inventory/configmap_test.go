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

package inventory

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeyFromObjectMetadata(t *testing.T) {
	testCases := map[string]struct {
		resourceMetadata ResourceMetadata
		expectedKey      string
	}{
		"complete metadata": {
			resourceMetadata: ResourceMetadata{
				Name:      "test-name",
				Namespace: "test-namespace",
				Group:     "example.com",
				Kind:      "Example",
			},
			expectedKey: "test-namespace_test-name_example.com_Example",
		},
		"core group": {
			resourceMetadata: ResourceMetadata{
				Name:      "test-name",
				Namespace: "default",
				Group:     "",
				Kind:      "ConfigMap",
			},
			expectedKey: "default_test-name__ConfigMap",
		},
		"cluster resource": {
			resourceMetadata: ResourceMetadata{
				Name:      "test-name",
				Namespace: "",
				Group:     "apiextensions.k8s.io",
				Kind:      "CustomResourceDefinition",
			},
			expectedKey: "_test-name_apiextensions.k8s.io_CustomResourceDefinition",
		},
		"RBAC resource": {
			resourceMetadata: ResourceMetadata{
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
			key := keyFromObjectMetadata(testCase.resourceMetadata)
			assert.Equal(t, testCase.expectedKey, key)
		})
	}
}

func TestParseObjectMetadataFromKey(t *testing.T) {
	testCases := map[string]struct {
		key                      string
		expectedFound            bool
		expectedResourceMetadata ResourceMetadata
	}{
		"correct string": {
			key:           "test-namespace_test-name_example.com_Example",
			expectedFound: true,
			expectedResourceMetadata: ResourceMetadata{
				Name:      "test-name",
				Namespace: "test-namespace",
				Kind:      "Example",
				Group:     "example.com",
			},
		},
		"colon in name and dashes in group": {
			key:           "test-namespace_test__name_dash-example.com_Example",
			expectedFound: true,
			expectedResourceMetadata: ResourceMetadata{
				Name:      "test:name",
				Namespace: "test-namespace",
				Kind:      "Example",
				Group:     "dash-example.com",
			},
		},
		"dashes in namespace": {
			key:           "test__namespace_test-name_example.com_Example",
			expectedFound: false,
			expectedResourceMetadata: ResourceMetadata{
				Name:      "",
				Namespace: "",
				Kind:      "",
				Group:     "",
			},
		},
		"random string": {
			key:           "wrong key",
			expectedFound: false,
			expectedResourceMetadata: ResourceMetadata{
				Name:      "",
				Namespace: "",
				Kind:      "",
				Group:     "",
			},
		},
		"cluster resource namespace": {
			key:           "_system__controller__namespace-controller_rbac.authorization.k8s.io_ClusterRole",
			expectedFound: true,
			expectedResourceMetadata: ResourceMetadata{
				Name:      "system:controller:namespace-controller",
				Namespace: "",
				Kind:      "ClusterRole",
				Group:     "rbac.authorization.k8s.io",
			},
		},
		"number in kind": {
			key:           "test-namespace_test-name_cilium.io_CiliumL2AnnouncementPolicy",
			expectedFound: true,
			expectedResourceMetadata: ResourceMetadata{
				Name:      "test-name",
				Namespace: "test-namespace",
				Kind:      "CiliumL2AnnouncementPolicy",
				Group:     "cilium.io",
			},
		},
		"core group": {
			key:           "test-namespace_test-name__ConfigMap",
			expectedFound: true,
			expectedResourceMetadata: ResourceMetadata{
				Name:      "test-name",
				Namespace: "test-namespace",
				Kind:      "ConfigMap",
				Group:     "",
			},
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			ok, resMeta := parseObjectMetadataFromKey(testCase.key)
			assert.Equal(t, testCase.expectedFound, ok)
			assert.Equal(t, testCase.expectedResourceMetadata, resMeta)
		})
	}
}
