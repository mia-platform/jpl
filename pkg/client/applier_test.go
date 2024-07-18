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

package client

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/mia-platform/jpl/pkg/client/cache"
	"github.com/mia-platform/jpl/pkg/event"
	"github.com/mia-platform/jpl/pkg/filter"
	"github.com/mia-platform/jpl/pkg/generator"
	fakeinventory "github.com/mia-platform/jpl/pkg/inventory/fake"
	"github.com/mia-platform/jpl/pkg/mutator"
	"github.com/mia-platform/jpl/pkg/resource"
	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNewApplier(t *testing.T) {
	t.Parallel()
	applier, err := NewBuilder().
		WithFactory(factoryForTesting(t, nil, nil)).
		WithInventory(&fakeinventory.Inventory{}).
		Build()

	assert.NotNil(t, applier)
	assert.NotNil(t, applier.runner)
	assert.NotNil(t, applier.mapper)
	assert.NotNil(t, applier.infoFetcher)
	assert.NotNil(t, applier.poller)
	assert.NoError(t, err)

	applier, err = NewBuilder().Build()
	assert.Nil(t, applier)
	assert.Error(t, err)
}

func TestApplierRun(t *testing.T) {
	t.Parallel()
	testdataPath := "testdata"

	deployment := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdataPath, "deployment.yaml"))
	namespace := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdataPath, "namespace.yaml"))

	testCases := map[string]struct {
		objects          []*unstructured.Unstructured
		inventoryObjects []*unstructured.Unstructured
		options          ApplierOptions
		expectedEvents   []event.Event
		statusEvents     []event.Event
	}{
		"Apply objects with success without previous inventory": {
			objects: []*unstructured.Unstructured{
				deployment,
				namespace,
			},
			expectedEvents: []event.Event{
				{
					Type: event.TypeApply,
					ApplyInfo: event.ApplyInfo{
						Object: namespace,
						Status: event.StatusPending,
					},
				},
				{
					Type: event.TypeApply,
					ApplyInfo: event.ApplyInfo{
						Object: namespace,
						Status: event.StatusSuccessful,
					},
				},
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
						Status: event.StatusSuccessful,
					},
				},
				{
					Type: event.TypeStatusUpdate,
					StatusUpdateInfo: event.StatusUpdateInfo{
						Message:        "",
						Status:         event.StatusSuccessful,
						ObjectMetadata: resource.ObjectMetadataFromUnstructured(deployment),
					},
				},
				{
					Type: event.TypeStatusUpdate,
					StatusUpdateInfo: event.StatusUpdateInfo{
						Message:        "",
						Status:         event.StatusSuccessful,
						ObjectMetadata: resource.ObjectMetadataFromUnstructured(namespace),
					},
				},
				{
					Type: event.TypeInventory,
					InventoryInfo: event.InventoryInfo{
						Status: event.StatusPending,
					},
				},
				{
					Type: event.TypeInventory,
					InventoryInfo: event.InventoryInfo{
						Status: event.StatusSuccessful,
					},
				},
			},
			statusEvents: []event.Event{
				{
					Type: event.TypeStatusUpdate,
					StatusUpdateInfo: event.StatusUpdateInfo{
						Message:        "",
						Status:         event.StatusSuccessful,
						ObjectMetadata: resource.ObjectMetadataFromUnstructured(deployment),
					},
				},
				{
					Type: event.TypeStatusUpdate,
					StatusUpdateInfo: event.StatusUpdateInfo{
						Message:        "",
						Status:         event.StatusSuccessful,
						ObjectMetadata: resource.ObjectMetadataFromUnstructured(namespace),
					},
				},
			},
		},
		"Apply objects with success with previous inventory": {
			objects: []*unstructured.Unstructured{
				deployment,
				namespace,
			},
			inventoryObjects: []*unstructured.Unstructured{
				namespace,
			},
			expectedEvents: []event.Event{
				{
					Type: event.TypeApply,
					ApplyInfo: event.ApplyInfo{
						Object: namespace,
						Status: event.StatusPending,
					},
				},
				{
					Type: event.TypeApply,
					ApplyInfo: event.ApplyInfo{
						Object: namespace,
						Status: event.StatusSuccessful,
					},
				},
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
						Status: event.StatusSuccessful,
					},
				},
				{
					Type: event.TypeInventory,
					InventoryInfo: event.InventoryInfo{
						Status: event.StatusPending,
					},
				},
				{
					Type: event.TypeInventory,
					InventoryInfo: event.InventoryInfo{
						Status: event.StatusSuccessful,
					},
				},
			},
			options: ApplierOptions{DryRun: true},
		},
		"Apply and prune objects with success with previous inventory": {
			objects: []*unstructured.Unstructured{
				deployment,
			},
			inventoryObjects: []*unstructured.Unstructured{
				namespace,
			},
			expectedEvents: []event.Event{
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
						Status: event.StatusSuccessful,
					},
				},
				{
					Type: event.TypePrune,
					PruneInfo: event.PruneInfo{
						Object: namespace,
						Status: event.StatusPending,
					},
				},
				{
					Type: event.TypePrune,
					PruneInfo: event.PruneInfo{
						Object: namespace,
						Status: event.StatusSuccessful,
					},
				},
				{
					Type: event.TypeInventory,
					InventoryInfo: event.InventoryInfo{
						Status: event.StatusPending,
					},
				},
				{
					Type: event.TypeInventory,
					InventoryInfo: event.InventoryInfo{
						Status: event.StatusSuccessful,
					},
				},
			},
			options: ApplierOptions{DryRun: true},
		},
		"error during graph building": {
			objects: []*unstructured.Unstructured{
				func() *unstructured.Unstructured {
					dep := deployment.DeepCopy()
					dep.SetAnnotations(map[string]string{
						resource.Annotation: "value",
					})
					return dep
				}(),
			},
			inventoryObjects: []*unstructured.Unstructured{
				namespace,
			},
			expectedEvents: []event.Event{
				{
					Type: event.TypeError,
					ErrorInfo: event.ErrorInfo{
						Error: fmt.Errorf("failed to parse object reference: unexpected field composition: value"),
					},
				},
			},
			options: ApplierOptions{DryRun: true},
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			applier := newTestApplier(t, testCase.objects, testCase.inventoryObjects, testCase.statusEvents, nil, nil, nil)
			withTimeout, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
			defer cancel()

			eventCh := applier.Run(context.TODO(), testCase.objects, testCase.options)
			var events []event.Event

		loop:
			for {
				select {
				case <-withTimeout.Done():
					if errors.Is(withTimeout.Err(), context.DeadlineExceeded) {
						assert.Fail(t, "context ended in timeout, something is pending")
						break loop
					}

				case e, open := <-eventCh:
					if !open {
						break loop
					}

					events = append(events, e)
				}
			}

			require.Equal(t, len(testCase.expectedEvents), len(events), "actual events found: %v", events)
			for idx, expectedEvent := range testCase.expectedEvents {
				assert.Equal(t, expectedEvent.String(), events[idx].String())
			}
		})
	}
}

