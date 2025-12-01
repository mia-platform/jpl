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
	"sort"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"

	"github.com/mia-platform/jpl/pkg/client/cache"
	"github.com/mia-platform/jpl/pkg/filter"
	"github.com/mia-platform/jpl/pkg/inventory"
	"github.com/mia-platform/jpl/pkg/poller"
	"github.com/mia-platform/jpl/pkg/resource"
	"github.com/mia-platform/jpl/pkg/runner"
	"github.com/mia-platform/jpl/pkg/runner/task"
)

type QueueOptions struct {
	Wait         bool
	DryRun       bool
	Prune        bool
	FieldManager string
}

type QueueBuilder struct {
	objects      []*unstructured.Unstructured
	pruneObjects []*unstructured.Unstructured

	Manager      *inventory.Manager
	Client       dynamic.Interface
	Mapper       meta.RESTMapper
	Filters      []filter.Interface
	InfoFetcher  task.InfoFetcher
	RemoteGetter cache.RemoteResourceGetter
	Poller       poller.StatusPoller
}

func (b *QueueBuilder) WithObjects(objs []*unstructured.Unstructured) *QueueBuilder {
	b.objects = objs
	return b
}

func (b *QueueBuilder) WithPruneObjects(objs []*unstructured.Unstructured) *QueueBuilder {
	b.pruneObjects = objs
	return b
}

func (b *QueueBuilder) Build(options QueueOptions) (<-chan runner.Task, error) {
	tasks := make([]runner.Task, 0)

	if len(b.objects) > 0 {
		graph, err := resource.NewDependencyGraph(b.objects)
		if err != nil {
			return nil, err
		}

		groups, err := graph.SortedResourceGroups()
		if err != nil {
			return nil, err
		}

		for _, group := range groups {
			tasks = append(tasks, &task.ApplyTask{
				DryRun:       options.DryRun,
				FieldManager: options.FieldManager,

				Objects:      group,
				Filters:      b.Filters,
				InfoFetcher:  b.InfoFetcher,
				RemoteGetter: b.RemoteGetter,
			})
			if !options.DryRun && options.Wait {
				tasks = append(tasks, &task.WaitTask{
					Objects: group,
					Poller:  b.Poller,
					Mapper:  b.Mapper,
				})
			}
		}
	}

	if options.Prune && len(b.pruneObjects) > 0 {
		sort.Sort(sort.Reverse(resource.SortableObjects(b.pruneObjects)))
		tasks = append(tasks, &task.PruneTask{
			DryRun:       options.DryRun,
			FieldManager: options.FieldManager,

			Objects: b.pruneObjects,
			Client:  b.Client,
			Mapper:  b.Mapper,
		})
	}

	tasks = append(tasks, &task.InventoryTask{
		Manager: b.Manager,
		DryRun:  options.DryRun,
	})

	queue := make(chan runner.Task, len(tasks))
	for _, task := range tasks {
		queue <- task
	}

	defer close(queue)
	return queue, nil
}
