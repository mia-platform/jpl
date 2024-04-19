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
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest/fake"
)

func TestNewManager(t *testing.T) {
	t.Parallel()

	inventory := testInventory(t, nil)
	manager := NewManager(inventory, nil)
	assert.NotNil(t, manager)
	assert.Equal(t, manager.Inventory, inventory)
	assert.NotNil(t, manager.objectStatuses)
	assert.Equal(t, 0, len(manager.objectStatuses))
}

func TestObjectSatus(t *testing.T) {
	t.Parallel()

	manager := NewManager(testInventory(t, nil), nil)

	testdata := filepath.Join("..", "..", "testdata", "commons")
	deployment := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "deployment.yaml"))
	namespace := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "namespace.yaml"))
	clustercr := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "cluster-cr.yaml"))
	clustercrd := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "cluster-crd.yaml"))

	manager.SetFailedApply(deployment)
	manager.SetFailedDelete(namespace)
	manager.SetSuccessfullApply(clustercr)
	manager.SetSuccessfullDelete(clustercrd)

	assert.Equal(t, 4, len(manager.objectStatuses))
	assert.Equal(t, sets.New(deployment), manager.objectsForStatus(objectStatusApplyFailed))
	assert.Equal(t, sets.New(namespace), manager.objectsForStatus(objectStatusDeleteFailed))
	assert.Equal(t, sets.New(clustercr), manager.objectsForStatus(objectStatusApplySuccessfull))
	assert.Equal(t, sets.New(clustercrd), manager.objectsForStatus(objectStatusDeleteSuccessfull))

	manager.SetSuccessfullApply(deployment)
	assert.Equal(t, 4, len(manager.objectStatuses))
	assert.Equal(t, sets.New[*unstructured.Unstructured](), manager.objectsForStatus(objectStatusApplyFailed))
	assert.Equal(t, sets.New(clustercr, deployment), manager.objectsForStatus(objectStatusApplySuccessfull))
}