func TestGenerators(t *testing.T) {
	t.Parallel()
	testdataPath := "testdata"
	deployment := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdataPath, "deployment.yaml"))
	cronjonb := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdataPath, "cronjob.yaml"))
	job := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdataPath, "job.yaml"))

	testCases := map[string]struct {
		objects        []*unstructured.Unstructured
		options        ApplierOptions
		generator      generator.Interface
		expectedEvents []event.Event
	}{
		"generate object": {
			objects: []*unstructured.Unstructured{
				deployment,
				cronjonb,
			},
			options: ApplierOptions{
				DryRun: true,
			},
			generator: &fakeGenerator{resource: job},
			expectedEvents: []event.Event{
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
						Status: event.StatusSuccessful,
					},
				},
				{
					Type: event.TypeApply,
					ApplyInfo: event.ApplyInfo{
						Object: cronjonb,
						Status: event.StatusPending,
					},
				},
				{
					Type: event.TypeApply,
					ApplyInfo: event.ApplyInfo{
						Object: cronjonb,
						Status: event.StatusSuccessful,
					},
				},
				{
					Type: event.TypeApply,
					ApplyInfo: event.ApplyInfo{
						Object: job,
						Status: event.StatusPending,
					},
				},
				{
					Type: event.TypeApply,
					ApplyInfo: event.ApplyInfo{
						Object: job,
						Status: event.StatusSuccessful,
					},
				},
				{
					Type: event.TypeInventory,
					InventoryInfo: event.InventoryInfo{
						Status: event.StatusPending,
					},
				},
				{
					Type: event.TypeInventory,
					InventoryInfo: event.InventoryInfo{
						Status: event.StatusSuccessful,
					},
				},
			},
		},
		"error during object generation": {
			objects: []*unstructured.Unstructured{
				deployment,
				cronjonb,
			},
			options: ApplierOptions{
				DryRun: true,
			},
			generator: &errorGenerator{err: fmt.Errorf("abort")},
			expectedEvents: []event.Event{
				{
					Type: event.TypeError,
					ErrorInfo: event.ErrorInfo{
						Error: fmt.Errorf("generate resource failed: abort"),
					},
				},
			},
		},
	}

	for testName, testCase := range testCases {
		withTimeout, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
		defer cancel()

		t.Run(testName, func(t *testing.T) {
			applier := newTestApplier(t, append(testCase.objects, job), nil, []event.Event{}, testCase.generator, nil, nil)
			eventCh := applier.Run(withTimeout, testCase.objects, ApplierOptions{DryRun: true})
			var events []event.Event
		loop:
			for {
				select {
				case <-withTimeout.Done():
					assert.Fail(t, "context endend in timeout, something is pending")
					break loop

				case e, open := <-eventCh:
					if !open {
						break loop
					}

					events = append(events, e)
				}
			}

			require.Equal(t, len(testCase.expectedEvents), len(events), "actual events found: %v", events)
			for idx, expectedEvent := range testCase.expectedEvents {
				assert.Equal(t, expectedEvent.String(), events[idx].String())
			}
		})
	}
}

