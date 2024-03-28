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
	"time"

	"github.com/mia-platform/jpl/pkg/event"
	"github.com/mia-platform/jpl/pkg/generator"
	"github.com/mia-platform/jpl/pkg/inventory"
	"github.com/mia-platform/jpl/pkg/runner"
	"github.com/mia-platform/jpl/pkg/runner/task"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// Applier can be used for appling a list of resources to a remote api-server
type Applier struct {
	mapper      meta.RESTMapper
	client      dynamic.Interface
	infoFetcher task.InfoFetcher

	runner     runner.TaskRunner
	manager    *inventory.Manager
	generators []generator.Interface
}

// ApplierOptions options for the apply step
type ApplierOptions struct {
	DryRun       bool
	Timeout      time.Duration
	FieldManager string
}

// Run will apply the passed objects to a remote api-server
func (a *Applier) Run(ctx context.Context, objects []*unstructured.Unstructured, options ApplierOptions) <-chan event.Event {
	eventChannel := make(chan event.Event)

	go func() {
		defer close(eventChannel)

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
					eventChannel <- event.Event{
						Type: event.TypeError,
						ErrorInfo: event.ErrorInfo{
							Error: fmt.Errorf("generate resource failed: %w", err),
						},
					}
					continue
				}
				generatedObject = append(generatedObject, generated...)
			}
		}

		objects = append(objects, generatedObject...)
		queueBuilder := QueueBuilder{
			Client:      a.client,
			Mapper:      a.mapper,
			Manager:     a.manager,
			InfoFetcher: a.infoFetcher,
		}
		queueOptions := QueueOptions{
			DryRun:       options.DryRun,
			Prune:        true,
			FieldManager: options.FieldManager,
		}

		contextState := &RunnerState{
			eventChannel: eventChannel,
			manager:      a.manager,
			context:      applierCtx,
		}

		tasksQueue := queueBuilder.
			WithObjects(objects).
			Build(queueOptions)

		if err := a.runner.RunWithQueue(contextState, tasksQueue); err != nil {
			eventChannel <- event.Event{
				Type: event.TypeError,
				ErrorInfo: event.ErrorInfo{
					Error: err,
				},
			}
		}
	}()

	return eventChannel
}
