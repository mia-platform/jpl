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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/features"
)

func TestApplyCronJobsWithGenerator(t *testing.T) {
	applyFeature := features.New("apply on empty namespace").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Helper()
			factory, store := factoryAndStoreForTesting(t, cfg, "inventory")

			resourcePath := testdataPathForPath(t, "generator-apply")
			applyResources(t, factory, store, nil, resourcePath, 2)
			return ctx
		}).
		Assess("check cronjobs", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Helper()

			cronjobs := new(batchv1.CronJobList)
			require.NoError(t, cfg.Client().Resources().WithNamespace(cfg.Namespace()).List(ctx, cronjobs))
			assert.Equal(t, 2, len(cronjobs.Items))
			return ctx
		}).
		Assess("check jobs", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			t.Helper()

			jobs := new(batchv1.JobList)
			require.NoError(t, cfg.Client().Resources().WithNamespace(cfg.Namespace()).List(ctx, jobs))
			require.Equal(t, 1, len(jobs.Items))

			assert.Equal(t, "manual", jobs.Items[0].Annotations["cronjob.kubernetes.io/instantiate"])
			return ctx
		}).
		Feature()

	testenv.Test(t, applyFeature)
}
