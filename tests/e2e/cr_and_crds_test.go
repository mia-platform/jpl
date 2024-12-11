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
	"context"
	"os"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestApplyExistingCR(t *testing.T) {
	inventoryName := "inventory"
	expectedFirstInventoryData := func(namespace string) map[string]string {
		return map[string]string{
			namespace + "_image-pull__Secret":              "",
			namespace + "_nginx-config__ConfigMap":         "",
			namespace + "_nginx__Service":                  "",
			namespace + "_nginx_apps_Deployment":           "",
			namespace + "_nginx_traefik.io_IngressRoute":   "",
			namespace + "_service-account__ServiceAccount": "",
		}
	}

	expectedUpdatedInventoryData := func(namespace string) map[string]string {
		return map[string]string{
			namespace + "_image-pull__Secret":              "",
			namespace + "_nginx-config__ConfigMap":         "",
			namespace + "_nginx__Service":                  "",
			namespace + "_nginx_apps_Deployment":           "",
			namespace + "_service-account__ServiceAccount": "",
		}
	}

	applyFeature := features.New("apply on empty namespace").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Helper()
			factory, store := factoryAndStoreForTesting(t, cfg, "inventory")

			resourcePath := testdataPathForPath(t, filepath.Join("complete-suite", "apply-first"))
			applyResources(t, factory, store, nil, resourcePath, 6)
			return ctx
		}).
		Assess("inventory after apply exists and is correct", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Helper()

			configMap := new(corev1.ConfigMap)
			require.NoError(t, cfg.Client().Resources().Get(ctx, inventoryName, cfg.Namespace(), configMap))

			t.Logf("config found: %s", configMap.Name)
			assert.Equal(t, inventoryName, configMap.Name)
			assert.Equal(t, map[string]string(nil), configMap.GetAnnotations())
			assert.Equal(t, map[string]string(nil), configMap.GetLabels())
			assert.Equal(t, expectedFirstInventoryData(cfg.Namespace()), configMap.Data)
			return ctx
		}).
		Assess("control that all resources are deployed", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Helper()

			require.NoError(t, cfg.Client().Resources().Get(ctx, "service-account", cfg.Namespace(), new(corev1.ServiceAccount)))
			require.NoError(t, cfg.Client().Resources().Get(ctx, "image-pull", cfg.Namespace(), new(corev1.Secret)))
			require.NoError(t, cfg.Client().Resources().Get(ctx, "nginx-config", cfg.Namespace(), new(corev1.ConfigMap)))
			require.NoError(t, cfg.Client().Resources().Get(ctx, "nginx", cfg.Namespace(), new(corev1.Service)))
			require.NoError(t, cfg.Client().Resources().Get(ctx, "nginx", cfg.Namespace(), new(appsv1.Deployment)))
			ingressRoute := &unstructured.Unstructured{}
			ingressRoute.SetGroupVersionKind(schema.GroupVersionKind{
				Kind:    "IngressRoute",
				Group:   "traefik.io",
				Version: "v1alpha1",
			})
			require.NoError(t, cfg.Client().Resources().Get(ctx, "nginx", cfg.Namespace(), ingressRoute))
			return ctx
		}).
		Feature()

	updateFeature := features.New("apply update manifests").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Helper()
			factory, store := factoryAndStoreForTesting(t, cfg, "inventory")

			resourcePath := testdataPathForPath(t, filepath.Join("complete-suite", "apply-update"))
			applyResources(t, factory, store, nil, resourcePath, 5)
			return ctx
		}).
		Assess("inventory after apply exists and is correct", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Helper()

			configMap := new(corev1.ConfigMap)
			require.NoError(t, cfg.Client().Resources().Get(ctx, inventoryName, cfg.Namespace(), configMap))

			t.Logf("config found: %s", configMap.Name)
			assert.Equal(t, inventoryName, configMap.Name)
			assert.Equal(t, map[string]string(nil), configMap.GetAnnotations())
			assert.Equal(t, map[string]string(nil), configMap.GetLabels())
			assert.Equal(t, expectedUpdatedInventoryData(cfg.Namespace()), configMap.Data)
			return ctx
		}).
		Assess("resources in inventory exists", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Helper()

			require.NoError(t, cfg.Client().Resources().Get(ctx, "service-account", cfg.Namespace(), new(corev1.ServiceAccount)))
			require.NoError(t, cfg.Client().Resources().Get(ctx, "image-pull", cfg.Namespace(), new(corev1.Secret)))
			require.NoError(t, cfg.Client().Resources().Get(ctx, "nginx-config", cfg.Namespace(), new(corev1.ConfigMap)))
			require.NoError(t, cfg.Client().Resources().Get(ctx, "nginx", cfg.Namespace(), new(corev1.Service)))
			require.NoError(t, cfg.Client().Resources().Get(ctx, "nginx", cfg.Namespace(), new(appsv1.Deployment)))

			return ctx
		}).
		Assess("check deletion of ingress route", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Helper()

			ingressRoutes := &unstructured.UnstructuredList{}
			ingressRoutes.SetGroupVersionKind(schema.GroupVersionKind{
				Kind:    "IngressRouteList",
				Group:   "traefik.io",
				Version: "v1alpha1",
			})
			assert.NoError(t, cfg.Client().Resources().WithNamespace(cfg.Namespace()).List(ctx, ingressRoutes))
			if len(ingressRoutes.Items) > 0 {
				assert.NotNil(t, ingressRoutes.Items[0].GetDeletionTimestamp())
			}

			return ctx
		}).
		Feature()

	testenv.Test(t, applyFeature, updateFeature)
}

