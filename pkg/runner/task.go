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

package runner

import (
	"context"
)

// Task provides abstractions that a Task must implement to be able to be used by a Runner
type Task interface {
	// Run is used to execute the action implemented by the Task, it expect an error as return if something goes wrong
	Run(State) error
	// Cancel is used to interrupt the task if possible
	Cancel()
}

// State encapsulate the state of the run for sharing data between different tasks execution
type State interface {
	// GetContext return the Context where to execute task
	GetContext() context.Context

	// SendEvent is used for sending back status updates for the current task
	SendEvent(event Event)
}
