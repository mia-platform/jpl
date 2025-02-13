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

	"github.com/mia-platform/jpl/pkg/event"
	"github.com/mia-platform/jpl/pkg/runner"
	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestCancelPruneTask(t *testing.T) {
	t.Parallel()

	tf := pkgtesting.NewTestClientFactory().WithNamespace("test")
	tf.FakeDynamicClient = nil

	mapper, err := tf.ToRESTMapper()
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	state := &runner.FakeState{Context: ctx}

	client, err := tf.DynamicClient()
	require.NoError(t, err)

	deployment := pkgtesting.UnstructuredFromFile(t, deploymentFilename)
	expectedEvents := []event.Event{
		{
			Type: event.TypePrune,
			PruneInfo: event.PruneInfo{
				Object: deployment,
				Status: event.StatusPending,
			},
		},
		{
			Type: event.TypePrune,
			PruneInfo: event.PruneInfo{
				Object: deployment,
				Status: event.StatusFailed,
				Error:  fmt.Errorf("client rate limiter Wait returned an error: context canceled"),
			},
		},
	}

	task := &PruneTask{
		Objects: []*unstructured.Unstructured{
			deployment,
		},

		Mapper: mapper,
		Client: client,
	}

	cancel()

	task.Run(state)
	require.Equal(t, len(expectedEvents), len(state.SentEvents))
	for idx, expectedEvent := range expectedEvents {
		assert.Equal(t, expectedEvent.String(), state.SentEvents[idx].String())
	}
}

func TestPruneAction(t *testing.T) {
	t.Parallel()

	tf := pkgtesting.NewTestClientFactory()

	deployment := pkgtesting.UnstructuredFromFile(t, deploymentFilename)
	require.NoError(t, tf.FakeDynamicClient.Tracker().Add(deployment))

	mapper, err := tf.ToRESTMapper()
	require.NoError(t, err)

	task := &PruneTask{
		Objects: []*unstructured.Unstructured{
			deployment,
		},
		Client: tf.FakeDynamicClient,
		Mapper: mapper,
	}

	expectedEvents := []event.Event{
		{
			Type: event.TypePrune,
			PruneInfo: event.PruneInfo{
				Status: event.StatusPending,
				Object: deployment,
			},
		},
		{
			Type: event.TypePrune,
			PruneInfo: event.PruneInfo{
				Status: event.StatusSuccessful,
				Object: deployment,
			},
		},
	}

	withTimeout, cancel := context.WithTimeout(t.Context(), 1*time.Second)
	defer cancel()
	state := &runner.FakeState{Context: withTimeout}

	task.Run(state)
	require.Equal(t, len(expectedEvents), len(state.SentEvents))
	for idx, expectedEvent := range expectedEvents {
		assert.Equal(t, expectedEvent.String(), state.SentEvents[idx].String())
	}
}
