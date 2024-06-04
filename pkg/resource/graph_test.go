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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNewDependencyGraph(t *testing.T) {
	t.Parallel()

	testdata := "../../testdata/commons"
	namespace := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "namespace.yaml"))

	deployment := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "deployment.yaml"))
	deployment.SetNamespace(namespace.GetName())

	cronjob := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "cronjob.yaml"))
	cronjob.SetNamespace(namespace.GetName())

	clusterCRD := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "cluster-crd.yaml"))
	clusterCR := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "cluster-cr.yaml"))

	namespacedCRD := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "namespaced-crd.yaml"))

	namespacedCR := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "namespaced-cr.yaml"))
	namespacedCR.SetNamespace(namespace.GetName())

	validatingWebhook := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "validating-webhook-configuration.yaml"))
	mutatingWebhook := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "mutating-webhook-configuration.yaml"))
	webhookService := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "webhook-service.yaml"))

	tests := map[string]struct {
		objects        []*unstructured.Unstructured
		expectedGroups [][]*unstructured.Unstructured
	}{
		"empty objects": {
			expectedGroups: make([][]*unstructured.Unstructured, 0),
		},
		"objects without crds or namesapaces": {
			objects: []*unstructured.Unstructured{
				cronjob,
				validatingWebhook,
				namespacedCR,
				deployment,
			},
			expectedGroups: [][]*unstructured.Unstructured{
				{
					deployment,
					cronjob,
					validatingWebhook,
					namespacedCR,
				},
			},
		},
		"objects with their namespace resource": {
			objects: []*unstructured.Unstructured{
				cronjob,
				namespacedCR,
				deployment,
				namespace,
			},
			expectedGroups: [][]*unstructured.Unstructured{
				{
					namespace,
				},
				{
					deployment,
					cronjob,
					namespacedCR,
				},
			},
		},
		"objects with crds inside": {
			objects: []*unstructured.Unstructured{
				cronjob,
				namespacedCR,
				deployment,
				clusterCRD,
				namespacedCRD,
				clusterCR,
				namespace,
				validatingWebhook,
			},
			expectedGroups: [][]*unstructured.Unstructured{
				{
					namespace,
					clusterCRD,
					namespacedCRD,
					validatingWebhook,
				},
				{
					deployment,
					cronjob,
					clusterCR,
					namespacedCR,
				},
			},
		},
		"only crd, cr and namespace": {
			objects: []*unstructured.Unstructured{
				namespacedCR,
				namespacedCRD,
				namespace,
			},
			expectedGroups: [][]*unstructured.Unstructured{
				{
					namespace,
					namespacedCRD,
				},
				{
					namespacedCR,
				},
			},
		},
		"webhook and its service": {
			objects: []*unstructured.Unstructured{
				validatingWebhook,
				webhookService,
				mutatingWebhook,
			},
			expectedGroups: [][]*unstructured.Unstructured{
				{
					webhookService,
				},
				{
					mutatingWebhook,
					validatingWebhook,
				},
			},
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			graph := NewDependencyGraph(testCase.objects)
			require.NotNil(t, graph)

			groups := graph.SortedResourceGroups()
			assert.Equal(t, testCase.expectedGroups, groups)
		})
	}
}
