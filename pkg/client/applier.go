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
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/mia-platform/jpl/pkg/generator"
	"github.com/mia-platform/jpl/pkg/resource"
	"github.com/mia-platform/jpl/pkg/runner"
	"github.com/mia-platform/jpl/pkg/runner/task"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Applier can be used for appling a list of resources to a remote api-server
type Applier struct {
	infoFetcher task.InfoFetcher
	mapper      meta.RESTMapper
	runner      runner.TaskRunner
	generators  []generator.Interface
}

// ApplierOptions options for the apply step
type ApplierOptions struct {
	DryRun       bool
	Timeout      time.Duration
	FieldManager string
}

// Run will apply the passed objects to a remote api-server
func (a *Applier) Run(ctx context.Context, objects []*unstructured.Unstructured, options ApplierOptions) error {
	applierCtx := ctx
	if options.Timeout > 0 {
		var cancel context.CancelFunc
		applierCtx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}

	var generatedObject []*unstructured.Unstructured
	for _, rg := range a.generators {
		for _, obj := range objects {
			objMetadata := meta.AsPartialObjectMetadata(obj)
			objMetadata.TypeMeta = metav1.TypeMeta{
				Kind:       obj.GetKind(),
				APIVersion: obj.GetAPIVersion(),
			}

			if !rg.CanHandleResource(objMetadata) {
				continue
			}

			generated, err := rg.Generate(obj)
			if err != nil {
				fmt.Fprintf(os.Stderr, "generate resource failed: %q", err)
				continue
			}
			generatedObject = append(generatedObject, generated...)
		}
	}

	objects = append(objects, generatedObject...)
	sort.Sort(resource.SortableObjects(objects))

	tasksQueue := taskQueue(objects, a.infoFetcher, options)
	return a.runner.RunWithQueue(applierCtx, tasksQueue)
}

// taskQueue transform an array of object in a channel of tasks for using it with a runner
func taskQueue(objects []*unstructured.Unstructured, infoFetcher task.InfoFetcher, options ApplierOptions) chan runner.Task {
	queue := make(chan runner.Task, len(objects))

	for _, obj := range objects {
		queue <- &task.ApplyTask{
			DryRun:       options.DryRun,
			FieldManager: options.FieldManager,

			Objects:     []*unstructured.Unstructured{obj},
			InfoFetcher: infoFetcher,
		}
	}

	defer close(queue)
	return queue
}