func TestMutators(t *testing.T) {
	t.Parallel()
	testdataPath := "testdata"
	deployment := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdataPath, "deployment.yaml"))

	testCases := map[string]struct {
		objects        []*unstructured.Unstructured
		options        ApplierOptions
		mutator        mutator.Interface
		expectedEvents []event.Event
	}{
		"mutate object": {
			objects: []*unstructured.Unstructured{
				deployment,
			},
			options: ApplierOptions{
				DryRun: true,
			},
			mutator: mutator.NewLabelsMutator(map[string]string{"foo": "bar"}),
			expectedEvents: []event.Event{
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
						Status: event.StatusSuccessful,
					},
				},
				{
					Type: event.TypeInventory,
					InventoryInfo: event.InventoryInfo{
						Status: event.StatusPending,
					},
				},
				{
					Type: event.TypeInventory,
					InventoryInfo: event.InventoryInfo{
						Status: event.StatusSuccessful,
					},
				},
			},
		},
		"skip mutate object": {
			objects: []*unstructured.Unstructured{
				deployment,
			},
			options: ApplierOptions{
				DryRun: true,
			},
			mutator: mutator.NewLabelsMutator(nil),
			expectedEvents: []event.Event{
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
						Status: event.StatusSuccessful,
					},
				},
				{
					Type: event.TypeInventory,
					InventoryInfo: event.InventoryInfo{
						Status: event.StatusPending,
					},
				},
				{
					Type: event.TypeInventory,
					InventoryInfo: event.InventoryInfo{
						Status: event.StatusSuccessful,
					},
				},
			},
		},
		"error during object mutation": {
			objects: []*unstructured.Unstructured{
				deployment,
			},
			options: ApplierOptions{
				DryRun: true,
			},
			mutator: mutator.NewLabelsMutator(map[string]string{"invalid-": "value"}),
			expectedEvents: []event.Event{
				{
					Type: event.TypeError,
					ErrorInfo: event.ErrorInfo{
						Error: fmt.Errorf(`mutate resource failed: labels: Invalid value: "invalid-": name part must consist of alphanumeric characters, '-', '_' or '.', and must start and end with an alphanumeric character (e.g. 'MyName',  or 'my.name',  or '123-abc', regex used for validation is '([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9]')`),
					},
				},
			},
		},
	}

	for testName, testCase := range testCases {
		withTimeout, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
		defer cancel()

		t.Run(testName, func(t *testing.T) {
			applier := newTestApplier(t, testCase.objects, nil, []event.Event{}, nil, testCase.mutator, nil)
			eventCh := applier.Run(withTimeout, testCase.objects, ApplierOptions{DryRun: true})
			var events []event.Event
		loop:
			for {
				select {
				case <-withTimeout.Done():
					assert.Fail(t, "context endend in timeout, something is pending")
					break loop

				case e, open := <-eventCh:
					if !open {
						break loop
					}

					events = append(events, e)
				}
			}

			require.Equal(t, len(testCase.expectedEvents), len(events), "actual events found: %v", events)
			for idx, expectedEvent := range testCase.expectedEvents {
				assert.Equal(t, expectedEvent.String(), events[idx].String())
			}
		})
	}
}

