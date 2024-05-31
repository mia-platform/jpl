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

package poller

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

func TestNewInformer(t *testing.T) {
	t.Parallel()

	deploymentGVK := appsv1.SchemeGroupVersion.WithKind(reflect.TypeOf(appsv1.Deployment{}).Name())
	tests := map[string]struct {
		mapper       meta.RESTMapper
		resource     InformerResource
		expectErr    bool
		errorMessage string
	}{
		"object with kind in mapper return informer": {
			mapper: fakeRESTMapper(deploymentGVK),
			resource: InformerResource{
				GroupKind: deploymentGVK.GroupKind(),
				Namespace: "",
			},
		},
		"object not in mapper return error": {
			mapper: fakeRESTMapper(),
			resource: InformerResource{
				GroupKind: deploymentGVK.GroupKind(),
				Namespace: "",
			},
			expectErr:    true,
			errorMessage: `no matches for kind "Deployment" in group "apps"`,
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			client := dynamicfake.NewSimpleDynamicClient(pkgtesting.Scheme)
			informerBuilder := NewInfromerBuilder(client, testCase.mapper, 0)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			informer, err := informerBuilder.NewInformer(ctx, testCase.resource)
			if testCase.expectErr {
				assert.Nil(t, informer)
				assert.ErrorContains(t, err, testCase.errorMessage)
				return
			}

			assert.Nil(t, err)
			assert.NotNil(t, informer)
		})
	}
}

func TestNewInformerCalls(t *testing.T) {
	t.Parallel()

	namespace := "test"
	deploymentGVK := appsv1.SchemeGroupVersion.WithKind(reflect.TypeOf(appsv1.Deployment{}).Name())
	mapper := fakeRESTMapper(deploymentGVK)
	tests := map[string]struct {
		setupClient func(*dynamicfake.FakeDynamicClient)
		handleError func(*testing.T, error)
	}{
		"calling list verb": {
			setupClient: func(client *dynamicfake.FakeDynamicClient) {
				client.PrependReactor("list", "deployments", func(action clienttesting.Action) (bool, runtime.Object, error) {
					listAction := action.(clienttesting.ListAction)
					if listAction.GetNamespace() != namespace {
						return true, nil, fmt.Errorf("Received unexpected namespace for request: %q", listAction.GetNamespace())
					}

					return true, nil, apierrors.NewNotFound(action.GetResource().GroupResource(), "")
				})
			},
			handleError: func(t *testing.T, err error) {
				t.Helper()

				assert.Error(t, err)
				switch {
				case apierrors.IsNotFound(err):
					t.Logf("Received expected typed NotFound error: %v", err)
				default:
					assert.Fail(t, fmt.Sprintf("Expected typed NotFound error, but got a different error: %v", err))
				}
			},
		},
		"calling list watch": {
			setupClient: func(client *dynamicfake.FakeDynamicClient) {
				client.PrependWatchReactor("deployments", func(action clienttesting.Action) (bool, watch.Interface, error) {
					return true, nil, apierrors.NewNotFound(action.GetResource().GroupResource(), "")
				})
			},
			handleError: func(t *testing.T, err error) {
				t.Helper()

				assert.Error(t, err)
				switch {
				case apierrors.IsNotFound(err):
					t.Logf("Received expected typed NotFound error: %v", err)
				default:
					assert.Fail(t, fmt.Sprintf("Expected typed NotFound error, but got a different error: %v", err))
				}
			},
		},
	}

	for testName, testCase := range tests {
		t.Run(testName, func(t *testing.T) {
			client := dynamicfake.NewSimpleDynamicClient(pkgtesting.Scheme)
			testCase.setupClient(client)

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			informerResource := InformerResource{
				GroupKind: deploymentGVK.GroupKind(),
				Namespace: namespace,
			}
			informerBuilder := NewInfromerBuilder(client, mapper, 0)
			informer, err := informerBuilder.NewInformer(ctx, informerResource)
			require.Nil(t, err)
			require.NotNil(t, informer)

			err = informer.SetWatchErrorHandler(func(_ *cache.Reflector, err error) {
				testCase.handleError(t, err)
				cancel()
			})
			assert.NoError(t, err)

			_, err = informer.AddEventHandler(cache.ResourceEventHandlerFuncs{})
			assert.NoError(t, err)

			informer.Run()
		})
	}
}

func fakeRESTMapper(gvks ...schema.GroupVersionKind) meta.RESTMapper {
	groupVersions := make([]schema.GroupVersion, 0, len(gvks))
	for _, gvk := range gvks {
		groupVersions = append(groupVersions, gvk.GroupVersion())
	}
	mapper := meta.NewDefaultRESTMapper(groupVersions)
	for _, gvk := range gvks {
		mapper.Add(gvk, meta.RESTScopeNamespace)
	}
	return mapper
}
