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

	"github.com/mia-platform/jpl/pkg/inventory"
	"github.com/mia-platform/jpl/pkg/resource"
	"github.com/mia-platform/jpl/pkg/runner"
	"github.com/mia-platform/jpl/pkg/runner/task"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

type QueueOptions struct {
	DryRun       bool
	Prune        bool
	FieldManager string
}

type QueueBuilder struct {
	objects      []*unstructured.Unstructured
	pruneObjects []*unstructured.Unstructured

	Manager     *inventory.Manager
	Client      dynamic.Interface
	Mapper      meta.RESTMapper
	InfoFetcher task.InfoFetcher
}

func (b *QueueBuilder) WithObjects(objs []*unstructured.Unstructured) *QueueBuilder {
	b.objects = objs
	return b
}

func (b *QueueBuilder) WithPruneObjects(objs []*unstructured.Unstructured) *QueueBuilder {
	b.pruneObjects = objs
	return b
}

func (b *QueueBuilder) Build(options QueueOptions) chan runner.Task {
	totalActionLenght := 1
	pruneObjects := options.Prune && len(b.pruneObjects) > 0
	applyObjects := len(b.objects) > 0

	if applyObjects {
		totalActionLenght++
	}

	if pruneObjects {
		totalActionLenght++
	}
	queue := make(chan runner.Task, totalActionLenght)

	if applyObjects {
		sort.Sort(resource.SortableObjects(b.objects))
		queue <- &task.ApplyTask{
			DryRun:       options.DryRun,
			FieldManager: options.FieldManager,

			Objects:     b.objects,
			InfoFetcher: b.InfoFetcher,
		}
	}

	if pruneObjects {
		sort.Sort(sort.Reverse(resource.SortableObjects(b.pruneObjects)))
		queue <- &task.PruneTask{
			DryRun:       options.DryRun,
			FieldManager: options.FieldManager,

			Objects: b.pruneObjects,
			Client:  b.Client,
			Mapper:  b.Mapper,
		}
	}

	queue <- &task.InventoryTask{
		Manager: b.Manager,
		DryRun:  options.DryRun,
	}

	defer close(queue)
	return queue
}
