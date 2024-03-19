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

//go:build conformance

package e2e

import (
	"testing"

	"github.com/mia-platform/jpl/pkg/inventory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestApplyCronJobsWithGenerator(t *testing.T) {
	t.Parallel()

	// prepare test constants
	expectedResourcesCount := 2
	resourcePath := testdataPathForPath(t, "generator-apply")
	namespace := createNamespaceForTesting(t)
	factory := factoryForTesting(t, &namespace)
	envtestListOptions := &client.ListOptions{Namespace: namespace}
	store, err := inventory.NewConfigMapStore(factory, "inventory", namespace, "jpl-e2e-test")
	require.NoError(t, err)

	// apply resources
	applyResources(t, factory, store, nil, resourcePath, expectedResourcesCount)

	// control that all CronJobs are present
	appliedCronJobs := &batchv1.CronJobList{}
	getResourcesFromEnv(t, appliedCronJobs, envtestListOptions)
	assert.Equal(t, expectedResourcesCount, len(appliedCronJobs.Items))

	// check that a job is created
	appliedJobs := &batchv1.JobList{}
	getResourcesFromEnv(t, appliedJobs, envtestListOptions)
	assert.Equal(t, 1, len(appliedJobs.Items))
}
