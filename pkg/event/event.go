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

	"github.com/mia-platform/jpl/pkg/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
	TypeStatusUpdate
)

// Status determine the status of events that are available.
//
//go:generate ${TOOLS_BIN}/stringer -type=Status -trimprefix=Status
type Status int

const (
	StatusPending Status = iota
	StatusSuccessful
	StatusFailed
	StatusSkipped
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

	// StatusUpdateInfo contains info for a TypeStatusUpdate event
	StatusUpdateInfo StatusUpdateInfo
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
	case TypeStatusUpdate:
		return e.StatusUpdateInfo.Status == StatusFailed
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
	case TypeStatusUpdate:
		return e.StatusUpdateInfo.String()
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
		return objID + ": apply started..."
	case StatusSuccessful:
		return objID + ": applied successfully"
	case StatusSkipped:
		return objID + ": apply skipped"
	case StatusFailed:
		return objID + ": failed to apply: " + i.Error.Error()
	default:
		return objID + ": apply status unknown"
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
		return objID + ": prune started..."
	case StatusSuccessful:
		return objID + ": pruned successfully"
	case StatusFailed:
		return objID + ": failed to prune: " + i.Error.Error()
	default:
		return objID + ": prune status unknown"
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

type StatusUpdateInfo struct {
	Status         Status
	Message        string
	ObjectMetadata resource.ObjectMetadata
}

func (i StatusUpdateInfo) String() string {
	gk := schema.GroupKind{
		Group: i.ObjectMetadata.Group,
		Kind:  i.ObjectMetadata.Kind,
	}
	return fmt.Sprintf("%s %s: %s", gk.String(), i.ObjectMetadata.Name, i.Message)
}
