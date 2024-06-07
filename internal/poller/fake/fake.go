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

package fake

import (
	"context"

	"github.com/mia-platform/jpl/internal/poller"
	"github.com/mia-platform/jpl/pkg/event"
	"github.com/mia-platform/jpl/pkg/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ poller.StatusPoller = &TestPoller{}

// TestPoller is used to test correct behaviour of code that will work with events sent from a StatusPoller
type TestPoller struct{}

// Start implement StatusPoller, but it will use the objs to generate the relevant statuses to return to the channel
func (p *TestPoller) Start(ctx context.Context, objs []*unstructured.Unstructured) <-chan event.Event {
	eventCh := make(chan event.Event)

	go func() {
		defer close(eventCh)

		for _, obj := range objs {
			if ctx.Err() != nil {
				break
			}

			result, err := poller.StatusCheck(obj)
			if err != nil {
				eventCh <- event.Event{
					Type: event.TypeError,
					ErrorInfo: event.ErrorInfo{
						Error: err,
					},
				}
				break
			}

			eventCh <- eventFromResult(result, obj)
		}
	}()

	return eventCh
}

func eventFromResult(result *poller.Result, obj *unstructured.Unstructured) event.Event {
	statusEvent := event.Event{
		Type: event.TypeStatusUpdate,
		StatusUpdateInfo: event.StatusUpdateInfo{
			Message:        result.Message,
			ObjectMetadata: resource.ObjectMetadataFromUnstructured(obj),
		},
	}
	switch result.Status {
	case poller.StatusCurrent:
		statusEvent.StatusUpdateInfo.Status = event.StatusSuccessful
	case poller.StatusInProgress, poller.StatusTerminating:
		statusEvent.StatusUpdateInfo.Status = event.StatusPending
	case poller.StatusFailed:
		statusEvent.StatusUpdateInfo.Status = event.StatusFailed
	}

	return statusEvent
}
