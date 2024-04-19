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
	"github.com/mia-platform/jpl/pkg/event"
	"github.com/mia-platform/jpl/pkg/inventory"
	"github.com/mia-platform/jpl/pkg/runner"
)

// keep it to always check if InventoryTask implement correctly the Task interface
var _ runner.Task = &InventoryTask{}

// InventoryTask is used for updating an inventory with the current state saved in the Manager
type InventoryTask struct {
	Manager *inventory.Manager
	DryRun  bool
}

// Run implement the runner.Task interface
func (t *InventoryTask) Run(state runner.State) {
	ctx := state.GetContext()
	state.SendEvent(event.Event{
		Type: event.TypeInventory,
		InventoryInfo: event.InventoryInfo{
			Status: event.StatusPending,
		},
	})

	if err := t.Manager.SaveCurrentInventoryState(ctx, t.DryRun); err != nil {
		state.SendEvent(event.Event{
			Type: event.TypeInventory,
			InventoryInfo: event.InventoryInfo{
				Status: event.StatusFailed,
				Error:  err,
			},
		})
		return
	}

	state.SendEvent(event.Event{
		Type: event.TypeInventory,
		InventoryInfo: event.InventoryInfo{
			Status: event.StatusSuccessful,
		},
	})
}