func TestCRDAndCR(t *testing.T) {
	inventoryName := "inventory"
	expectedInventoryData := func(namespace string) map[string]string {
		return map[string]string{
			"_replicas.example.com_apiextensions.k8s.io_CustomResourceDefinition": "",
			namespace + "_replica-test_example.com_Replica":                       "",
		}
	}
	applyFeature := features.New("apply on empty namespace").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Helper()
			factory, store := factoryAndStoreForTesting(t, cfg, "inventory")

			resourcePath := testdataPathForPath(t, filepath.Join("complete-suite", "apply-cluster"))
			type templateData struct {
				Namespace string
			}

			tmpDir := t.TempDir()
			tmpl := template.Must(template.ParseFS(os.DirFS(resourcePath), "*.yaml"))
			for _, template := range tmpl.Templates() {
				file, err := os.Create(filepath.Join(tmpDir, template.Name()))
				require.NoError(t, err)
				require.NoError(t, template.Execute(file, templateData{Namespace: cfg.Namespace()}))
				require.NoError(t, file.Close())
			}

			applyResources(t, factory, store, nil, tmpDir, 2)
			return ctx
		}).
		Assess("inventory after apply exists and is correct", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Helper()

			configMap := new(corev1.ConfigMap)
			require.NoError(t, cfg.Client().Resources().Get(ctx, inventoryName, cfg.Namespace(), configMap))

			t.Logf("config found: %s", configMap.Name)
			assert.Equal(t, inventoryName, configMap.Name)
			assert.Equal(t, map[string]string(nil), configMap.GetAnnotations())
			assert.Equal(t, map[string]string(nil), configMap.GetLabels())
			assert.Equal(t, expectedInventoryData(cfg.Namespace()), configMap.Data)
			return ctx
		}).
		Assess("control that all resources are deployed", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Helper()

			require.NoError(t, cfg.Client().Resources().Get(ctx, "replicas.example.com", cfg.Namespace(), new(apiextv1.CustomResourceDefinition)))

			cr := &unstructured.Unstructured{}
			cr.SetGroupVersionKind(schema.GroupVersionKind{
				Kind:    "Replica",
				Group:   "example.com",
				Version: "v1",
			})
			require.NoError(t, cfg.Client().Resources().Get(ctx, "replica-test", cfg.Namespace(), cr))
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Helper()
			cr := &unstructured.Unstructured{}
			cr.SetGroupVersionKind(schema.GroupVersionKind{
				Kind:    "Replica",
				Group:   "example.com",
				Version: "v1",
			})
			cr.SetName("replica-test")
			cr.SetNamespace(cfg.Namespace())
			assert.NoError(t, cfg.Client().Resources().Delete(ctx, cr))

			crd := new(apiextv1.CustomResourceDefinition)
			crd.Name = "replicas.example.com"
			assert.NoError(t, cfg.Client().Resources().Delete(ctx, crd))

			return ctx
		}).
		Feature()

	testenv.Test(t, applyFeature)
}
