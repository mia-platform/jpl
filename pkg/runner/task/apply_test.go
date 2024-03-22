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

package task

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/mia-platform/jpl/pkg/runner"
	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/rest/fake"
)

var (
	codec = pkgtesting.Codecs.LegacyCodec(pkgtesting.Scheme.PrioritizedVersionsAllGroups()...)
)

var (
	testdataFolder            = "testdata"
	deploymentFilename        = filepath.Join(testdataFolder, "deploy.yaml")
	namespaceFilename         = filepath.Join(testdataFolder, "namespace.yaml")
	deploymentAppliedFilename = filepath.Join(testdataFolder, "deploy-applied.yaml")
	namespaceAppliedFilename  = filepath.Join(testdataFolder, "namespace-applied.yaml")
)

func TestCancelApplyTask(t *testing.T) {
	t.Parallel()

	tf := pkgtesting.NewTestClientFactory().WithNamespace("applytest")
	tf.Client = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusUnsupportedMediaType, Header: pkgtesting.DefaultHeaders()}, nil
		}),
	}

	infoFetcher, err := DefaultInfoFetcherBuilder(tf)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.TODO())
	state := &runner.FakeState{Context: ctx}

	task := &ApplyTask{
		FieldManager: "test",
		InfoFetcher:  infoFetcher,
		Objects: []*unstructured.Unstructured{
			pkgtesting.UnstructuredFromFile(t, deploymentFilename),
		},

		// weird trick, but at least we can see that if tasks are clogged and the parent context is cancelled,
		// tasks will not run call to the remote server
		cancel: cancel,
	}

	task.Cancel()

	err = task.Run(state)
	require.Error(t, err)
	assert.ErrorContains(t, err, "context canceled")
}

func TestInfoFetcherBuilderError(t *testing.T) {
	t.Parallel()
	tf := pkgtesting.NewTestClientFactory().WithNamespace("applytest")
	errorMessage := "error during client creation"
	tf.UnstructuredClientForMappingFunc = func(_ schema.GroupVersion) (resource.RESTClient, error) {
		// create a client even if we are testing error handling to check that no requests are made
		client := &fake.RESTClient{
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Client: fake.CreateHTTPClient(func(r *http.Request) (*http.Response, error) {
				t.Logf("unexpected request: %#v\n%#v", r.URL, r)
				return nil, fmt.Errorf("unexpected request")
			}),
		}
		return client, fmt.Errorf(errorMessage)
	}

	infoFetcher, err := DefaultInfoFetcherBuilder(tf)
	require.NoError(t, err)

	task := &ApplyTask{
		FieldManager: "test",
		InfoFetcher:  infoFetcher,
		Objects: []*unstructured.Unstructured{
			pkgtesting.UnstructuredFromFile(t, deploymentFilename),
		},
	}

	withTimeout, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()
	state := &runner.FakeState{Context: withTimeout}

	err = task.Run(state)
	require.Error(t, err)
	assert.ErrorContains(t, err, errorMessage)
}

func TestUnsupportedMediaTypeError(t *testing.T) {
	t.Parallel()

	deployPath := "/namespaces/applytest/deployments/nginx"
	tf := pkgtesting.NewTestClientFactory().WithNamespace("applytest")
	applied := 0
	tf.Client = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(r *http.Request) (*http.Response, error) {
			switch path, method := r.URL.Path, r.Method; {
			case method == http.MethodPatch && path == deployPath:
				applied++
				return &http.Response{StatusCode: http.StatusUnsupportedMediaType, Header: pkgtesting.DefaultHeaders()}, nil
			default:
				t.Logf("unexpected request: %#v\n%#v", r.URL, r)
				return nil, fmt.Errorf("unexpected request")
			}
		}),
	}

	infoFetcher, err := DefaultInfoFetcherBuilder(tf)
	require.NoError(t, err)

	task := &ApplyTask{
		FieldManager: "test",
		InfoFetcher:  infoFetcher,
		Objects: []*unstructured.Unstructured{
			pkgtesting.UnstructuredFromFile(t, deploymentFilename),
			pkgtesting.UnstructuredFromFile(t, deploymentFilename),
		},
	}

	withTimeout, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()
	state := &runner.FakeState{Context: withTimeout}

	err = task.Run(state)
	assert.Error(t, err)
	assert.ErrorContains(t, err, "server-side apply not available on the server")
	assert.Equal(t, 1, applied, "when error is unsupported media, don't make any request after the first")
}

