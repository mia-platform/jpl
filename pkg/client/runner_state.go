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

	"github.com/mia-platform/jpl/pkg/event"
	"github.com/mia-platform/jpl/pkg/inventory"
	"github.com/mia-platform/jpl/pkg/runner"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ runner.State = &RunnerState{}

type RunnerState struct {
	eventChannel chan event.Event
	manager      *inventory.Manager
	context      context.Context
}

func (s *RunnerState) GetContext() context.Context {
	return s.context
}

func (s *RunnerState) SendEvent(e event.Event) {
	switch e.Type {
	case event.TypeApply:
		s.registerEventInManager(e.Type, e.ApplyInfo.Status, e.ApplyInfo.Object)
	case event.TypePrune:
		s.registerEventInManager(e.Type, e.PruneInfo.Status, e.PruneInfo.Object)
	}
	s.eventChannel <- e
}

func (s *RunnerState) registerEventInManager(eventType event.Type, status event.Status, obj *unstructured.Unstructured) {
	switch {
	case eventType == event.TypeApply && status == event.StatusSuccessful:
		s.manager.SetSuccessfullApply(obj)
	case eventType == event.TypeApply && status == event.StatusFailed:
		s.manager.SetFailedApply(obj)
	case eventType == event.TypeApply && status == event.StatusSkipped:
		s.manager.SetSkipped(obj)
	case eventType == event.TypePrune && status == event.StatusSuccessful:
		s.manager.SetSuccessfullDelete(obj)
	case eventType == event.TypePrune && status == event.StatusFailed:
		s.manager.SetFailedDelete(obj)
	}
}

func (s *RunnerState) SkipWaitCurrentStatus(obj *unstructured.Unstructured) bool {
	return s.manager.IsFailedApply(obj) || s.manager.IsSkipped(obj)
}
