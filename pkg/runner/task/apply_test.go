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

	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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

func TestCancelTask(t *testing.T) {
	t.Parallel()
	tf := pkgtesting.NewTestClientFactory().WithNamespace("applytest")
	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(_ *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusUnsupportedMediaType, Header: pkgtesting.DefaultHeaders()}, nil
		}),
	}

	infoFetcher, err := DefaultInfoFetcherBuilder(tf)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.TODO())

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

	err = task.Run(ctx)
	require.Error(t, err)
	assert.ErrorContains(t, err, "context canceled")
}

func TestInfoFetcherBuilderError(t *testing.T) {
	t.Parallel()
	tf := pkgtesting.NewTestClientFactory().WithNamespace("applytest")
	errorMessage := "error during client creation"
	tf.UnstructerdClientFunc = func(_ *meta.RESTMapping) (resource.RESTClient, error) {
		// create a client even if we are testing error handling to check that no requests are made
		client := &fake.RESTClient{
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Client: fake.CreateHTTPClient(func(r *http.Request) (*http.Response, error) {
				assert.FailNow(t, fmt.Sprintf("unexpected request: %#v\n%#v", r.URL, r))
				return nil, nil
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

	err = task.Run(context.TODO())
	require.Error(t, err)
	assert.ErrorContains(t, err, errorMessage)
}

func TestUnsupportedMediaTypeError(t *testing.T) {
	t.Parallel()
	deployPath := "/namespaces/applytest/deployments/nginx"
	tf := pkgtesting.NewTestClientFactory().WithNamespace("applytest")
	applied := 0
	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(r *http.Request) (*http.Response, error) {
			switch path, method := r.URL.Path, r.Method; {
			case method == http.MethodPatch && path == deployPath:
				applied++
				return &http.Response{StatusCode: http.StatusUnsupportedMediaType, Header: pkgtesting.DefaultHeaders()}, nil
			default:
				assert.FailNow(t, fmt.Sprintf("unexpected request: %#v\n%#v", r.URL, r))
			}
			return nil, nil
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

	err = task.Run(context.TODO())
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
		tf.UnstructuredClient = &fake.RESTClient{
			NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
			Client: fake.CreateHTTPClient(func(r *http.Request) (*http.Response, error) {
				require.Equal(t, r.Header.Get("Content-Type"), string(types.ApplyPatchType))
				require.Equal(t, r.URL.Query().Get("force"), "true")
				if testCase.dryRun {
					require.Equal(t, "All", r.URL.Query().Get("dryRun"))
				}
				switch path, method := r.URL.Path, r.Method; {
				case path == "PATCH" && r.URL.Query().Get("dryRun") == "All":
					data, err := io.ReadAll(r.Body)
					require.NoError(t, err)
					return &http.Response{StatusCode: http.StatusOK, Header: pkgtesting.DefaultHeaders(), Body: io.NopCloser(bytes.NewReader(data))}, nil
				case method == http.MethodPatch && path == deployPath:
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
					assert.FailNow(t, fmt.Sprintf("unexpected request: %#v\n%#v", r.URL, r))
				}
				return nil, nil
			}),
		}
		infoFetcher, err := DefaultInfoFetcherBuilder(tf)
		require.NoError(t, err)

		task := &ApplyTask{
			InfoFetcher: infoFetcher,
			Objects:     testCase.resources,
			DryRun:      testCase.dryRun,
		}

		err = task.Run(context.TODO())
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
	deployPath := "/namespaces/applytest/deployments/nginx"
	response := pkgtesting.UnstructuredFromFile(t, deploymentAppliedFilename)
	data, err := runtime.Encode(unstructured.NewJSONFallbackEncoder(codec), response)
	require.NoError(t, err)
	tf := pkgtesting.NewTestClientFactory().WithNamespace("applytest")
	applied := 0
	tf.UnstructuredClient = &fake.RESTClient{
		NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
		Client: fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch path, method := req.URL.Path, req.Method; {
			case method == http.MethodPatch && path == deployPath:
				require.Equal(t, req.Header.Get("Content-Type"), string(types.ApplyPatchType))
				require.Equal(t, req.URL.Query().Get("force"), "true")
				applied++
				bodyRC := io.NopCloser(bytes.NewReader(data))
				return &http.Response{StatusCode: http.StatusOK, Header: pkgtesting.DefaultHeaders(), Body: bodyRC}, nil
			default:
				assert.FailNow(t, fmt.Sprintf("unexpected request: %#v\n%#v", req.URL, req))
			}
			return nil, nil
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

	assert.NoError(t, task.Run(context.TODO()))
	assert.Equal(t, 1, applied, "only one PATCH call is made")
}
