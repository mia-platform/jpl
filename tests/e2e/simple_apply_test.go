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
	"context"
	"path/filepath"
	"testing"
	"text/template"

	"github.com/mia-platform/jpl/pkg/resourcereader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestApplyToEmptyNamespace(t *testing.T) {
	inventoryName := "inventory"
	expectedInventoryData := func(namespace string) map[string]string {
		return map[string]string{
			namespace + "_busybox_apps_Deployment": "",
			namespace + "_nginx_apps_Deployment":   "",
		}
	}

	firstApplyFeature := features.New("apply on empty namespace").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Helper()
			factory, store := factoryAndStoreForTesting(t, cfg, inventoryName)

			resourcePath := testdataPathForPath(t, filepath.Join("simple-apply", "first"))
			applyResources(t, factory, store, nil, resourcePath, 2)
			return ctx
		}).
		Assess("inventory after apply exists and is correct", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Helper()
			// check inventory object
			configMap := new(corev1.ConfigMap)
			require.NoError(t, cfg.Client().Resources().Get(ctx, inventoryName, cfg.Namespace(), configMap))

			t.Logf("config found: %s", configMap.Name)
			assert.Equal(t, inventoryName, configMap.Name)
			assert.Equal(t, map[string]string(nil), configMap.GetAnnotations())
			assert.Equal(t, map[string]string(nil), configMap.GetLabels())
			assert.Equal(t, expectedInventoryData(cfg.Namespace()), configMap.Data)
			return ctx
		}).
		Assess("deployments after apply", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Helper()

			deployments := new(appsv1.DeploymentList)
			require.NoError(t, cfg.Client().Resources().WithNamespace(cfg.Namespace()).List(ctx, deployments))
			assert.Equal(t, 2, len(deployments.Items))
			return ctx
		}).
		Feature()

	secondApplyFeature := features.New("apply on previous namespace").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Helper()
			factory, store := factoryAndStoreForTesting(t, cfg, inventoryName)

			resourcePath := testdataPathForPath(t, filepath.Join("simple-apply", "second"))
			applyResources(t, factory, store, nil, resourcePath, 2)
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
		Assess("deployments after apply", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Helper()

			deployments := new(appsv1.DeploymentList)
			require.NoError(t, cfg.Client().Resources().WithNamespace(cfg.Namespace()).List(ctx, deployments))
			assert.Equal(t, 2, len(deployments.Items))

			for _, deployment := range deployments.Items {
				assert.NotNil(t, deployment.ObjectMeta.ManagedFields) // check that the object is managed by server side apply
				if deployment.Name != "nginx" {
					assert.EqualValues(t, deployment.ObjectMeta.Generation, 1) // other deployment must not have changed
					continue
				}

				assert.EqualValues(t, *deployment.Spec.Replicas, 2)        // correct filed value
				assert.EqualValues(t, deployment.ObjectMeta.Generation, 2) // updated generation
			}
			return ctx
		}).
		Feature()

	testenv.Test(t, firstApplyFeature, secondApplyFeature)
}

func TestApplyWithNamespace(t *testing.T) {
	inventoryName := "inventory-with-namespace"
	expectedInventoryData := func(namespace string) map[string]string {
		return map[string]string{
			"_" + namespace + "__Namespace":      "",
			namespace + "_nginx__Service":        "",
			namespace + "_nginx_apps_Deployment": "",
		}
	}

	newNamespaceKey := "jpl.context.namespace.key"

	applyFeature := features.New("apply with namespace").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Helper()
			newNamespace := "new" + cfg.Namespace()

			resourcePath := testdataPathForPath(t, filepath.Join("simple-apply", "namespace", "allresources.yaml"))
			fileData := new(bytes.Buffer)
			// apply resources from buffer create via templating to inject random namespace name
			type templateData struct {
				Namespace string
			}
			tmpl := template.Must(template.ParseFiles(resourcePath))
			err := tmpl.Execute(fileData, templateData{Namespace: newNamespace})
			require.NoError(t, err)

			factory, store := factoryAndStoreForTesting(t, cfg.WithNamespace(newNamespace), inventoryName)
			applyResources(t, factory, store, fileData, resourcereader.StdinPath, 3)
			return context.WithValue(ctx, &newNamespaceKey, newNamespace)
		}).
		Assess("inventory after apply exists and is correct", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Helper()

			testNamespace := ctx.Value(&newNamespaceKey).(string)
			configMap := new(corev1.ConfigMap)
			require.NoError(t, cfg.Client().Resources().Get(ctx, inventoryName, testNamespace, configMap))

			t.Logf("config found: %s", configMap.Name)
			assert.Equal(t, inventoryName, configMap.Name)
			assert.Equal(t, map[string]string(nil), configMap.GetAnnotations())
			assert.Equal(t, map[string]string(nil), configMap.GetLabels())
			assert.Equal(t, expectedInventoryData(testNamespace), configMap.Data)
			return ctx
		}).
		Assess("deployments after apply", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Helper()

			testNamespace := ctx.Value(&newNamespaceKey).(string)
			deployments := new(appsv1.DeploymentList)
			require.NoError(t, cfg.Client().Resources().WithNamespace(testNamespace).List(ctx, deployments))
			assert.Equal(t, 1, len(deployments.Items))
			return ctx
		}).
		Assess("services after apply", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Helper()

			testNamespace := ctx.Value(&newNamespaceKey).(string)
			services := new(corev1.ServiceList)
			require.NoError(t, cfg.Client().Resources().WithNamespace(testNamespace).List(ctx, services))
			assert.Equal(t, 1, len(services.Items))
			return ctx
		}).
		Teardown(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Helper()

			testNamespace := ctx.Value(&newNamespaceKey).(string)
			namespace := new(corev1.Namespace)
			namespace.Name = testNamespace
			assert.NoError(t, cfg.Client().Resources().Delete(ctx, namespace))
			return ctx
		}).
		Feature()

	testenv.Test(t, applyFeature)
}
