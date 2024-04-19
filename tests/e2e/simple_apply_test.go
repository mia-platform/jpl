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

//go:build conformance

package e2e

import (
	"bytes"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/mia-platform/jpl/pkg/inventory"
	"github.com/mia-platform/jpl/pkg/resourcereader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestApplyToEmptyNamespace(t *testing.T) {
	t.Parallel()

	// prepare test constants
	resourcePath := testdataPathForPath(t, filepath.Join("simple-apply", "first"))
	updatePath := testdataPathForPath(t, filepath.Join("simple-apply", "second"))
	expectedResourcesCount := 2
	namespace := createNamespaceForTesting(t)
	inventoryName := "inventory"
	factory := factoryForTesting(t, &namespace)
	store, err := inventory.NewConfigMapStore(factory, inventoryName, namespace, "jpl-e2e-test")
	require.NoError(t, err)
	envtestListOptions := &client.ListOptions{Namespace: namespace}
	expectedInventoryData := map[string]string{
		namespace + "_busybox_apps_Deployment": "",
		namespace + "_nginx_apps_Deployment":   "",
	}

	// apply on empty namespace
	applyResources(t, factory, store, nil, resourcePath, expectedResourcesCount)
	appliedDeployments := &appsv1.DeploymentList{}
	getResourcesFromEnv(t, appliedDeployments, envtestListOptions)
	assert.Equal(t, expectedResourcesCount, len(appliedDeployments.Items))

	// apply updates to namespace
	applyResources(t, factory, store, nil, updatePath, expectedResourcesCount)
	updatedDeployments := &appsv1.DeploymentList{}
	getResourcesFromEnv(t, updatedDeployments, envtestListOptions)
	assert.Equal(t, expectedResourcesCount, len(updatedDeployments.Items))

	// check that only one object has changed
	assert.NotEqual(t, appliedDeployments.Items, updatedDeployments.Items)
	for _, deployment := range updatedDeployments.Items {
		assert.NotNil(t, deployment.ObjectMeta.ManagedFields) // check that the object is managed by server side apply
		if deployment.Name != "nginx" {
			assert.EqualValues(t, deployment.ObjectMeta.Generation, 1) // other deployment must not have changed
			continue
		}

		assert.EqualValues(t, *deployment.Spec.Replicas, 2)        // correct filed value
		assert.EqualValues(t, deployment.ObjectMeta.Generation, 2) // updated generation
	}

	// check inventory object
	configMap := &corev1.ConfigMap{}
	getResourceFromEnv(t, client.ObjectKey{Namespace: namespace, Name: inventoryName}, configMap, &client.GetOptions{})
	assert.Equal(t, inventoryName, configMap.Name)
	assert.Equal(t, namespace, configMap.Namespace)
	assert.Equal(t, map[string]string(nil), configMap.GetAnnotations())
	assert.Equal(t, map[string]string(nil), configMap.GetLabels())
	assert.Equal(t, expectedInventoryData, configMap.Data)
}

func TestApplyWithNamespace(t *testing.T) {
	t.Parallel()

	// prepare test constants
	resourcePath := testdataPathForPath(t, filepath.Join("simple-apply", "namespace", "allresources.yaml"))
	namespace := namespaceForTesting(t)
	inventoryName := namespace
	factory := factoryForTesting(t, nil)
	fileData := new(bytes.Buffer)
	envtestListOptions := &client.ListOptions{Namespace: namespace}
	store, err := inventory.NewConfigMapStore(factory, namespace, metav1.NamespaceSystem, "jpl-e2e-test")
	require.NoError(t, err)

	expectedInventoryData := map[string]string{
		"_" + namespace + "__Namespace":      "",
		namespace + "_nginx__Service":        "",
		namespace + "_nginx_apps_Deployment": "",
	}

	// apply resources from buffer create via templating to inject random namespace name
	type templateData struct {
		Namespace string
	}
	tmpl := template.Must(template.ParseFiles(resourcePath))
	err = tmpl.Execute(fileData, templateData{Namespace: namespace})
	require.NoError(t, err)
	applyResources(t, factory, store, fileData, resourcereader.StdinPath, 3)

	// check that the deployment is in the correct namespace
	appliedDeployments := &appsv1.DeploymentList{}
	getResourcesFromEnv(t, appliedDeployments, envtestListOptions)
	assert.Equal(t, 1, len(appliedDeployments.Items))

	// check that the service is in the correct namespace
	appliedServices := &corev1.ServiceList{}
	getResourcesFromEnv(t, appliedServices, envtestListOptions)
	assert.Equal(t, 1, len(appliedServices.Items))

	// check inventory object
	configMap := &corev1.ConfigMap{}
	getResourceFromEnv(t, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: inventoryName}, configMap, &client.GetOptions{})
	assert.Equal(t, inventoryName, configMap.Name)
	assert.Equal(t, metav1.NamespaceSystem, configMap.Namespace)
	assert.Equal(t, map[string]string(nil), configMap.GetAnnotations())
	assert.Equal(t, map[string]string(nil), configMap.GetLabels())
	assert.Equal(t, expectedInventoryData, configMap.Data)
}
