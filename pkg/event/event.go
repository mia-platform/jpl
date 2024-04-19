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

package event

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Type determines the type of events that are available.
//
//go:generate ${TOOLS_BIN}/stringer -type=Type -trimprefix=Type
type Type int

const (
	TypeError Type = iota
	TypeQueue
	TypeApply
	TypePrune
	TypeInventory
)

// Status determine the status of events that are available.
//
//go:generate ${TOOLS_BIN}/stringer -type=Status -trimprefix=Status
type Status int

const (
	StatusPending Status = iota
	StatusSuccessful
	StatusFailed
)

// Event is the basic block for encapsulate the progression of a task or queue during its execution, more state
// can be encapsulated extending this struct but
type Event struct {
	Type Type

	// ErrorInfo contains info for a TypeError event
	ErrorInfo ErrorInfo

	// QueueInfo contains info for a TypeQueue event
	QueueInfo QueueInfo

	// ApplyInfo contains info for a TypeApply event
	ApplyInfo ApplyInfo

	// PruneInfo contains info for a PruneInfo event
	PruneInfo PruneInfo

	// InventoryInfo contains info for a TypeInventory event
	InventoryInfo InventoryInfo
}

// IsErrorEvent can be used to check if the error contains some type of error
func (e Event) IsErrorEvent() bool {
	switch e.Type {
	case TypeError:
		return true
	case TypeApply:
		return e.ApplyInfo.Error != nil
	case TypePrune:
		return e.PruneInfo.Error != nil
	case TypeInventory:
		return e.InventoryInfo.Error != nil
	default:
		return false
	}
}

// String can be used for logging, the base struct will only print what type of event this is and the error if available
func (e Event) String() string {
	switch e.Type {
	case TypeQueue:
		return e.QueueInfo.String()
	case TypeError:
		return e.ErrorInfo.String()
	case TypeApply:
		return e.ApplyInfo.String()
	case TypePrune:
		return e.PruneInfo.String()
	case TypeInventory:
		return e.InventoryInfo.String()
	default:
		return "event type unknown"
	}
}

type ErrorInfo struct {
	Error error
}

func (i ErrorInfo) String() string {
	return i.Error.Error()
}

type QueueInfo struct {
	Objects []*unstructured.Unstructured
}

func (i QueueInfo) String() string {
	objIDs := make([]string, 0, len(i.Objects))
	for _, obj := range i.Objects {
		objIDs = append(objIDs, identifierFromObject(obj))
	}

	return fmt.Sprintf("queue started for: %s", objIDs)
}

type ApplyInfo struct {
	Object *unstructured.Unstructured
	Status Status
	Error  error
}

func (i ApplyInfo) String() string {
	objID := identifierFromObject(i.Object)
	switch i.Status {
	case StatusPending:
		return fmt.Sprintf("%s: apply started...", objID)
	case StatusSuccessful:
		return fmt.Sprintf("%s: applied successfully", objID)
	case StatusFailed:
		return fmt.Sprintf("%s: failed to apply: %s", objID, i.Error)
	default:
		return fmt.Sprintf("%s: apply status unknown", objID)
	}
}

type PruneInfo struct {
	Object *unstructured.Unstructured
	Status Status
	Error  error
}

func (i PruneInfo) String() string {
	objID := identifierFromObject(i.Object)
	switch i.Status {
	case StatusPending:
		return fmt.Sprintf("%s: prune started...", objID)
	case StatusSuccessful:
		return fmt.Sprintf("%s: pruned successfully", objID)
	case StatusFailed:
		return fmt.Sprintf("%s: failed to prune: %s", objID, i.Error)
	default:
		return fmt.Sprintf("%s: prune status unknown", objID)
	}
}

type InventoryInfo struct {
	Status Status
	Error  error
}

func (i InventoryInfo) String() string {
	switch i.Status {
	case StatusPending:
		return "inventory: apply started..."
	case StatusSuccessful:
		return "inventory: applied successfully"
	case StatusFailed:
		return fmt.Sprintf("inventory: failed to apply: %s", i.Error)
	default:
		return "inventory: apply status unknown"
	}
}

// identifierFromObject return a string to print that identify the obj
func identifierFromObject(obj *unstructured.Unstructured) string {
	return fmt.Sprintf("%s %s", obj.GroupVersionKind().GroupKind().String(), obj.GetName())
}
