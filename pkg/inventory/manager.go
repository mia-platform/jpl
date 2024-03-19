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
)

type objectStatus int

const (
	objectStatusApplySuccessfull objectStatus = iota
	objectStatusApplyFailed
	objectStatusDeleteSuccessfull
	objectStatusDeleteFailed
)

// Manager will save and manage the current state of objects for
type Manager struct {
	inventory Store

	objectStatuses map[*unstructured.Unstructured]objectStatus
}

// NewManager create a new instace of a manger for the given inventory Store
func NewManager(inventory Store) *Manager {
	return &Manager{
		inventory:      inventory,
		objectStatuses: make(map[*unstructured.Unstructured]objectStatus, 0),
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

// SetSuccessfullDelete keep track of the passed objs as successfully deleted
func (m *Manager) SetSuccessfullDelete(obj *unstructured.Unstructured) {
	m.setStatus(obj, objectStatusDeleteSuccessfull)
}

// SetFailedDelete keep track of the passed objs as failed to delete
func (m *Manager) SetFailedDelete(obj *unstructured.Unstructured) {
	m.setStatus(obj, objectStatusDeleteFailed)
}

// SaveCurrentInventoryState will use the current tracked objects statuses for creating a new inventory status
// and persist it to the remote server
func (m *Manager) SaveCurrentInventoryState(ctx context.Context, dryRun bool) error {
	// oldInventory, err := m.inventory.Load(ctx)
	// if err != nil {
	// 	return fmt.Errorf("error while loading previous inventory: %w", err)
	// }

	newInventory := sets.New[*unstructured.Unstructured]()

	// add all object that was applied successfully
	newInventory = newInventory.Union(m.objectsForStatus(objectStatusApplySuccessfull))
	// add all object that failed to be pruned for not leaving abandoned objects in the cluster
	newInventory = newInventory.Union(m.objectsForStatus(objectStatusDeleteFailed))

	// TODO: track the intersection of the oldInventory objects with the failed applies, and only save object that where
	// previously tracked.
	newInventory = newInventory.Union(m.objectsForStatus(objectStatusApplyFailed))

	m.inventory.SetObjects(newInventory)
	return m.inventory.Save(ctx, dryRun)
}

// DeleteRemoteInventoryIfPossible calling this method will remove the remote invetory storage if possible.
// If any object has been marked as failed to delete for any reason, we cannot remove
func (m *Manager) DeleteRemoteInventoryIfPossible(ctx context.Context, dryRun bool) error {
	// if applies or deletes are failing we cannot remove the inventory for not leaving objects stranding in the cluster
	if len(m.objectsForStatus(objectStatusDeleteFailed)) != 0 {
		return nil
	}

	return m.inventory.Delete(ctx, dryRun)
}
