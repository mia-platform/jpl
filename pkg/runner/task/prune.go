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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"

	"github.com/mia-platform/jpl/pkg/event"
	"github.com/mia-platform/jpl/pkg/runner"
)

// keep it to always check if PruneTask implement correctly the Task interface
var _ runner.Task = &PruneTask{}

// PruneTask is the task used for removing Objects from the remote server
type PruneTask struct {
	DryRun       bool
	FieldManager string
	Client       dynamic.Interface
	Mapper       meta.RESTMapper

	Objects []*unstructured.Unstructured
}

// Run implement the runner.Task interface
func (t *PruneTask) Run(state runner.State) {
	ctx := state.GetContext()
	for _, obj := range t.Objects {
		state.SendEvent(pruneEvent(event.StatusPending, obj, nil))
		if err := pruneObject(ctx, t.Mapper, t.Client, obj, t.DryRun); err != nil {
			// if the object is already missing don't return an error
			if !apierrors.IsNotFound(err) {
				state.SendEvent(pruneEvent(event.StatusFailed, obj, err))
			}
			continue
		}

		state.SendEvent(pruneEvent(event.StatusSuccessful, obj, nil))
	}
}

// pruneEvent create an Event for a prune action with the passed object and status
func pruneEvent(status event.Status, obj *unstructured.Unstructured, err error) event.Event {
	return event.Event{
		Type: event.TypePrune,
		PruneInfo: event.PruneInfo{
			Status: status,
			Object: obj,
			Error:  err,
		},
	}
}

// pruneObject will delete the passed objMeta object in the remote cluster
func pruneObject(ctx context.Context, mapper meta.RESTMapper, client dynamic.Interface, obj *unstructured.Unstructured, dryRun bool) error {
	mapping, err := mapper.RESTMapping(obj.GroupVersionKind().GroupKind())
	if err != nil {
		return err
	}
	propagationPolicy := metav1.DeletePropagationForeground
	opts := metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	}

	if dryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}
	return client.Resource(mapping.Resource).Namespace(obj.GetNamespace()).Delete(ctx, obj.GetName(), opts)
}
