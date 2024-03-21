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

// TaskRunner provides abstraction for a TaskRunner implementation
type TaskRunner interface {
	RunWithQueue(context.Context, chan Task) error
	Cancel()
}

// NewTaskRunner return an implementation of TaskRunner
func NewTaskRunner() TaskRunner {
	return &taskRunner{}
}

type taskRunner struct {
	cancel context.CancelFunc
}

func (r *taskRunner) RunWithQueue(ctx context.Context, taskQueue chan Task) error {
	withCancel, cancel := context.WithCancel(ctx)
	r.cancel = cancel
	defer r.Cancel()

	runnerState := &runnerState{
		context: withCancel,
	}

	for {
		select {
		// cycle on task in the queue until they are there
		case currentTask, ok := <-taskQueue:
			if !ok {
				return nil
			}

			if err := currentTask.Run(runnerState); err != nil {
				return err
			}
			// if the context is ended or cancelled return the error if present (always nil if done with success)
		case <-withCancel.Done():
			return withCancel.Err()
		// default will be called when no task are available or the context is not cancelled or anything
		default:
			return nil
		}
	}
}

func (r *taskRunner) Cancel() {
	if r.cancel != nil {
		r.cancel()
	}
}
