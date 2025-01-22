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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/mia-platform/jpl/pkg/client/cache"
	"github.com/mia-platform/jpl/pkg/event"
	"github.com/mia-platform/jpl/pkg/filter"
	"github.com/mia-platform/jpl/pkg/runner"
	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/rest/fake"
	"k8s.io/client-go/util/csaupgrade"
)

var (
	codec = pkgtesting.Codecs.LegacyCodec(pkgtesting.Scheme.PrioritizedVersionsAllGroups()...)
)

var (
	testdataFolder               = "testdata"
	deploymentFilename           = filepath.Join(testdataFolder, "deploy.yaml")
	namespaceFilename            = filepath.Join(testdataFolder, "namespace.yaml")
	managedFieldsFilename        = filepath.Join(testdataFolder, "managed-fields.yaml")
	deploymentAppliedFilename    = filepath.Join(testdataFolder, "deploy-applied.yaml")
	namespaceAppliedFilename     = filepath.Join(testdataFolder, "namespace-applied.yaml")
	managedFieldsAppliedFilename = filepath.Join(testdataFolder, "managed-fields-applied.yaml")
)

func TestCancelApplyTask(t *testing.T) {
	t.Parallel()

	tf := pkgtesting.NewTestClientFactory().WithNamespace("test")
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

	deployement := pkgtesting.UnstructuredFromFile(t, deploymentFilename)
	expectedEvents := []event.Event{
		{
			Type: event.TypeApply,
			ApplyInfo: event.ApplyInfo{
				Object: deployement,
				Status: event.StatusPending,
			},
		},
		{
			Type: event.TypeApply,
			ApplyInfo: event.ApplyInfo{
				Object: deployement,
				Status: event.StatusFailed,
				Error:  fmt.Errorf("context canceled"),
			},
		},
	}

	task := &ApplyTask{
		FieldManager: "test",
		InfoFetcher:  infoFetcher,
		Objects: []*unstructured.Unstructured{
			deployement,
		},
	}

	cancel()

	task.Run(state)
	require.Equal(t, len(expectedEvents), len(state.SentEvents))
	for idx, expectedEvent := range expectedEvents {
		assert.Equal(t, expectedEvent.String(), state.SentEvents[idx].String())
	}
}

func TestInfoFetcherBuilderError(t *testing.T) {
	t.Parallel()
	tf := pkgtesting.NewTestClientFactory().WithNamespace("test")
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
		return client, errors.New(errorMessage)
	}

	infoFetcher, err := DefaultInfoFetcherBuilder(tf)
	require.NoError(t, err)

	deployment := pkgtesting.UnstructuredFromFile(t, deploymentFilename)
	expectedEvents := []event.Event{
		{
			Type: event.TypeApply,
			ApplyInfo: event.ApplyInfo{
				Object: deployment,
				Status: event.StatusPending,
			},
		},
		{
			Type: event.TypeApply,
			ApplyInfo: event.ApplyInfo{
				Object: deployment,
				Status: event.StatusFailed,
				Error:  fmt.Errorf("error during client creation"),
			},
		},
	}

	task := &ApplyTask{
		FieldManager: "test",
		InfoFetcher:  infoFetcher,
		Objects: []*unstructured.Unstructured{
			deployment,
		},
	}

	withTimeout, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()
	state := &runner.FakeState{Context: withTimeout}

	task.Run(state)
	require.Equal(t, len(expectedEvents), len(state.SentEvents))
	for idx, expectedEvent := range expectedEvents {
		assert.Equal(t, expectedEvent.String(), state.SentEvents[idx].String())
	}
}

