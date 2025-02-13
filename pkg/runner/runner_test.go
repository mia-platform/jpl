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

	"github.com/mia-platform/jpl/pkg/event"
	"github.com/stretchr/testify/assert"
)

type fakeTask struct {
	err error
}

func (t *fakeTask) Run(state State) {
	state.SendEvent(event.Event{
		Type: event.TypeApply,
		ApplyInfo: event.ApplyInfo{
			Error: t.err,
		},
	})
}

func (t *fakeTask) Cancel() {}

func TestNewTaskRunner(t *testing.T) {
	t.Parallel()
	assert.NotNil(t, NewTaskRunner())
}

func TestRunWithQueue(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		taskQueue      func() chan Task
		expectedEvents []event.Event
	}{
		"run multiple tasks": {
			taskQueue: func() chan Task {
				queue := make(chan Task, 3)
				queue <- &fakeTask{}
				queue <- &fakeTask{}
				queue <- &fakeTask{}
				defer close(queue)
				return queue
			},
			expectedEvents: []event.Event{
				{Type: event.TypeApply, ApplyInfo: event.ApplyInfo{}},
				{Type: event.TypeApply, ApplyInfo: event.ApplyInfo{}},
				{Type: event.TypeApply, ApplyInfo: event.ApplyInfo{}},
			},
		},
		"run nil queue": {
			taskQueue:      func() chan Task { return nil },
			expectedEvents: []event.Event(nil),
		},
		"run empty queue": {
			taskQueue: func() chan Task {
				return make(chan Task)
			},
			expectedEvents: []event.Event(nil),
		},
		"error in one of the tasks": {
			taskQueue: func() chan Task {
				queue := make(chan Task, 3)
				queue <- &fakeTask{}
				queue <- &fakeTask{err: fmt.Errorf("error in task")}
				queue <- &fakeTask{}
				return queue
			},
			expectedEvents: []event.Event{
				{Type: event.TypeApply, ApplyInfo: event.ApplyInfo{}},
				{Type: event.TypeApply, ApplyInfo: event.ApplyInfo{Error: fmt.Errorf("error in task")}},
				{Type: event.TypeApply, ApplyInfo: event.ApplyInfo{}},
			},
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			withTimeout, cancel := context.WithTimeout(t.Context(), 1*time.Second)
			defer cancel()
			state := &FakeState{Context: withTimeout}

			r := &taskRunner{}
			r.RunWithQueue(state, testCase.taskQueue())
			assert.Equal(t, testCase.expectedEvents, state.SentEvents)
		})
	}
}

func TestCancelQueue(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	state := &FakeState{Context: ctx}
	r := &taskRunner{}
	taskQueue := func() chan Task {
		queue := make(chan Task, 3)
		queue <- &fakeTask{}
		queue <- &fakeTask{}
		queue <- &fakeTask{}
		return queue
	}

	cancel()

	err := r.RunWithQueue(state, taskQueue())
	assert.ErrorContains(t, err, "context canceled")
}
