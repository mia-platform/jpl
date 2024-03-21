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

	"github.com/mia-platform/jpl/pkg/inventory"
	"github.com/mia-platform/jpl/pkg/runner"
)

// keep it to always check if InventoryTask implement correctly the Task interface
var _ runner.Task = &InventoryTask{}

// InventoryTask is used for updating an inventory with the current state saved in the Manager
type InventoryTask struct {
	Manager *inventory.Manager
	DryRun  bool

	cancel context.CancelFunc
}

// Run implement the runner.Task interface
func (t *InventoryTask) Run(state runner.CurrentState) error {
	withCancel, cancel := context.WithCancel(state.GetContext())
	t.cancel = cancel
	defer t.Cancel()

	return t.Manager.SaveCurrentInventoryState(withCancel, t.DryRun)
}

// Cancel implement the runner.Task interface
func (t *InventoryTask) Cancel() {
	if t.cancel != nil {
		t.cancel()
	}
}
