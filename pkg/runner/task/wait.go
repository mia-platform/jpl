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

	"github.com/mia-platform/jpl/pkg/event"
	"github.com/mia-platform/jpl/pkg/inventory"
	"github.com/mia-platform/jpl/pkg/poller"
	"github.com/mia-platform/jpl/pkg/resource"
	"github.com/mia-platform/jpl/pkg/runner"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
)

// keep it to always check if WaitTask implement correctly the Task interface
var _ runner.Task = &WaitTask{}

// WaitTask is the task used for removing Objects from the remote server
type WaitTask struct {
	Objects []*unstructured.Unstructured
	Poller  poller.StatusPoller
	Mapper  meta.RESTMapper
	Manager *inventory.Manager

	objectsToWatch sets.Set[resource.ObjectMetadata]
}

// Run implement the runner.Task interface
func (t *WaitTask) Run(state runner.State) {
	ctx, cancel := context.WithCancel(state.GetContext())

	t.objectsToWatch = sets.New[resource.ObjectMetadata]()
	pollerObjects := make([]*unstructured.Unstructured, 0)
	for _, obj := range t.Objects {
		if state.SkipWaitCurrentStatus(obj) {
			continue
		}

		pollerObjects = append(pollerObjects, obj)
		t.objectsToWatch.Insert(resource.ObjectMetadataFromUnstructured(obj))
	}

	if len(pollerObjects) == 0 {
		cancel()
		return
	}

	pollerCh := t.Poller.Start(ctx, pollerObjects)
	resetMapper := false
	for {
		msg, open := <-pollerCh
		if !open {
			cancel()
			break
		}

		// avoid to send events for an object that has been already received as ready
		if !t.objectsToWatch.Has(msg.StatusUpdateInfo.ObjectMetadata) {
			continue
		}

		state.SendEvent(msg)

		if msg.Type == event.TypeStatusUpdate && msg.StatusUpdateInfo.Status == event.StatusSuccessful {
			t.objectsToWatch.Delete(msg.StatusUpdateInfo.ObjectMetadata)
			if resource.MetadataIsCRD(msg.StatusUpdateInfo.ObjectMetadata) {
				resetMapper = true
			}
		}

		if len(t.objectsToWatch) == 0 {
			cancel()
			break
		}
	}

	if resetMapper {
		meta.MaybeResetRESTMapper(t.Mapper)
	}
}
