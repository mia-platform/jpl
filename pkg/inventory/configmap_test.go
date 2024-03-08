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

	"github.com/mia-platform/jpl/pkg/resource"
	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest/fake"
)

func TestNewConfigMapStore(t *testing.T) {
	name := "test-name"
	namespace := "test-namespace"
	fieldManager := "jpl-inventory-test"
	factory := pkgtesting.NewTestClientFactory()
	factory.Client = &fake.RESTClient{}

	store, err := NewConfigMapStore(factory, name, namespace, fieldManager)
	assert.NoError(t, err)
	assert.NotNil(t, store)
	assert.IsType(t, &configMapStore{}, store)
	cmStore := store.(*configMapStore)
	assert.NotNil(t, cmStore.clientset)
	assert.Equal(t, name, cmStore.name)
	assert.Equal(t, namespace, cmStore.namespace)
	assert.Equal(t, fieldManager, cmStore.fieldManager)
}

func TestLoad(t *testing.T) {
	t.Parallel()

	name := "test-name"
	notFound := "not-found"
	forbidden := "forbidden"
	namespace := "test-namespace"
	codec := pkgtesting.Codecs.LegacyCodec(pkgtesting.Scheme.PrioritizedVersionsAllGroups()...)

	testCases := map[string]struct {
		name             string
		body             *corev1.ConfigMap
		expectedMetadata sets.Set[resource.ObjectMetadata]
		expectErr        bool
		errMessage       string
	}{
		"parsing data inside config map": {
			name: name,
			body: &corev1.ConfigMap{Data: map[string]string{
				"namespace_pod__Pod":               "",
				"namespace_deploy_apps_Deployment": "",
			}},
			expectedMetadata: sets.New(resource.ObjectMetadata{
				Name:      "pod",
				Namespace: "namespace",
				Kind:      "Pod",
			},
				resource.ObjectMetadata{
					Name:      "deploy",
					Namespace: "namespace",
					Group:     "apps",
					Kind:      "Deployment",
				},
			),
		},
		"empty data in config map": {
			name:             name,
			body:             &corev1.ConfigMap{Data: map[string]string{}},
			expectedMetadata: sets.Set[resource.ObjectMetadata]{},
		},
		"config map without data field": {
			name:             name,
			body:             &corev1.ConfigMap{},
			expectedMetadata: sets.Set[resource.ObjectMetadata]{},
		},
		"missing config map": {
			name:             notFound,
			expectedMetadata: sets.Set[resource.ObjectMetadata]{},
		},
		"error during GET": {
			name:       forbidden,
			expectErr:  true,
			errMessage: "failed to find inventory",
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			factory := pkgtesting.NewTestClientFactory()
			factory.Client = &fake.RESTClient{
				Client: fake.CreateHTTPClient(func(r *http.Request) (*http.Response, error) {
					switch path, method := r.URL.Path, r.Method; {
					case method == http.MethodGet && path == fmt.Sprintf("/api/v1/namespaces/%s/configmaps/%s", namespace, name):
						body := io.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(codec, testCase.body))))
						return &http.Response{StatusCode: http.StatusOK, Header: pkgtesting.DefaultHeaders(), Body: body}, nil
					case method == http.MethodGet && path == fmt.Sprintf("/api/v1/namespaces/%s/configmaps/%s", namespace, notFound):
						return &http.Response{StatusCode: http.StatusNotFound, Header: pkgtesting.DefaultHeaders()}, nil
					case method == http.MethodGet && path == fmt.Sprintf("/api/v1/namespaces/%s/configmaps/%s", namespace, forbidden):
						return &http.Response{StatusCode: http.StatusForbidden, Header: pkgtesting.DefaultHeaders()}, nil
					default:
						t.Logf("unexpected request: %#v\n%#v", r.URL, r)
						return nil, fmt.Errorf("unexpected request")
					}
				}),
			}

			store, err := NewConfigMapStore(factory, testCase.name, namespace, "jpl-inventory-test")
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
			defer cancel()

			metadata, err := store.(*configMapStore).load(ctx)
			if testCase.expectErr {
				require.Error(t, err)
				assert.ErrorContains(t, err, testCase.errMessage)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedMetadata, metadata)
		})
	}
}