func TestSaveConfigMap(t *testing.T) {
	t.Parallel()

	testdata := filepath.Join("..", "..", "testdata", "commons")
	deployment := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "deployment.yaml"))
	cronjob := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "cronjob.yaml"))
	namespace := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "namespace.yaml"))
	namespacedcrd := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "namespaced-crd.yaml"))

	testCases := map[string]struct {
		client          *fake.RESTClient
		currentStatus   map[*unstructured.Unstructured]objectStatus
		startingObjects []*unstructured.Unstructured
		expectErr       bool
		dryRun          bool
	}{
		"save inventory without previous data": {
			client: &fake.RESTClient{
				Client: fake.CreateHTTPClient(func(r *http.Request) (*http.Response, error) {
					switch {
					case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/namespaces/test/configmaps/test":
						data, err := io.ReadAll(r.Body)
						require.NoError(t, err)
						decoder := pkgtesting.Codecs.UniversalDecoder()
						var configMap corev1.ConfigMap
						err = runtime.DecodeInto(decoder, data, &configMap)
						require.NoError(t, err)
						assert.Equal(t, map[string]string{
							"_nginx_apps_Deployment": "",
							"_test__Namespace":       "",
						}, configMap.Data)
						return &http.Response{
							StatusCode: http.StatusNoContent,
							Header:     pkgtesting.DefaultHeaders(),
							Body:       io.NopCloser(bytes.NewBuffer(data)),
						}, nil
					default:
						t.Logf("unexpected request: %#v\n%#v", r.URL, r)
						return nil, fmt.Errorf("no calls are expected here")
					}
				}),
			},
			currentStatus: map[*unstructured.Unstructured]objectStatus{
				deployment:    objectStatusApplySuccessfull,
				cronjob:       objectStatusDeleteSuccessfull,
				namespace:     objectStatusDeleteFailed,
				namespacedcrd: objectStatusApplyFailed,
			},
		},
		"save inventory with previous data": {
			client: &fake.RESTClient{
				Client: fake.CreateHTTPClient(func(r *http.Request) (*http.Response, error) {
					switch {
					case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/namespaces/test/configmaps/test":
						data, err := io.ReadAll(r.Body)
						require.NoError(t, err)
						decoder := pkgtesting.Codecs.UniversalDecoder()
						var configMap corev1.ConfigMap
						err = runtime.DecodeInto(decoder, data, &configMap)
						require.NoError(t, err)
						assert.Equal(t, map[string]string{
							"_nginx_apps_Deployment": "",
							"_test__Namespace":       "",
						}, configMap.Data)
						return &http.Response{
							StatusCode: http.StatusNoContent,
							Header:     pkgtesting.DefaultHeaders(),
							Body:       io.NopCloser(bytes.NewBuffer(data)),
						}, nil
					default:
						t.Logf("unexpected request: %#v\n%#v", r.URL, r)
						return nil, fmt.Errorf("no calls are expected here")
					}
				}),
			},
			currentStatus: map[*unstructured.Unstructured]objectStatus{
				deployment:    objectStatusApplySuccessfull,
				cronjob:       objectStatusDeleteSuccessfull,
				namespace:     objectStatusApplyFailed,
				namespacedcrd: objectStatusApplyFailed,
			},
			startingObjects: []*unstructured.Unstructured{
				pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "namespace.yaml")),
			},
		},
		"dry run save inventory": {
			client: &fake.RESTClient{
				Client: fake.CreateHTTPClient(func(r *http.Request) (*http.Response, error) {
					data, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					assert.Equal(t, "All", r.URL.Query().Get("dryRun"))
					switch {
					case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/namespaces/test/configmaps/test":
						return &http.Response{
							StatusCode: http.StatusNoContent,
							Header:     pkgtesting.DefaultHeaders(),
							Body:       io.NopCloser(bytes.NewBuffer(data)),
						}, nil
					default:
						t.Logf("unexpected request: %#v\n%#v", r.URL, r)
						return nil, fmt.Errorf("no calls are expected here")
					}
				}),
			},
			currentStatus: map[*unstructured.Unstructured]objectStatus{
				deployment: objectStatusApplySuccessfull,
			},
			dryRun: true,
		},
		"failed save": {
			client: &fake.RESTClient{
				Client: fake.CreateHTTPClient(func(r *http.Request) (*http.Response, error) {
					switch {
					case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/namespaces/test/configmaps/test":
						return &http.Response{StatusCode: http.StatusForbidden, Header: pkgtesting.DefaultHeaders()}, nil
					default:
						t.Logf("unexpected request: %#v\n%#v", r.URL, r)
						return nil, fmt.Errorf("no calls are expected here")
					}
				}),
			},
			currentStatus: map[*unstructured.Unstructured]objectStatus{
				cronjob: objectStatusApplySuccessfull,
			},
			expectErr: true,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			factory := pkgtesting.NewTestClientFactory()
			factory.Client = testCase.client
			manager := NewManager(testInventory(t, factory), testCase.startingObjects)
			manager.objectStatuses = testCase.currentStatus
			ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
			defer cancel()
			err := manager.SaveCurrentInventoryState(ctx, testCase.dryRun)

			switch testCase.expectErr {
			case true:
				assert.Error(t, err)
			default:
				assert.NoError(t, err)
			}
		})
	}
}