func TestUnsupportedMediaTypeError(t *testing.T) {
	t.Parallel()

	deployPath := "/namespaces/test/deployments/nginx"
	tf := pkgtesting.NewTestClientFactory().WithNamespace("test")
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

	deployment := pkgtesting.UnstructuredFromFile(t, deploymentFilename)
	expectedEvents := []event.Event{
		{
			Type: event.TypeApply,
			ApplyInfo: event.ApplyInfo{
				Object: deployment,
				Status: event.StatusPending,
			},
		},
		{
			Type: event.TypeApply,
			ApplyInfo: event.ApplyInfo{
				Object: deployment,
				Status: event.StatusFailed,
				Error:  fmt.Errorf("server-side apply not available on the server: unknown (patch deployments nginx)"),
			},
		},
	}

	task := &ApplyTask{
		FieldManager: "test",
		InfoFetcher:  infoFetcher,
		Objects: []*unstructured.Unstructured{
			pkgtesting.UnstructuredFromFile(t, deploymentFilename),
			pkgtesting.UnstructuredFromFile(t, namespaceFilename),
		},
	}

	withTimeout, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()
	state := &runner.FakeState{Context: withTimeout}

	task.Run(state)
	require.Equal(t, len(expectedEvents), len(state.SentEvents))
	for idx, expectedEvent := range expectedEvents {
		assert.Equal(t, expectedEvent.String(), state.SentEvents[idx].String())
	}
}

func TestApplyTask(t *testing.T) {
	t.Parallel()

	deployPath := "/namespaces/test/deployments/nginx"
	namespacePath := "/namespaces/test"

	deployment := pkgtesting.UnstructuredFromFile(t, deploymentFilename)
	namespace := pkgtesting.UnstructuredFromFile(t, namespaceFilename)

	testCases := map[string]struct {
		resources      []*unstructured.Unstructured
		expectedEvents []event.Event
		filters        []filter.Interface
		dryRun         bool
	}{
		"apply one object": {
			resources: []*unstructured.Unstructured{
				deployment,
			},
			expectedEvents: []event.Event{
				{
					Type: event.TypeApply,
					ApplyInfo: event.ApplyInfo{
						Status: event.StatusPending,
						Object: deployment,
					},
				},
				{
					Type: event.TypeApply,
					ApplyInfo: event.ApplyInfo{
						Status: event.StatusSuccessful,
						Object: deployment,
					},
				},
			},
		},
		"apply multiple objects": {
			resources: []*unstructured.Unstructured{
				deployment,
				namespace,
			},
			expectedEvents: []event.Event{
				{
					Type: event.TypeApply,
					ApplyInfo: event.ApplyInfo{
						Status: event.StatusPending,
						Object: deployment,
					},
				},
				{
					Type: event.TypeApply,
					ApplyInfo: event.ApplyInfo{
						Status: event.StatusSuccessful,
						Object: deployment,
					},
				},
				{
					Type: event.TypeApply,
					ApplyInfo: event.ApplyInfo{
						Status: event.StatusPending,
						Object: namespace,
					},
				},
				{
					Type: event.TypeApply,
					ApplyInfo: event.ApplyInfo{
						Status: event.StatusSuccessful,
						Object: namespace,
					},
				},
			},
			filters: []filter.Interface{
				&testFilter{Kind: "CronJob"},
				&testFilter{Kind: "Pod"},
			},
		},
		"apply object with filters": {
			resources: []*unstructured.Unstructured{
				deployment,
				namespace,
			},
			expectedEvents: []event.Event{
				{
					Type: event.TypeApply,
					ApplyInfo: event.ApplyInfo{
						Status: event.StatusSkipped,
						Object: deployment,
					},
				},
				{
					Type: event.TypeApply,
					ApplyInfo: event.ApplyInfo{
						Status: event.StatusPending,
						Object: namespace,
					},
				},
				{
					Type: event.TypeApply,
					ApplyInfo: event.ApplyInfo{
						Status: event.StatusSuccessful,
						Object: namespace,
					},
				},
			},
			filters: []filter.Interface{
				&testFilter{Kind: "Deployment"},
			},
		},
		"error during filtering": {
			resources: []*unstructured.Unstructured{
				deployment,
				namespace,
			},
			expectedEvents: []event.Event{
				{
					Type: event.TypeApply,
					ApplyInfo: event.ApplyInfo{
						Status: event.StatusSkipped,
						Object: deployment,
					},
				},
				{
					Type: event.TypeApply,
					ApplyInfo: event.ApplyInfo{
						Status: event.StatusFailed,
						Object: namespace,
						Error:  fmt.Errorf("error in filter"),
					},
				},
			},
			filters: []filter.Interface{
				&testFilter{Kind: "Deployment"},
				&testFilter{Error: fmt.Errorf("error in filter")},
			},
		},
		"dry run": {
			resources: []*unstructured.Unstructured{
				deployment,
			},
			expectedEvents: []event.Event{
				{
					Type: event.TypeApply,
					ApplyInfo: event.ApplyInfo{
						Status: event.StatusPending,
						Object: deployment,
					},
				},
				{
					Type: event.TypeApply,
					ApplyInfo: event.ApplyInfo{
						Status: event.StatusSuccessful,
						Object: deployment,
					},
				},
			},
			dryRun: true,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			tf := pkgtesting.NewTestClientFactory().WithNamespace("test")
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
						data, err := io.ReadAll(r.Body)
						require.NoError(t, err)
						return &http.Response{StatusCode: http.StatusOK, Header: pkgtesting.DefaultHeaders(), Body: io.NopCloser(bytes.NewReader(data))}, nil
					case method == http.MethodPatch && path == deployPath && r.URL.Query().Get("dryRun") != "All":
						response := pkgtesting.UnstructuredFromFile(t, deploymentAppliedFilename)
						data, err := runtime.Encode(unstructured.NewJSONFallbackEncoder(codec), response)
						require.NoError(t, err)
						bodyRC := io.NopCloser(bytes.NewReader(data))
						return &http.Response{StatusCode: http.StatusOK, Header: pkgtesting.DefaultHeaders(), Body: bodyRC}, nil
					case method == http.MethodPatch && path == namespacePath:
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
				FieldManager: "test",
				InfoFetcher:  infoFetcher,
				Objects:      testCase.resources,
				Filters:      testCase.filters,
				DryRun:       testCase.dryRun,
			}

			withTimeout, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
			defer cancel()
			state := &runner.FakeState{Context: withTimeout}

			task.Run(state)
			t.Log(state.SentEvents)
			require.Equal(t, len(testCase.expectedEvents), len(state.SentEvents))
			for idx, expectedEvent := range testCase.expectedEvents {
				assert.Equal(t, expectedEvent.String(), state.SentEvents[idx].String())
			}
		})
	}
}