func TestSave(t *testing.T) {
	t.Parallel()

	name := "test-name"
	namespace := "test-namespace"
	forbidden := "forbidden"

	testCases := map[string]struct {
		name         string
		data         []resource.ObjectMetadata
		dryRun       bool
		expectedData map[string]string
		expectErr    bool
		errMessage   string
	}{
		"save empty confimap": {
			name:         name,
			data:         []resource.ObjectMetadata{},
			expectedData: nil,
		},
		"save single element confimap": {
			name: name,
			data: []resource.ObjectMetadata{
				{
					Kind:      "Pod",
					Name:      name,
					Namespace: namespace,
				},
			},
			expectedData: map[string]string{
				namespace + "_" + name + "__Pod": "",
			},
		},
		"save multiple element confimap": {
			name: name,
			data: []resource.ObjectMetadata{
				{
					Kind:      "Pod",
					Name:      name,
					Namespace: namespace,
				},
				{
					Kind:      "Deployment",
					Name:      name,
					Namespace: namespace,
				},
			},
			expectedData: map[string]string{
				namespace + "_" + name + "__Deployment": "",
				namespace + "_" + name + "__Pod":        "",
			},
		},
		"save with dryRun": {
			name: name,
			data: []resource.ObjectMetadata{
				{
					Kind:      "Pod",
					Name:      name,
					Namespace: namespace,
				},
			},
			dryRun: true,
			expectedData: map[string]string{
				namespace + "_" + name + "__Pod": "",
			},
		},
		"save end in error": {
			name:       forbidden,
			data:       []resource.ObjectMetadata{},
			expectErr:  true,
			errMessage: "failed to save inventory",
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			factory := pkgtesting.NewTestClientFactory()
			var configMap corev1.ConfigMap

			factory.Client = &fake.RESTClient{
				Client: fake.CreateHTTPClient(func(r *http.Request) (*http.Response, error) {
					assert.Equal(t, string(types.ApplyPatchType), r.Header.Get("Content-Type"))
					switch testCase.dryRun {
					case true:
						assert.Equal(t, "All", r.URL.Query().Get("dryRun"))
					default:
						assert.Equal(t, "", r.URL.Query().Get("dryRun"))
					}
					switch path, method := r.URL.Path, r.Method; {
					case method == http.MethodPatch && path == fmt.Sprintf("/api/v1/namespaces/%s/configmaps/%s", namespace, name):
						data, err := io.ReadAll(r.Body)
						require.NoError(t, err)
						decoder := pkgtesting.Codecs.UniversalDecoder()
						err = runtime.DecodeInto(decoder, data, &configMap)
						require.NoError(t, err)
						return &http.Response{
							StatusCode: http.StatusOK,
							Header:     pkgtesting.DefaultHeaders(),
							Body:       io.NopCloser(bytes.NewBuffer(data)),
						}, nil
					case method == http.MethodPatch && path == fmt.Sprintf("/api/v1/namespaces/%s/configmaps/%s", namespace, forbidden):
						return &http.Response{StatusCode: http.StatusForbidden, Header: pkgtesting.DefaultHeaders()}, nil
					default:
						t.Logf("unexpected request: %#v\n%#v", r.URL, r)
						return nil, fmt.Errorf("unexpected request")
					}
				}),
			}

			store, err := NewConfigMapStore(factory, testCase.name, namespace, "jpl-inventory-test")
			require.NoError(t, err)
			cmStore := store.(*configMapStore)

			ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
			defer cancel()

			cmStore.savedObjects = testCase.data
			err = store.Save(ctx, testCase.dryRun)
			if testCase.expectErr {
				require.Error(t, err)
				assert.ErrorContains(t, err, testCase.errMessage)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedData, configMap.Data)
		})
	}
}

