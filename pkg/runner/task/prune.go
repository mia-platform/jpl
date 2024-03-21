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

	"github.com/mia-platform/jpl/pkg/resource"
	"github.com/mia-platform/jpl/pkg/runner"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/dynamic"
)

// keep it to always check if PruneTask implement correctly the Task interface
var _ runner.Task = &PruneTask{}

// PruneTask is the task used for removing Objects from the remote server
type PruneTask struct {
	DryRun       bool
	FieldManager string
	Client       dynamic.Interface
	Mapper       meta.RESTMapper

	Objects []resource.ObjectMetadata

	cancel context.CancelFunc
}

// Run implement the runner.Task interface
func (t *PruneTask) Run(state runner.CurrentState) error {
	withCancel, cancel := context.WithCancel(state.GetContext())
	t.cancel = cancel
	defer t.Cancel()

	var errList []error
	for _, objMeta := range t.Objects {
		if err := pruneObject(withCancel, t.Mapper, t.Client, objMeta, t.DryRun); err != nil {
			// if the object is already missing don't return an error
			if !apierrors.IsNotFound(err) {
				errList = append(errList, err)
			}
		}
	}

	return utilerrors.NewAggregate(errList)
}

// Cancel implement the runner.Task interface
func (t *PruneTask) Cancel() {
	if t.cancel != nil {
		t.cancel()
	}
}

// pruneObject will delete the passed objMeta object in the remote cluster
func pruneObject(ctx context.Context, mapper meta.RESTMapper, client dynamic.Interface, objMeta resource.ObjectMetadata, dryRun bool) error {
	mapping, err := mapper.RESTMapping(schema.GroupKind{Group: objMeta.Group, Kind: objMeta.Kind})
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
	return client.Resource(mapping.Resource).Namespace(objMeta.Namespace).Delete(ctx, objMeta.Name, opts)
}