func TestClientSideMigration(t *testing.T) {
	t.Parallel()

	managedFieldsPath := "/namespaces/test/deployments/managed-fields"
	managedFields := pkgtesting.UnstructuredFromFile(t, managedFieldsFilename)
	fieldManager := "test"

	testCases := map[string]struct {
		resources      []*unstructured.Unstructured
		expectedEvents []event.Event
		filters        []filter.Interface
		dryRun         bool
	}{
		"applying resource with client side apply annotation": {
			resources: []*unstructured.Unstructured{
				managedFields,
			},
			expectedEvents: []event.Event{
				{
					Type: event.TypeApply,
					ApplyInfo: event.ApplyInfo{
						Status: event.StatusPending,
						Object: managedFields,
					},
				},
				{
					Type: event.TypeApply,
					ApplyInfo: event.ApplyInfo{
						Status: event.StatusSuccessful,
						Object: managedFields,
					},
				},
			},
		},
	}

	postPatchObject := pkgtesting.UnstructuredFromFile(t, managedFieldsAppliedFilename)
	expectedPatch, err := csaupgrade.UpgradeManagedFieldsPatch(postPatchObject, sets.New("kubectl", fieldManager), fieldManager)
	require.NoError(t, err)

	err = csaupgrade.UpgradeManagedFields(postPatchObject, sets.New("kubectl", fieldManager), fieldManager)
	require.NoError(t, err)

	postPatchData, err := json.Marshal(postPatchObject)
	require.NoError(t, err)

	patches := 0
	targetPatches := 2
	applies := 0

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			tf := pkgtesting.NewTestClientFactory().WithNamespace("test")
			tf.Client = &fake.RESTClient{
				NegotiatedSerializer: resource.UnstructuredPlusDefaultContentConfig().NegotiatedSerializer,
				Client: fake.CreateHTTPClient(func(r *http.Request) (*http.Response, error) {
					contentType := r.Header.Get("Content-Type")
					response := pkgtesting.UnstructuredFromFile(t, managedFieldsAppliedFilename)
					data, err := runtime.Encode(unstructured.NewJSONFallbackEncoder(codec), response)
					require.NoError(t, err)
					bodyRC := io.NopCloser(bytes.NewReader(data))

					switch path, method := r.URL.Path, r.Method; {
					case method == http.MethodGet && path == managedFieldsPath:
						if patches < targetPatches {
							return &http.Response{StatusCode: http.StatusOK, Header: pkgtesting.DefaultHeaders(), Body: bodyRC}, nil
						}

						t.Logf("unexpected request: %#v\n%#v", r.URL, r)
						require.Fail(t, "sent more GET requests than expected")
						return nil, fmt.Errorf("unexpected request")
					case contentType == string(types.ApplyPatchType) && method == http.MethodPatch && path == managedFieldsPath:
						defer func() {
							applies++
						}()

						switch applies {
						case 0:
							return &http.Response{StatusCode: http.StatusOK, Header: pkgtesting.DefaultHeaders(), Body: bodyRC}, nil
						case 1, 2:
							bodyRC := io.NopCloser(bytes.NewReader(postPatchData))
							return &http.Response{StatusCode: http.StatusOK, Header: pkgtesting.DefaultHeaders(), Body: bodyRC}, nil
						}

						t.Logf("unexpected request: %#v\n%#v", r.URL, r)
						require.Fail(t, "sent more apply requests than expected")
						return &http.Response{StatusCode: http.StatusBadRequest, Header: pkgtesting.DefaultHeaders()}, nil
					case contentType == string(types.JSONPatchType) && method == http.MethodPatch && path == managedFieldsPath:
						defer func() {
							patches++
						}()

						defer r.Body.Close()
						// Require that the patch is equal to what is expected
						body, err := io.ReadAll(r.Body)
						require.NoError(t, err)
						require.Equal(t, string(expectedPatch), string(body))

						if patches == targetPatches-1 {
							bodyRC := io.NopCloser(bytes.NewReader(postPatchData))
							return &http.Response{StatusCode: http.StatusOK, Header: pkgtesting.DefaultHeaders(), Body: bodyRC}, nil
						}

						return &http.Response{StatusCode: http.StatusConflict, Header: pkgtesting.DefaultHeaders()}, nil
					default:
						require.Fail(t, "unexpected request", r.URL, r)
						return nil, fmt.Errorf("unexpected request")
					}
				}),
			}
			infoFetcher, err := DefaultInfoFetcherBuilder(tf)
			require.NoError(t, err)

			task := &ApplyTask{
				FieldManager: fieldManager,
				InfoFetcher:  infoFetcher,
				Objects:      testCase.resources,
				Filters:      testCase.filters,
				DryRun:       testCase.dryRun,
			}

			withTimeout, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
			defer cancel()
			state := &runner.FakeState{Context: withTimeout}

			task.Run(state)
			t.Log(state.SentEvents)
			require.Equal(t, len(testCase.expectedEvents), len(state.SentEvents))
			for idx, expectedEvent := range testCase.expectedEvents {
				assert.Equal(t, expectedEvent.String(), state.SentEvents[idx].String())
			}
		})
	}
}

type testFilter struct {
	Kind  string
	Error error
}

func (f *testFilter) Filter(obj *unstructured.Unstructured, _ cache.RemoteResourceGetter) (bool, error) {
	if f.Error != nil {
		return false, f.Error
	}
	return obj.GroupVersionKind().Kind == f.Kind, nil
}