func TestDiff(t *testing.T) {
	t.Parallel()
	codec := pkgtesting.Codecs.LegacyCodec(pkgtesting.Scheme.PrioritizedVersionsAllGroups()...)

	testdata := "testdata"
	inventory := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "inventory.yaml"))
	deployment := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "deployment.yaml"))
	service := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdata, "service.yaml"))

	testCases := map[string]struct {
		client       *http.Client
		objects      []*unstructured.Unstructured
		expectedDiff []resource.ObjectMetadata
		expectErr    bool
	}{
		"no diff found": {
			client: fake.CreateHTTPClient(func(_ *http.Request) (*http.Response, error) {
				body := io.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(codec, inventory))))
				return &http.Response{StatusCode: http.StatusOK, Header: pkgtesting.DefaultHeaders(), Body: body}, nil
			}),
			objects: []*unstructured.Unstructured{
				deployment,
				service,
			},
			expectedDiff: []resource.ObjectMetadata{},
		},
		"diff found": {
			client: fake.CreateHTTPClient(func(_ *http.Request) (*http.Response, error) {
				body := io.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(codec, inventory))))
				return &http.Response{StatusCode: http.StatusOK, Header: pkgtesting.DefaultHeaders(), Body: body}, nil
			}),
			objects: []*unstructured.Unstructured{
				service,
			},
			expectedDiff: []resource.ObjectMetadata{
				{
					Name:      deployment.GetName(),
					Namespace: deployment.GetNamespace(),
					Kind:      deployment.GroupVersionKind().Kind,
					Group:     deployment.GroupVersionKind().Group,
				},
			},
		},
		"no remote state found": {
			client: fake.CreateHTTPClient(func(_ *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusNotFound, Header: pkgtesting.DefaultHeaders()}, nil
			}),
			objects: []*unstructured.Unstructured{
				deployment,
			},
			expectedDiff: []resource.ObjectMetadata{},
		},
		"error in retrieving state": {
			client: fake.CreateHTTPClient(func(_ *http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusForbidden, Header: pkgtesting.DefaultHeaders()}, nil
			}),
			expectErr: true,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			factory := pkgtesting.NewTestClientFactory()
			factory.Client = &fake.RESTClient{
				Client: testCase.client,
			}
			ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
			defer cancel()
			store, err := NewConfigMapStore(factory, "name", "namespace", "field-manager")
			require.NoError(t, err)

			diff, err := store.Diff(ctx, testCase.objects)
			if testCase.expectErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.ElementsMatch(t, testCase.expectedDiff, diff)
		})
	}
}

func TestSetObjects(t *testing.T) {
	t.Parallel()

	testdataFolder := filepath.Join("..", "..", "testdata", "commons")
	deploymentFilename := filepath.Join(testdataFolder, "deployment.yaml")
	startingMetadata := resource.ObjectMetadata{
		Name:      "pod",
		Namespace: "",
		Kind:      "Pod",
	}
	deploymentMetadata := resource.ObjectMetadata{
		Name:      "nginx",
		Namespace: "",
		Kind:      "Deployment",
		Group:     "apps",
	}

	testCases := map[string]struct {
		resource            *unstructured.Unstructured
		startingMetadata    []resource.ObjectMetadata
		expectedObjMetadata []resource.ObjectMetadata
	}{
		"nil starting metatada": {
			resource:            pkgtesting.UnstructuredFromFile(t, deploymentFilename),
			expectedObjMetadata: []resource.ObjectMetadata{deploymentMetadata},
		},
		"empty starting metadata": {
			resource:            pkgtesting.UnstructuredFromFile(t, deploymentFilename),
			startingMetadata:    []resource.ObjectMetadata{},
			expectedObjMetadata: []resource.ObjectMetadata{deploymentMetadata},
		},
		"elements already in metadata": {
			resource:            pkgtesting.UnstructuredFromFile(t, deploymentFilename),
			startingMetadata:    []resource.ObjectMetadata{startingMetadata},
			expectedObjMetadata: []resource.ObjectMetadata{deploymentMetadata},
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			s := &configMapStore{
				savedObjects: testCase.startingMetadata,
			}
			s.SetObjects([]*unstructured.Unstructured{testCase.resource})
			assert.Equal(t, testCase.expectedObjMetadata, s.savedObjects)
		})
	}
}