func TestApplyTask(t *testing.T) {
	t.Parallel()

	deployPath := "/namespaces/applytest/deployments/nginx"
	namespacePath := "/namespaces/applytest"

	testCases := map[string]struct {
		resources       []*unstructured.Unstructured
		resourceApplied int
		expectedErr     string
		dryRun          bool
	}{
		"apply one object": {
			resources: []*unstructured.Unstructured{
				pkgtesting.UnstructuredFromFile(t, deploymentFilename),
			},
			resourceApplied: 1,
		},
		"apply multiple objects": {
			resources: []*unstructured.Unstructured{
				pkgtesting.UnstructuredFromFile(t, deploymentFilename),
				pkgtesting.UnstructuredFromFile(t, namespaceFilename),
			},
			resourceApplied: 2,
		},
		"dry run": {
			resources: []*unstructured.Unstructured{
				pkgtesting.UnstructuredFromFile(t, deploymentFilename),
			},
			resourceApplied: 1,
			dryRun:          true,
		},
	}

	for testName, testCase := range testCases {
		applied := 0
		tf := pkgtesting.NewTestClientFactory().WithNamespace("applytest")
		tf.Client = &fake.RESTClient{
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Client: fake.CreateHTTPClient(func(r *http.Request) (*http.Response, error) {
				require.Equal(t, r.Header.Get("Content-Type"), string(types.ApplyPatchType))
				require.Equal(t, r.URL.Query().Get("force"), "true")
				if testCase.dryRun {
					require.Equal(t, "All", r.URL.Query().Get("dryRun"))
				}
				switch path, method := r.URL.Path, r.Method; {
				case method == http.MethodPatch && path == deployPath && r.URL.Query().Get("dryRun") == "All":
					applied++
					data, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					return &http.Response{StatusCode: http.StatusOK, Header: pkgtesting.DefaultHeaders(), Body: io.NopCloser(bytes.NewReader(data))}, nil
				case method == http.MethodPatch && path == deployPath && r.URL.Query().Get("dryRun") != "All":
					applied++
					response := pkgtesting.UnstructuredFromFile(t, deploymentAppliedFilename)
					data, err := runtime.Encode(unstructured.NewJSONFallbackEncoder(codec), response)
					require.NoError(t, err)
					bodyRC := io.NopCloser(bytes.NewReader(data))
					return &http.Response{StatusCode: http.StatusOK, Header: pkgtesting.DefaultHeaders(), Body: bodyRC}, nil
				case method == http.MethodPatch && path == namespacePath:
					applied++
					response := pkgtesting.UnstructuredFromFile(t, namespaceAppliedFilename)
					data, err := runtime.Encode(unstructured.NewJSONFallbackEncoder(codec), response)
					require.NoError(t, err)
					bodyRC := io.NopCloser(bytes.NewReader(data))
					return &http.Response{StatusCode: http.StatusOK, Header: pkgtesting.DefaultHeaders(), Body: bodyRC}, nil
				default:
					t.Logf("unexpected request: %#v\n%#v", r.URL, r)
					return nil, fmt.Errorf("unexpected request")
				}
			}),
		}
		infoFetcher, err := DefaultInfoFetcherBuilder(tf)
		require.NoError(t, err)

		task := &ApplyTask{
			InfoFetcher: infoFetcher,
			Objects:     testCase.resources,
			DryRun:      testCase.dryRun,
		}

		withTimeout, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
		defer cancel()
		state := &runner.FakeState{Context: withTimeout}

		err = task.Run(state)
		t.Run(testName, func(t *testing.T) {
			assert.Equal(t, testCase.resourceApplied, applied)
			if len(testCase.expectedErr) > 0 {
				assert.ErrorContains(t, err, testCase.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSimpleApplyTask(t *testing.T) {
	t.Parallel()

	deployPath := "/namespaces/applytest/deployments/nginx"
	response := pkgtesting.UnstructuredFromFile(t, deploymentAppliedFilename)
	data, err := runtime.Encode(unstructured.NewJSONFallbackEncoder(codec), response)
	require.NoError(t, err)
	tf := pkgtesting.NewTestClientFactory().WithNamespace("applytest")
	applied := 0
	tf.Client = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(r *http.Request) (*http.Response, error) {
			switch path, method := r.URL.Path, r.Method; {
			case method == http.MethodPatch && path == deployPath:
				require.Equal(t, r.Header.Get("Content-Type"), string(types.ApplyPatchType))
				require.Equal(t, r.URL.Query().Get("force"), "true")
				applied++
				bodyRC := io.NopCloser(bytes.NewReader(data))
				return &http.Response{StatusCode: http.StatusOK, Header: pkgtesting.DefaultHeaders(), Body: bodyRC}, nil
			default:
				t.Logf("unexpected request: %#v\n%#v", r.URL, r)
				return nil, fmt.Errorf("unexpected request")
			}
		}),
	}

	infoFetcher, err := DefaultInfoFetcherBuilder(tf)
	require.NoError(t, err)

	task := &ApplyTask{
		FieldManager: "test",
		InfoFetcher:  infoFetcher,
		Objects: []*unstructured.Unstructured{
			pkgtesting.UnstructuredFromFile(t, deploymentFilename),
		},
	}

	withTimeout, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()
	state := &runner.FakeState{Context: withTimeout}

	assert.NoError(t, task.Run(state))
	assert.Equal(t, 1, applied, "only one PATCH call is made")
}
