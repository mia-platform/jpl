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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
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

	annotatedWebhook := validatingWebhook.DeepCopy()
	err := SetObjectExplicitDependencies(annotatedWebhook, []ObjectMetadata{
		ObjectMetadataFromUnstructured(deployment),
		ObjectMetadataFromUnstructured(cronjob),
	})
	require.NoError(t, err)

	annotatedDeployment := deployment.DeepCopy()
	err = SetObjectExplicitDependencies(annotatedDeployment, []ObjectMetadata{
		ObjectMetadataFromUnstructured(annotatedWebhook),
	})
	require.NoError(t, err)

	tests := map[string]struct {
		objects              []*unstructured.Unstructured
		expectedGroups       [][]*unstructured.Unstructured
		expectedGraphError   string
		expectedSortingError string
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
		"explicit dependencies": {
			objects: []*unstructured.Unstructured{
				cronjob,
				namespacedCR,
				deployment,
				clusterCRD,
				namespacedCRD,
				clusterCR,
				namespace,
				annotatedWebhook,
			},
			expectedGroups: [][]*unstructured.Unstructured{
				{
					namespace,
					clusterCRD,
					namespacedCRD,
				},
				{
					deployment,
					cronjob,
					clusterCR,
					namespacedCR,
				},
				{
					annotatedWebhook,
				},
			},
		},
		"error in explicit dependencies": {
			objects: []*unstructured.Unstructured{
				cronjob,
				namespacedCR,
				deployment,
				clusterCRD,
				func() *unstructured.Unstructured {
					obj := webhookService.DeepCopy()
					obj.SetAnnotations(map[string]string{
						DependsOnAnnotation: "value",
					})
					return obj
				}(),
			},
			expectedGraphError: "failed to parse object reference",
		},
		"error in missing resource of explicit dependencies": {
			objects: []*unstructured.Unstructured{
				annotatedWebhook,
			},
			expectedGraphError: "external dependency from admissionregistration.k8s.io/ValidatingWebhookConfiguration example to apps/Deployment test/nginx, external dependency from admissionregistration.k8s.io/ValidatingWebhookConfiguration example to batch/CronJob test/cronjob",
		},
		"error in sorting graph with cyclical dependencies": {
			objects: []*unstructured.Unstructured{
				annotatedWebhook,
				annotatedDeployment,
				cronjob,
			},
			expectedSortingError: "cyclical dependencies:",
			expectedGroups: [][]*unstructured.Unstructured{
				{
					cronjob,
				},
			},
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			graph, err := NewDependencyGraph(testCase.objects)
			switch len(testCase.expectedGraphError) {
			case 0:
				assert.NoError(t, err)
				assert.NotNil(t, graph)
			default:
				assert.ErrorContains(t, err, testCase.expectedGraphError)
				assert.Nil(t, graph)
				return
			}

			groups, err := graph.SortedResourceGroups()
			switch len(testCase.expectedSortingError) {
			case 0:
				assert.NoError(t, err)
			default:
				assert.ErrorContains(t, err, testCase.expectedSortingError)
			}
			assert.Equal(t, testCase.expectedGroups, groups)
		})
	}
}
