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
	"fmt"
	"testing"
	"time"

	"github.com/mia-platform/jpl/pkg/inventory"
	fakeinventory "github.com/mia-platform/jpl/pkg/inventory/fake"
	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	fakerest "k8s.io/client-go/rest/fake"
)

func TestCancelInventoryTask(t *testing.T) {
	t.Parallel()

	tf := pkgtesting.NewTestClientFactory().WithNamespace("applytest")
	tf.Client = &fakerest.RESTClient{}
	configmap, err := inventory.NewConfigMapStore(tf, "test", "test", "jpl")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.TODO())
	state := &fakeState{context: ctx}

	task := &InventoryTask{
		Manager: inventory.NewManager(configmap),
		cancel:  cancel,
	}

	task.Cancel()

	err = task.Run(state)
	require.Error(t, err)
	assert.ErrorContains(t, err, "context canceled")
}

func TestInventoryTaskRun(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		inventory     *fakeinventory.Inventory
		dryRun        bool
		expectErr     bool
		expectedError string
	}{
		"update inventory without error": {
			inventory: &fakeinventory.Inventory{},
		},
		"update inventory with dryRun": {
			inventory: &fakeinventory.Inventory{
				SaveFunc: func(_ context.Context, dryRun bool) error {
					require.True(t, dryRun)
					return nil
				},
			},
			dryRun: true,
		},
		"update inventory with error during save": {
			inventory: &fakeinventory.Inventory{
				SaveErr: fmt.Errorf("error during saving"),
			},
			expectErr:     true,
			expectedError: "error during saving",
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			task := &InventoryTask{
				Manager: inventory.NewManager(testCase.inventory),
				DryRun:  testCase.dryRun,
			}

			withTimeout, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
			defer cancel()
			state := &fakeState{context: withTimeout}

			err := task.Run(state)
			switch testCase.expectErr {
			case true:
				assert.Error(t, err)
				assert.ErrorContains(t, err, testCase.expectedError)
			default:
				assert.NoError(t, err)
			}
		})
	}
}
