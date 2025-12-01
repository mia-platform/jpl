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

package inventory

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/mia-platform/jpl/pkg/resource"
)

type objectStatus int

const (
	objectStatusApplySuccessfull objectStatus = iota
	objectStatusApplyFailed
	objectStatusDeleteSuccessfull
	objectStatusDeleteFailed
	objectStatusSkipped
)

// Manager will save and manage the current state of objects for
type Manager struct {
	Inventory Store

	startingObjects []*unstructured.Unstructured
	objectStatuses  map[*unstructured.Unstructured]objectStatus
}

// NewManager create a new instace of a manger for the given inventory Store
func NewManager(inventory Store, startingObjects []*unstructured.Unstructured) *Manager {
	return &Manager{
		Inventory:       inventory,
		startingObjects: startingObjects,
		objectStatuses:  make(map[*unstructured.Unstructured]objectStatus, 0),
	}
}

// setStatus will update the obj status saved in memory or set it up if is not present
func (m *Manager) setStatus(obj *unstructured.Unstructured, status objectStatus) {
	m.objectStatuses[obj] = status
}

// objectsForStatus return the set of objects saved for the provided status
func (m *Manager) objectsForStatus(status objectStatus) sets.Set[*unstructured.Unstructured] {
	returnObjects := sets.New[*unstructured.Unstructured]()
	for object, objStatus := range m.objectStatuses {
		if objStatus == status {
			returnObjects.Insert(object)
		}
	}

	return returnObjects
}

// SetSuccessfullApply keep track of the passed objs as successfully applied
func (m *Manager) SetSuccessfullApply(obj *unstructured.Unstructured) {
	m.setStatus(obj, objectStatusApplySuccessfull)
}

// SetFailedApply keep track of the passed objs as failed to apply
func (m *Manager) SetFailedApply(obj *unstructured.Unstructured) {
	m.setStatus(obj, objectStatusApplyFailed)
}

// IsFailedApply return if the passed in object has been marked as failed to apply from the manager
func (m *Manager) IsFailedApply(obj *unstructured.Unstructured) bool {
	status, found := m.objectStatuses[obj]
	if !found {
		return false
	}

	return status == objectStatusApplyFailed
}

// SetSuccessfullDelete keep track of the passed objs as successfully deleted
func (m *Manager) SetSuccessfullDelete(obj *unstructured.Unstructured) {
	m.setStatus(obj, objectStatusDeleteSuccessfull)
}

// SetFailedDelete keep track of the passed objs as failed to delete
func (m *Manager) SetFailedDelete(obj *unstructured.Unstructured) {
	m.setStatus(obj, objectStatusDeleteFailed)
}

// SetSkipped keep track of the passed objs as skipped
func (m *Manager) SetSkipped(obj *unstructured.Unstructured) {
	m.setStatus(obj, objectStatusSkipped)
}

// IsSkipped return if the passed in object has been marked as skipped from the manager
func (m *Manager) IsSkipped(obj *unstructured.Unstructured) bool {
	status, found := m.objectStatuses[obj]
	if !found {
		return false
	}

	return status == objectStatusSkipped
}

// SaveCurrentInventoryState will use the current tracked objects statuses for creating a new inventory status
// and persist it to the remote server
func (m *Manager) SaveCurrentInventoryState(ctx context.Context, dryRun bool) error {
	newInventory := sets.New[*unstructured.Unstructured]()

	// add all object that was applied successfully
	newInventory = newInventory.Union(m.objectsForStatus(objectStatusApplySuccessfull))
	// add all object that failed to be pruned for not leaving abandoned objects in the cluster
	newInventory = newInventory.Union(m.objectsForStatus(objectStatusDeleteFailed))

	// add all objects that failed to apply only if they are already been tracked
	applyFailed := m.intersectedObjects(m.objectsForStatus(objectStatusApplyFailed), m.startingObjects)
	newInventory = newInventory.Union(applyFailed)

	// add all skipped objects only if they are already been tracked
	skipped := m.intersectedObjects(m.objectsForStatus(objectStatusSkipped), m.startingObjects)
	newInventory = newInventory.Union(skipped)

	m.Inventory.SetObjects(newInventory)
	return m.Inventory.Save(ctx, dryRun)
}

// DeleteRemoteInventoryIfPossible calling this method will remove the remote invetory storage if possible.
// If any object has been marked as failed to delete for any reason, we cannot remove
func (m *Manager) DeleteRemoteInventoryIfPossible(ctx context.Context, dryRun bool) error {
	// if applies or deletes are failing we cannot remove the inventory for not leaving objects stranding in the cluster
	if len(m.objectsForStatus(objectStatusDeleteFailed)) != 0 {
		return nil
	}

	return m.Inventory.Delete(ctx, dryRun)
}

// intersectedObjects return objects that are contained in first and second
func (m *Manager) intersectedObjects(first sets.Set[*unstructured.Unstructured], second []*unstructured.Unstructured) sets.Set[*unstructured.Unstructured] {
	intersectionSet := make(sets.Set[resource.ObjectMetadata], len(second))
	for _, obj := range second {
		intersectionSet.Insert(resource.ObjectMetadataFromUnstructured(obj))
	}

	intersection := sets.New[*unstructured.Unstructured]()
	for obj := range first {
		if intersectionSet.Has(resource.ObjectMetadataFromUnstructured(obj)) {
			intersection.Insert(obj)
		}
	}

	return intersection
}
