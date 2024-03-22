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

package client

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/mia-platform/jpl/pkg/generator"
	fakeinventory "github.com/mia-platform/jpl/pkg/inventory/fake"
	"github.com/mia-platform/jpl/pkg/runner"
	"github.com/mia-platform/jpl/pkg/runner/task"
	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestNewApplier(t *testing.T) {
	t.Parallel()
	applier, err := NewBuilder().
		WithFactory(pkgtesting.NewTestClientFactory()).
		WithInventory(&fakeinventory.Inventory{}).
		Build()

	assert.NotNil(t, applier)
	assert.NotNil(t, applier.runner)
	assert.NotNil(t, applier.mapper)
	assert.NotNil(t, applier.infoFetcher)
	assert.NoError(t, err)

	applier, err = NewBuilder().Build()
	assert.Nil(t, applier)
	assert.Error(t, err)
}

func TestApplierRun(t *testing.T) {
	t.Parallel()
	testdataPath := filepath.Join("..", "..", "testdata", "commons")
	deploymentFilename := filepath.Join(testdataPath, "deployment.yaml")
	clusterCr := filepath.Join(testdataPath, "cluster-cr.yaml")
	testCases := map[string]struct {
		runner      runner.TaskRunner
		objects     []*unstructured.Unstructured
		options     ApplierOptions
		expectError bool
	}{
		"Apply objects with success": {
			objects: []*unstructured.Unstructured{
				pkgtesting.UnstructuredFromFile(t, deploymentFilename),
				pkgtesting.UnstructuredFromFile(t, clusterCr),
			},
			runner: &fakeRunner{
				runHandler: func(_ runner.State, queue chan runner.Task) error {
					assert.Equal(t, 2, len(queue))
					for currentTask := range queue {
						switch typedTask := currentTask.(type) {
						case *task.ApplyTask:
							assert.False(t, typedTask.DryRun)
							assert.Equal(t, 2, len(typedTask.Objects))
						case *task.InventoryTask:
							assert.False(t, typedTask.DryRun)
						default:
							assert.FailNow(t, "unknown type for task: %v", typedTask)
						}
					}
					return nil
				},
			},
		},
		"Context timeout": {
			objects: []*unstructured.Unstructured{
				pkgtesting.UnstructuredFromFile(t, deploymentFilename),
				pkgtesting.UnstructuredFromFile(t, clusterCr),
			},
			runner: &fakeRunner{
				runHandler: func(state runner.State, _ chan runner.Task) error {
					<-state.GetContext().Done()
					return state.GetContext().Err()
				},
			},
			options:     ApplierOptions{Timeout: 1 * time.Millisecond},
			expectError: true,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			applier, err := NewBuilder().
				WithFactory(pkgtesting.NewTestClientFactory()).
				WithInventory(&fakeinventory.Inventory{}).
				WithRunner(testCase.runner).
				Build()
			require.NoError(t, err)

			withTimeout, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
			defer cancel()

			err = applier.Run(withTimeout, testCase.objects, testCase.options)
			switch testCase.expectError {
			case true:
				assert.Error(t, err)
			case false:
				assert.NoError(t, err)
			}
		})
	}
}

func TestGenerators(t *testing.T) {
	t.Parallel()
	testdataPath := filepath.Join("..", "..", "testdata", "commons")
	deploymentFilename := filepath.Join(testdataPath, "deployment.yaml")
	cronjobFilename := filepath.Join(testdataPath, "cronjob.yaml")

	applier, err := NewBuilder().
		WithFactory(pkgtesting.NewTestClientFactory()).
		WithInventory(&fakeinventory.Inventory{}).
		WithRunner(&fakeRunner{
			runHandler: func(_ runner.State, queue chan runner.Task) error {
				assert.Equal(t, 2, len(queue))
				for currentTask := range queue {
					switch typedTask := currentTask.(type) {
					case *task.ApplyTask:
						assert.True(t, typedTask.DryRun)
						assert.Equal(t, 3, len(typedTask.Objects))
					case *task.InventoryTask:
						assert.True(t, typedTask.DryRun)
					default:
						assert.FailNow(t, "unknown type for task: %v", typedTask)
					}
				}
				return nil
			},
		}).
		WithGenerators(generator.NewJobGenerator("jpl.mia-platform.eu/create", "true")).
		Build()
	require.NoError(t, err)

	objects := []*unstructured.Unstructured{
		pkgtesting.UnstructuredFromFile(t, deploymentFilename),
		pkgtesting.UnstructuredFromFile(t, cronjobFilename),
	}

	withTimeout, cancel := context.WithTimeout(context.TODO(), 1*time.Second)
	defer cancel()

	err = applier.Run(withTimeout, objects, ApplierOptions{DryRun: true})
	require.NoError(t, err)
}

// keep it to always check if fakeRunner implement correctly the TaskRunner interface
var _ runner.TaskRunner = &fakeRunner{}

// fakeRunner used to abstract away the runner implementation during unit tests
type fakeRunner struct {
	runHandler func(state runner.State, queue chan runner.Task) error
}

// Cancel implement the runner.TaskRunner interface
func (r *fakeRunner) Cancel() {}

// RunWithQueue implement the runner.TaskRunner interface
func (r *fakeRunner) RunWithQueue(state runner.State, queue chan runner.Task) error {
	return r.runHandler(state, queue)
}
