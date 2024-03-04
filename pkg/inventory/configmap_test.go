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
	"testing"
	"time"

	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest/fake"
)

func TestNewConfigMapStore(t *testing.T) {
	name := "test-name"
	namespace := "test-namespace"
	factory := pkgtesting.NewTestClientFactory()
	factory.Client = &fake.RESTClient{}

	store, err := NewConfigMapStore(factory, name, namespace)
	assert.NoError(t, err)
	assert.NotNil(t, store)
	assert.NotNil(t, store.clientset)
	assert.Equal(t, name, store.name)
	assert.Equal(t, namespace, store.namespace)
}

func TestKeyFromObjectMetadata(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
		expectedMetadata []ResourceMetadata
		expectErr        bool
		errMessage       string
	}{
		"parsing data inside config map": {
			name: name,
			body: &corev1.ConfigMap{Data: map[string]string{
				"namespace_pod__Pod":               "",
				"namespace_deploy_apps_Deployment": "",
			}},
			expectedMetadata: []ResourceMetadata{
				{
					Name:      "pod",
					Namespace: "namespace",
					Kind:      "Pod",
				},
				{
					Name:      "deploy",
					Namespace: "namespace",
					Group:     "apps",
					Kind:      "Deployment",
				},
			},
		},
		"empty data in config map": {
			name:             name,
			body:             &corev1.ConfigMap{Data: map[string]string{}},
			expectedMetadata: []ResourceMetadata{},
		},
		"config map without data field": {
			name:             name,
			body:             &corev1.ConfigMap{},
			expectedMetadata: []ResourceMetadata{},
		},
		"missing config map": {
			name:             notFound,
			expectedMetadata: []ResourceMetadata{},
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

			store, err := NewConfigMapStore(factory, testCase.name, namespace)
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
			defer cancel()

			metadata, err := store.Load(ctx)
			if testCase.expectErr {
				require.Error(t, err)
				assert.ErrorContains(t, err, testCase.errMessage)
				return
			}

			assert.NoError(t, err)
			assert.ElementsMatch(t, testCase.expectedMetadata, metadata)
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
		data         []ResourceMetadata
		expectedData map[string]string
		expectedErr  bool
		errMessage   string
	}{
		"save empty confimap": {
			name:         name,
			data:         []ResourceMetadata{},
			expectedData: nil,
		},
		"save single element confimap": {
			name: name,
			data: []ResourceMetadata{
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
			data: []ResourceMetadata{
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
		"save end in error": {
			name:        forbidden,
			data:        []ResourceMetadata{},
			expectedErr: true,
			errMessage:  "failed to save inventory",
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			factory := pkgtesting.NewTestClientFactory()
			var configMap corev1.ConfigMap

			factory.Client = &fake.RESTClient{
				Client: fake.CreateHTTPClient(func(r *http.Request) (*http.Response, error) {
					assert.Equal(t, string(types.ApplyPatchType), r.Header.Get("Content-Type"))
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

			store, err := NewConfigMapStore(factory, testCase.name, namespace)
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
			defer cancel()

			store.savedObjects = testCase.data
			err = store.Save(ctx, "jpl-inventory-test")
			if testCase.expectedErr {
				require.Error(t, err)
				assert.ErrorContains(t, err, testCase.errMessage)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedData, configMap.Data)
		})
	}
}
