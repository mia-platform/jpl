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

	"github.com/mia-platform/jpl/internal/poller"
	"github.com/mia-platform/jpl/pkg/event"
	"github.com/mia-platform/jpl/pkg/resource"
	"github.com/mia-platform/jpl/pkg/runner"
	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestCancelWaitTask(t *testing.T) {
	t.Parallel()

	tf := pkgtesting.NewTestClientFactory().WithNamespace("test")
	tf.FakeDynamicClient = nil

	mapper, err := tf.ToRESTMapper()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.TODO())
	state := &runner.FakeState{Context: ctx}

	client, err := tf.DynamicClient()
	require.NoError(t, err)

	deployment := pkgtesting.UnstructuredFromFile(t, deploymentFilename)

	task := &WaitTask{
		Objects: []*unstructured.Unstructured{
			deployment,
		},
		Poller: poller.NewDefaultStatusPoller(client, mapper),
	}

	cancel()

	task.Run(state)
	require.Equal(t, 0, len(state.SentEvents))
}

func TestWaitTask(t *testing.T) {
	t.Parallel()

	tf := pkgtesting.NewTestClientFactory()
	mapper, err := tf.ToRESTMapper()
	require.NoError(t, err)

	deployment := pkgtesting.UnstructuredFromFile(t, deploymentFilename)
	deploymentUpdate := pkgtesting.UnstructuredFromFile(t, deploymentAppliedFilename)
	mapping, err := mapper.RESTMapping(deployment.GroupVersionKind().GroupKind())
	require.NoError(t, err)

	task := &WaitTask{
		Objects: []*unstructured.Unstructured{
			deployment,
		},
		Poller: poller.NewDefaultStatusPoller(tf.FakeDynamicClient, mapper),
	}

	expectedEvents := []event.Event{
		{
			Type: event.TypeStatusUpdate,
			StatusUpdateInfo: event.StatusUpdateInfo{
				Status:         event.StatusSuccessful,
				Message:        "Deployment is available with 1 replicas",
				ObjectMetadata: resource.ObjectMetadataFromUnstructured(deployment),
			},
		},
	}

	withTimeout, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()
	state := &runner.FakeState{Context: withTimeout}

	go func() {
		require.NoError(t, tf.FakeDynamicClient.Tracker().Create(mapping.Resource, deployment, deployment.GetNamespace()))
		require.NoError(t, tf.FakeDynamicClient.Tracker().Update(mapping.Resource, deploymentUpdate, deployment.GetNamespace()))
	}()

	task.Run(state)

	require.NotEqual(t, withTimeout.Err(), context.DeadlineExceeded)
	require.Equal(t, len(expectedEvents), len(state.SentEvents))
	for idx, expectedEvent := range expectedEvents {
		assert.Equal(t, expectedEvent.String(), state.SentEvents[idx].String())
	}
}
