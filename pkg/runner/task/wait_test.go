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
	"testing"
	"time"

	"github.com/mia-platform/jpl/pkg/event"
	"github.com/mia-platform/jpl/pkg/poller"
	"github.com/mia-platform/jpl/pkg/resource"
	"github.com/mia-platform/jpl/pkg/runner"
	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestCancelWaitTask(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	state := &runner.FakeState{Context: ctx}

	deployment := pkgtesting.UnstructuredFromFile(t, deploymentFilename)

	task := &WaitTask{
		Objects: []*unstructured.Unstructured{
			deployment,
		},
		Poller: &poller.FakePoller{},
	}

	cancel()

	task.Run(state)
	require.Empty(t, state.SentEvents)
}

func TestWaitTask(t *testing.T) {
	t.Parallel()

	deployment := pkgtesting.UnstructuredFromFile(t, deploymentFilename)
	deploymentApplied := pkgtesting.UnstructuredFromFile(t, deploymentAppliedFilename)
	namespace := pkgtesting.UnstructuredFromFile(t, namespaceFilename)

	task := &WaitTask{
		Objects: []*unstructured.Unstructured{
			deployment,
			deploymentApplied,
			deploymentApplied,
			namespace,
		},
		Poller: &poller.FakePoller{},
	}

	expectedEvents := []event.Event{
		{
			Type: event.TypeStatusUpdate,
			StatusUpdateInfo: event.StatusUpdateInfo{
				Status:         event.StatusPending,
				Message:        "Deployment creating replicas: 0/1",
				ObjectMetadata: resource.ObjectMetadataFromUnstructured(deployment),
			},
		},
		{
			Type: event.TypeStatusUpdate,
			StatusUpdateInfo: event.StatusUpdateInfo{
				Status:         event.StatusSuccessful,
				Message:        "Deployment is available with 1 replicas",
				ObjectMetadata: resource.ObjectMetadataFromUnstructured(deployment),
			},
		},
		{
			Type: event.TypeStatusUpdate,
			StatusUpdateInfo: event.StatusUpdateInfo{
				Status:         event.StatusSuccessful,
				Message:        "Resource is current",
				ObjectMetadata: resource.ObjectMetadataFromUnstructured(namespace),
			},
		},
	}

	withTimeout, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	state := &runner.FakeState{Context: withTimeout}

	go func() {
		task.Run(state)
		cancel()
	}()

	<-withTimeout.Done()
	require.NotEqual(t, withTimeout.Err(), context.DeadlineExceeded)
	assert.Len(t, state.SentEvents, len(expectedEvents))
	for idx, expectedEvent := range expectedEvents {
		assert.Equal(t, expectedEvent.String(), state.SentEvents[idx].String())
	}
}
