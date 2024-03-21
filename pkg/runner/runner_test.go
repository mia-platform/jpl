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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeTask struct {
	increase *int
	err      error
}

func (t *fakeTask) Run(_ CurrentState) error {
	*t.increase++
	return t.err
}

func (t *fakeTask) Cancel() {}

func TestNewTaskRunner(t *testing.T) {
	t.Parallel()
	assert.NotNil(t, NewTaskRunner())
}

func TestRunWithQueue(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		taskQueue       func(*int) chan Task
		wantErr         bool
		expectedTaskRun int
	}{
		"run multiple tasks": {
			taskQueue: func(count *int) chan Task {
				queue := make(chan Task, 3)
				queue <- &fakeTask{increase: count}
				queue <- &fakeTask{increase: count}
				queue <- &fakeTask{increase: count}
				defer close(queue)
				return queue
			},
			expectedTaskRun: 3,
		},
		"run nil queue": {
			taskQueue: func(_ *int) chan Task { return nil },
			wantErr:   false,
		},
		"run empty queue": {
			taskQueue: func(_ *int) chan Task {
				return make(chan Task)
			},
		},
		"error in one of the tasks": {
			taskQueue: func(count *int) chan Task {
				queue := make(chan Task, 3)
				queue <- &fakeTask{increase: count}
				queue <- &fakeTask{increase: count, err: fmt.Errorf("error in task")}
				queue <- &fakeTask{increase: count}
				return queue
			},
			expectedTaskRun: 2,
			wantErr:         true,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			count := 0
			withTimeout, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
			defer cancel()

			r := &taskRunner{}
			err := r.RunWithQueue(withTimeout, testCase.taskQueue(&count))
			switch testCase.wantErr {
			case true:
				assert.Error(t, err)
			case false:
				assert.NoError(t, err)
			}

			assert.Equal(t, testCase.expectedTaskRun, count)
		})
	}
}

func TestCancelQueue(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.TODO())
	r := &taskRunner{cancel: cancel}
	taskQueue := func(count *int) chan Task {
		queue := make(chan Task, 3)
		queue <- &fakeTask{increase: count}
		queue <- &fakeTask{increase: count}
		queue <- &fakeTask{increase: count}
		return queue
	}

	r.Cancel()

	count := 0
	err := r.RunWithQueue(ctx, taskQueue(&count))
	assert.ErrorContains(t, err, "context canceled")
}
