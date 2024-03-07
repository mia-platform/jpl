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

	"github.com/mia-platform/jpl/pkg/inventory"
	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	clienttesting "k8s.io/client-go/testing"
)

func TestCancelPruneTask(t *testing.T) {
	t.Parallel()

	tf := pkgtesting.NewTestClientFactory().WithNamespace("applytest")
	tf.FakeDynamicClient = nil
	mapper, err := tf.ToRESTMapper()
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.TODO())

	client, err := tf.DynamicClient()
	require.NoError(t, err)
	task := &PruneTask{
		ObjectMetadata: []inventory.ResourceMetadata{
			{
				Name:      "cancel-test",
				Namespace: "cancel-test",
				Kind:      "Pod",
				Group:     "",
			},
		},

		Mapper: mapper,
		Client: client,
		cancel: cancel,
	}

	task.Cancel()

	err = task.Run(ctx)
	require.Error(t, err)
	assert.ErrorContains(t, err, "context canceled")
}

func TestPruneAction(t *testing.T) {
	t.Parallel()

	tf := pkgtesting.NewTestClientFactory()

	mapper, err := tf.ToRESTMapper()
	require.NoError(t, err)

	task := &PruneTask{
		ObjectMetadata: []inventory.ResourceMetadata{
			{
				Kind:      "Pod",
				Name:      "test",
				Namespace: "prune-test",
			},
		},
		Client: tf.FakeDynamicClient,
		Mapper: mapper,
	}

	err = task.Run(context.TODO())
	assert.NoError(t, err)

	require.Equal(t, 1, len(tf.FakeDynamicClient.Actions()))
	action := tf.FakeDynamicClient.Actions()[0]
	require.IsType(t, clienttesting.DeleteActionImpl{}, action)

	deleteAction := action.(clienttesting.DeleteActionImpl)
	assert.Equal(t, "test", deleteAction.GetName())
	assert.Equal(t, "prune-test", deleteAction.GetNamespace())
}