func TestDeleteConfigMap(t *testing.T) {
	t.Parallel()

	testdata := "testdata"
	deployment := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "deployment.yaml"))
	service := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "service.yaml"))

	testCases := map[string]struct {
		client        *fake.RESTClient
		currentStatus map[*unstructured.Unstructured]objectStatus
		expectErr     bool
		dryRun        bool
	}{
		"delete inventory": {
			client: &fake.RESTClient{
				Client: fake.CreateHTTPClient(func(r *http.Request) (*http.Response, error) {
					switch {
					case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/namespaces/test/configmaps/test":
						return &http.Response{StatusCode: http.StatusNoContent, Header: pkgtesting.DefaultHeaders()}, nil
					default:
						t.Logf("unexpected request: %#v\n%#v", r.URL, r)
						return nil, fmt.Errorf("no calls are expected here")
					}
				}),
			},
			currentStatus: map[*unstructured.Unstructured]objectStatus{
				deployment: objectStatusApplySuccessfull,
			},
		},
		"dry run delete inventory": {
			client: &fake.RESTClient{
				Client: fake.CreateHTTPClient(func(r *http.Request) (*http.Response, error) {
					data, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					decoder := pkgtesting.Codecs.UniversalDecoder()
					deleteOptions := metav1.DeleteOptions{}
					err = runtime.DecodeInto(decoder, data, &deleteOptions)
					require.NoError(t, err)
					assert.Equal(t, []string{"All"}, deleteOptions.DryRun)
					switch {
					case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/namespaces/test/configmaps/test":
						return &http.Response{StatusCode: http.StatusNoContent, Header: pkgtesting.DefaultHeaders()}, nil
					default:
						t.Logf("unexpected request: %#v\n%#v", r.URL, r)
						return nil, fmt.Errorf("no calls are expected here")
					}
				}),
			},
			currentStatus: map[*unstructured.Unstructured]objectStatus{
				deployment: objectStatusApplySuccessfull,
			},
			dryRun: true,
		},
		"missing inventory on remote cluster": {
			client: &fake.RESTClient{
				Client: fake.CreateHTTPClient(func(r *http.Request) (*http.Response, error) {
					switch {
					case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/namespaces/test/configmaps/test":
						return &http.Response{StatusCode: http.StatusNotFound, Header: pkgtesting.DefaultHeaders()}, nil
					default:
						t.Logf("unexpected request: %#v\n%#v", r.URL, r)
						return nil, fmt.Errorf("no calls are expected here")
					}
				}),
			},
			currentStatus: map[*unstructured.Unstructured]objectStatus{
				deployment: objectStatusApplySuccessfull,
			},
		},
		"avoid deletion because failed objects": {
			client: &fake.RESTClient{
				Client: fake.CreateHTTPClient(func(r *http.Request) (*http.Response, error) {
					t.Logf("unexpected request: %#v\n%#v", r.URL, r)
					return nil, fmt.Errorf("no calls are expected here")
				}),
			},
			currentStatus: map[*unstructured.Unstructured]objectStatus{
				deployment: objectStatusDeleteFailed,
				service:    objectStatusApplySuccessfull,
			},
		},
		"failed deletion": {
			client: &fake.RESTClient{
				Client: fake.CreateHTTPClient(func(r *http.Request) (*http.Response, error) {
					switch {
					case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/namespaces/test/configmaps/test":
						return &http.Response{StatusCode: http.StatusForbidden, Header: pkgtesting.DefaultHeaders()}, nil
					default:
						t.Logf("unexpected request: %#v\n%#v", r.URL, r)
						return nil, fmt.Errorf("no calls are expected here")
					}
				}),
			},
			currentStatus: map[*unstructured.Unstructured]objectStatus{
				service: objectStatusApplySuccessfull,
			},
			expectErr: true,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			factory := pkgtesting.NewTestClientFactory()
			factory.Client = testCase.client
			manager := NewManager(testInventory(t, factory), nil)
			manager.objectStatuses = testCase.currentStatus
			ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
			defer cancel()
			err := manager.DeleteRemoteInventoryIfPossible(ctx, testCase.dryRun)

			switch testCase.expectErr {
			case true:
				assert.Error(t, err)
			default:
				assert.NoError(t, err)
			}
		})
	}
}

func testInventory(t *testing.T, factory *pkgtesting.TestClientFactory) Store {
	t.Helper()

	if factory == nil {
		factory = pkgtesting.NewTestClientFactory()
		factory.Client = &fake.RESTClient{}
	}

	inventory, err := NewConfigMapStore(factory, "test", "test", "")
	require.NoError(t, err)

	return inventory
}
