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
	"fmt"
	"sync"

	"github.com/mia-platform/jpl/pkg/resource"
	"github.com/mia-platform/jpl/pkg/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	clientv1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
)

// keep it to always check if configMapStore implement correctly the Store interface
var _ Store = &configMapStore{}

// configMapStore is an inventory store backed by a ConfigMap saved on the remote server where the
// operations are performed. It only keep track of what resources have been deployed but not their contents.
type configMapStore struct {
	name         string
	namespace    string
	fieldManager string

	clientset    kubernetes.Interface
	savedObjects sets.Set[*unstructured.Unstructured]

	cachedRemoteSet sets.Set[resource.ObjectMetadata]
	lock            sync.Mutex
}

// NewConfigMapStore return a new Store instance configured with the provided factory that will persist
// data via a ConfigMap resource. The namespace is where the backing ConfigMap will be read and saved.
func NewConfigMapStore(factory util.ClientFactory, name, namespace, fieldManager string) (Store, error) {
	clientset, err := factory.KubernetesClientSet()
	if err != nil {
		return nil, err
	}

	return &configMapStore{
		name:         name,
		namespace:    namespace,
		fieldManager: fieldManager,
		clientset:    clientset,
	}, nil
}

// Save implement Store interface
func (s *configMapStore) Save(ctx context.Context, dryRun bool) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	opts := metav1.ApplyOptions{
		Force:        true,
		FieldManager: s.fieldManager,
	}

	if dryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}

	cm := clientv1.ConfigMap(s.name, s.namespace).WithData(dataForStore(s))
	if _, err := s.clientset.CoreV1().ConfigMaps(s.namespace).Apply(ctx, cm, opts); err != nil {
		return fmt.Errorf("failed to save inventory: %w", err)
	}

	s.savedObjects = nil
	return nil
}

// Delete implement Store interface
func (s *configMapStore) Delete(ctx context.Context, dryRun bool) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	propagation := metav1.DeletePropagationBackground
	opts := metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	}

	if dryRun {
		opts.DryRun = []string{metav1.DryRunAll}
	}

	if err := s.clientset.CoreV1().ConfigMaps(s.namespace).Delete(ctx, s.name, opts); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete inventory: %w", err)
	}

	s.savedObjects = nil
	return nil
}

// Save implement Store interface
func (s *configMapStore) SetObjects(objs sets.Set[*unstructured.Unstructured]) {
	s.savedObjects = objs.Clone()
}

// Load will read the remote storage to retrieve the saved metadata
func (s *configMapStore) Load(ctx context.Context) (sets.Set[resource.ObjectMetadata], error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.cachedRemoteSet != nil {
		return s.cachedRemoteSet, nil
	}

	metadataSet := make(sets.Set[resource.ObjectMetadata], 0)
	cm, err := s.clientset.CoreV1().ConfigMaps(s.namespace).Get(ctx, s.name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return metadataSet, nil
		}
		return nil, fmt.Errorf("failed to find inventory: %w", err)
	}

	for dataKey := range cm.Data {
		if ok, objMeta := resource.ObjectMetadataFromString(dataKey); ok {
			metadataSet.Insert(objMeta)
		}
	}

	s.cachedRemoteSet = metadataSet
	return metadataSet, nil
}

// dataForStore create a ConfigMap data map based on the savedObjects inside store.
// The savedObjects would be encoded in a string format for easy storage.
func dataForStore(store *configMapStore) map[string]string {
	data := make(map[string]string)

	for obj := range store.savedObjects {
		data[resource.ObjectMetadataFromUnstructured(obj).ToString()] = ""
	}

	return data
}
