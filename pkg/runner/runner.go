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

// TaskRunner provides abstraction for a TaskRunner implementation
type TaskRunner interface {
	// RunWithQueue will start to execute all the tasks that will be found in the channel
	RunWithQueue(State, <-chan Task) error
}

// NewTaskRunner return an implementation of TaskRunner
func NewTaskRunner() TaskRunner {
	return &taskRunner{}
}

type taskRunner struct{}

func (r *taskRunner) RunWithQueue(state State, taskQueue <-chan Task) error {
	ctx := state.GetContext()
	done := ctx.Done()

	for {
		select {
		// if the context is ended or cancelled return the error if present (always nil if done with success)
		case <-done:
			return ctx.Err()

		// cycle on task in the queue until they are there
		case currentTask, open := <-taskQueue:
			if !open {
				return nil
			}

			currentTask.Run(state)
		}
	}
}
