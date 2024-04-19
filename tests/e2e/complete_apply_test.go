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
	"os"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/mia-platform/jpl/pkg/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestInventory(t *testing.T) {
	t.Parallel()

	resourcePath := testdataPathForPath(t, filepath.Join("complete-suite", "apply-first"))
	namespace := createNamespaceForTesting(t)
	inventoryName := "inventory"
	factory := factoryForTesting(t, &namespace)
	store, err := inventory.NewConfigMapStore(factory, inventoryName, namespace, "jpl-e2e-test")
	require.NoError(t, err)
	expectedInventoryData := map[string]string{
		namespace + "_image-pull__Secret":              "",
		namespace + "_nginx-config__ConfigMap":         "",
		namespace + "_nginx__Service":                  "",
		namespace + "_nginx_apps_Deployment":           "",
		namespace + "_nginx_traefik.io_IngressRoute":   "",
		namespace + "_service-account__ServiceAccount": "",
	}

	applyResources(t, factory, store, nil, resourcePath, 6)

	// check inventory object
	configMap := &corev1.ConfigMap{}
	getResourceFromEnv(t, client.ObjectKey{Namespace: namespace, Name: inventoryName}, configMap, &client.GetOptions{})
	assert.Equal(t, expectedInventoryData, configMap.Data)

	// control all things are deployed correctly
	getResourceFromEnv(t, client.ObjectKey{Namespace: namespace, Name: "service-account"}, &corev1.ServiceAccount{}, &client.GetOptions{})
	getResourceFromEnv(t, client.ObjectKey{Namespace: namespace, Name: "image-pull"}, &corev1.Secret{}, &client.GetOptions{})
	getResourceFromEnv(t, client.ObjectKey{Namespace: namespace, Name: "nginx"}, &corev1.Service{}, &client.GetOptions{})
	getResourceFromEnv(t, client.ObjectKey{Namespace: namespace, Name: "nginx"}, &appsv1.Deployment{}, &client.GetOptions{})
	ingressRoute := &unstructured.Unstructured{}
	ingressRoute.SetGroupVersionKind(schema.GroupVersionKind{
		Kind:    "IngressRoute",
		Group:   "traefik.io",
		Version: "v1alpha1",
	})
	getResourceFromEnv(t, client.ObjectKey{Namespace: namespace, Name: "nginx"}, ingressRoute, &client.GetOptions{})

	// create a new apply with one broken object, and one missing, the result will must keep the broken one and delete
	// only the missing one, without changing anything else

	updatePath := testdataPathForPath(t, filepath.Join("complete-suite", "apply-update"))
	applyResources(t, factory, store, nil, updatePath, 5)
	expectedUpdatedInventoryData := map[string]string{
		namespace + "_image-pull__Secret":              "",
		namespace + "_nginx-config__ConfigMap":         "",
		namespace + "_nginx__Service":                  "",
		namespace + "_nginx_apps_Deployment":           "",
		namespace + "_service-account__ServiceAccount": "",
	}

	// check inventory object
	getResourceFromEnv(t, client.ObjectKey{Namespace: namespace, Name: inventoryName}, configMap, &client.GetOptions{})
	assert.Equal(t, expectedUpdatedInventoryData, configMap.Data)

	// control all things are deployed correctly
	getResourceFromEnv(t, client.ObjectKey{Namespace: namespace, Name: "service-account"}, &corev1.ServiceAccount{}, &client.GetOptions{})
	getResourceFromEnv(t, client.ObjectKey{Namespace: namespace, Name: "image-pull"}, &corev1.Secret{}, &client.GetOptions{})
	getResourceFromEnv(t, client.ObjectKey{Namespace: namespace, Name: "nginx"}, &corev1.Service{}, &client.GetOptions{})
	getResourceFromEnv(t, client.ObjectKey{Namespace: namespace, Name: "nginx"}, &appsv1.Deployment{}, &client.GetOptions{})
	ingressRouteList := &unstructured.UnstructuredList{}
	ingressRouteList.SetGroupVersionKind(schema.GroupVersionKind{
		Kind:    "IngressRouteList",
		Group:   "traefik.io",
		Version: "v1alpha1",
	})
	getResourcesFromEnv(t, ingressRouteList, &client.ListOptions{Namespace: namespace})
	for _, ir := range ingressRouteList.Items {
		assert.NotNil(t, ir.GetDeletionTimestamp())
	}
}

func TestCRDAndCR(t *testing.T) {
	t.Parallel()

	resourcePath := testdataPathForPath(t, filepath.Join("complete-suite", "apply-cluster"))
	namespace := namespaceForTesting(t)
	inventoryName := namespace
	factory := factoryForTesting(t, nil)
	store, err := inventory.NewConfigMapStore(factory, inventoryName, metav1.NamespaceSystem, "jpl-e2e-test")
	require.NoError(t, err)

	expectedInventoryData := map[string]string{
		"_" + namespace + "__Namespace":                                       "",
		"_replicas.example.com_apiextensions.k8s.io_CustomResourceDefinition": "",
		namespace + "_replica-test_example.com_Replica":                       "",
	}

	// apply resources from buffer create via templating to inject random namespace name
	type templateData struct {
		Namespace string
	}

	tmpDir := t.TempDir()
	tmpl := template.Must(template.ParseFS(os.DirFS(resourcePath), "*.yaml"))
	for _, template := range tmpl.Templates() {
		file, err := os.Create(filepath.Join(tmpDir, template.Name()))
		require.NoError(t, err)
		require.NoError(t, template.Execute(file, templateData{Namespace: namespace}))
		require.NoError(t, file.Close())
	}

	applyResources(t, factory, store, nil, tmpDir, 3)

	// check inventory object
	configMap := &corev1.ConfigMap{}
	objectKey := client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: inventoryName}
	getResourceFromEnv(t, objectKey, configMap, &client.GetOptions{})
	assert.Equal(t, expectedInventoryData, configMap.Data)

	crList := &unstructured.UnstructuredList{}
	crList.SetGroupVersionKind(schema.GroupVersionKind{
		Kind:    "Replica",
		Group:   "example.com",
		Version: "v1",
	})
	getResourcesFromEnv(t, crList, &client.ListOptions{Namespace: namespace})
	t.Log(crList.Items)
	assert.Equal(t, 1, len(crList.Items))
}
