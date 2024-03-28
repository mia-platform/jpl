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

package runner

import (
	"context"

	"github.com/mia-platform/jpl/pkg/event"
)

var _ State = &FakeState{}

type FakeState struct {
	Context    context.Context
	SentEvents []event.Event
}

func (s *FakeState) GetContext() context.Context {
	return s.Context
}

func (s *FakeState) SendEvent(event event.Event) {
	s.SentEvents = append(s.SentEvents, event)
}
