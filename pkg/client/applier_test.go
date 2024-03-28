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
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/mia-platform/jpl/pkg/event"
	fakeinventory "github.com/mia-platform/jpl/pkg/inventory/fake"
	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNewApplier(t *testing.T) {
	t.Parallel()
	applier, err := NewBuilder().
		WithFactory(factoryForTesting(t, nil)).
		WithInventory(&fakeinventory.Inventory{}).
		Build()

	assert.NotNil(t, applier)
	assert.NotNil(t, applier.runner)
	assert.NotNil(t, applier.mapper)
	assert.NotNil(t, applier.infoFetcher)
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
		objects        []*unstructured.Unstructured
		options        ApplierOptions
		expectedEvents []event.Event
	}{
		"Apply objects with success": {
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
		"Context timeout": {
			objects: []*unstructured.Unstructured{
				deployment,
				namespace,
			},
			options: ApplierOptions{Timeout: 1 * time.Nanosecond},
			expectedEvents: []event.Event{
				{
					Type: event.TypeError,
					ErrorInfo: event.ErrorInfo{
						Error: fmt.Errorf("context deadline exceeded"),
					},
				},
			},
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			applier := newTestApplier(t, testCase.objects)
			withTimeout, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
			defer cancel()

			eventCh := applier.Run(context.TODO(), testCase.objects, testCase.options)
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

func TestGenerators(t *testing.T) {
	t.Parallel()
	testdataPath := "testdata"
	deployment := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdataPath, "deployment.yaml"))
	cronjonb := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdataPath, "cronjob.yaml"))
	job := pkgtesting.UnstructuredFromFile(t, filepath.Join(testdataPath, "job.yaml"))

	objects := []*unstructured.Unstructured{
		deployment,
		cronjonb,
	}

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
	}

	withTimeout, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()

	applier := newTestApplier(t, append(objects, job), &fakeGenerator{resource: job})
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