func TestFilters(t *testing.T) {
	t.Parallel()
	testdataPath := "testdata"
	deployment := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdataPath, "deployment.yaml"))
	job := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdataPath, "job.yaml"))

	objects := []*unstructured.Unstructured{
		deployment,
		job,
	}

	expectedEvents := []event.Event{
		{
			Type: event.TypeApply,
			ApplyInfo: event.ApplyInfo{
				Object: deployment,
				Status: event.StatusSkipped,
			},
		},
		{
			Type: event.TypeApply,
			ApplyInfo: event.ApplyInfo{
				Object: job,
				Status: event.StatusPending,
			},
		},
		{
			Type: event.TypeApply,
			ApplyInfo: event.ApplyInfo{
				Object: job,
				Status: event.StatusSuccessful,
			},
		},
		{
			Type: event.TypeInventory,
			InventoryInfo: event.InventoryInfo{
				Status: event.StatusPending,
			},
		},
		{
			Type: event.TypeInventory,
			InventoryInfo: event.InventoryInfo{
				Status: event.StatusSuccessful,
			},
		},
	}

	filter := &testFilter{}

	withTimeout, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()

	applier := newTestApplier(t, objects, nil, []event.Event{}, nil, nil, filter)
	eventCh := applier.Run(withTimeout, objects, ApplierOptions{DryRun: true})
	var events []event.Event
loop:
	for {
		select {
		case <-withTimeout.Done():
			assert.Fail(t, "context endend in timeout, something is pending")
			break loop

		case e, open := <-eventCh:
			if !open {
				break loop
			}

			events = append(events, e)
		}
	}

	require.Equal(t, len(expectedEvents), len(events), "actual events found: %v", events)
	for idx, expectedEvent := range expectedEvents {
		assert.Equal(t, expectedEvent.String(), events[idx].String())
	}
}

func TestLoadObjectFromInventory(t *testing.T) {
	t.Parallel()

	testdataPath := "testdata"

	deployment := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdataPath, "deployment.yaml"))
	namespace := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdataPath, "namespace.yaml"))

	tests := map[string]struct {
		inventory       *fakeinventory.Inventory
		remoteObjects   []*unstructured.Unstructured
		expectedObjects int
	}{
		"load no objects if inventory is empty": {
			inventory:       &fakeinventory.Inventory{InventoryObjects: nil},
			expectedObjects: 0,
		},
		"load all objects inside the inventory": {
			inventory: &fakeinventory.Inventory{InventoryObjects: []*unstructured.Unstructured{
				deployment,
				namespace,
			}},
			remoteObjects: []*unstructured.Unstructured{
				deployment,
				namespace,
			},
			expectedObjects: 2,
		},
		"return all objects available on remote and not return error": {
			inventory: &fakeinventory.Inventory{InventoryObjects: []*unstructured.Unstructured{
				deployment,
				namespace,
			}},
			remoteObjects: []*unstructured.Unstructured{
				namespace,
			},
			expectedObjects: 1,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			factory := factoryForTesting(t, nil, test.remoteObjects)
			applier, err := NewBuilder().
				WithInventory(test.inventory).
				WithFactory(factory).
				Build()

			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
			defer cancel()

			resourceCache := cache.NewCachedResourceGetter(applier.mapper, applier.client)
			objs, err := applier.loadObjectsFromInventory(ctx, resourceCache)
			assert.NoError(t, err)
			assert.Equal(t, test.expectedObjects, len(objs))
		})
	}
}

var _ filter.Interface = &testFilter{}

type testFilter struct{}

func (f *testFilter) Filter(obj *unstructured.Unstructured, _ cache.RemoteResourceGetter) (bool, error) {
	if obj.GroupVersionKind().Kind == "Deployment" {
		return true, nil
	}

	return false, nil
}
