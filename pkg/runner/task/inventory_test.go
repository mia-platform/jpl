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
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mia-platform/jpl/pkg/event"
	"github.com/mia-platform/jpl/pkg/inventory"
	fakeinventory "github.com/mia-platform/jpl/pkg/inventory/fake"
	"github.com/mia-platform/jpl/pkg/runner"
	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	fakerest "k8s.io/client-go/rest/fake"
)

func TestCancelInventoryTask(t *testing.T) {
	t.Parallel()

	tf := pkgtesting.NewTestClientFactory().WithNamespace("test")
	tf.Client = &fakerest.RESTClient{}
	configmap, err := inventory.NewConfigMapStore(tf, "test", "test", "jpl")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	state := &runner.FakeState{Context: ctx}

	expectedEvents := []event.Event{
		{
			Type: event.TypeInventory,
			InventoryInfo: event.InventoryInfo{
				Status: event.StatusPending,
			},
		},
		{
			Type: event.TypeInventory,
			InventoryInfo: event.InventoryInfo{
				Status: event.StatusFailed,
				Error:  errors.New("failed to save inventory: client rate limiter Wait returned an error: context canceled"),
			},
		},
	}
	task := &InventoryTask{
		Manager: inventory.NewManager(configmap, nil),
	}

	cancel()

	task.Run(state)
	require.Len(t, state.SentEvents, len(expectedEvents))
	for idx, expectedEvent := range expectedEvents {
		assert.Equal(t, expectedEvent.String(), state.SentEvents[idx].String())
	}
}

func TestInventoryTaskRun(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		inventory      *fakeinventory.Inventory
		expectedEvents []event.Event
		dryRun         bool
	}{
		"update inventory without error": {
			inventory: &fakeinventory.Inventory{},
			expectedEvents: []event.Event{
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
		"update inventory with dryRun": {
			inventory: &fakeinventory.Inventory{
				SaveFunc: func(_ context.Context, dryRun bool) error {
					require.True(t, dryRun)
					return nil
				},
			},
			expectedEvents: []event.Event{
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
			dryRun: true,
		},
		"update inventory with error during save": {
			inventory: &fakeinventory.Inventory{
				SaveErr: errors.New("error during saving"),
			},
			expectedEvents: []event.Event{
				{
					Type: event.TypeInventory,
					InventoryInfo: event.InventoryInfo{
						Status: event.StatusPending,
					},
				},
				{
					Type: event.TypeInventory,
					InventoryInfo: event.InventoryInfo{
						Status: event.StatusFailed,
						Error:  errors.New("error during saving"),
					},
				},
			},
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			task := &InventoryTask{
				Manager: inventory.NewManager(testCase.inventory, nil),
				DryRun:  testCase.dryRun,
			}

			withTimeout, cancel := context.WithTimeout(t.Context(), 1*time.Second)
			defer cancel()
			state := &runner.FakeState{Context: withTimeout}

			task.Run(state)
			require.Len(t, state.SentEvents, len(testCase.expectedEvents))
			for idx, expectedEvent := range testCase.expectedEvents {
				assert.Equal(t, expectedEvent.String(), state.SentEvents[idx].String())
			}
		})
	}
}
