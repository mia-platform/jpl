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

package poller

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/mia-platform/jpl/pkg/event"
)

var _ StatusPoller = &FakePoller{}

// FakePoller is used to test correct behaviour of code that will work with events sent from a StatusPoller
type FakePoller struct{}

// Start implement StatusPoller, but it will use the objs to generate the relevant statuses to return to the channel
func (p *FakePoller) Start(ctx context.Context, objs []*unstructured.Unstructured) <-chan event.Event {
	eventCh := make(chan event.Event)

	go func() {
		defer close(eventCh)

		for _, obj := range objs {
			if ctx.Err() != nil {
				break
			}

			result, err := statusCheck(obj, nil)
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
