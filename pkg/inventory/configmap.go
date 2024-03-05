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
	"regexp"
	"strings"

	"github.com/mia-platform/jpl/pkg/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clientv1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	separator           = "_"
	colonReplacement    = "__"
	regexTemplate       = "^(%s)%s(%s)%s(%s)%s(%s)$"
	regexNamespaceChars = "[[:alnum:]-.]*[[:alnum:]]?"
	regexNameChars      = "[[:alnum:]][[:alnum:]-._]*[[:alnum:]]?"
	regexGroupChars     = "[[:alnum:]-.]*[[:alnum:]]?"
	regexKindChars      = "[[:alnum:]]*"
)

// keep it to always check if configMapStore implement correctly the Store interface
var _ Store = &configMapStore{}

// configMapStore is an inventory store backed by a ConfigMap saved on the remote server where the
// operations are performed. It only keep track of what resources have been deployed but not their contents.
type configMapStore struct {
	name         string
	namespace    string
	fieldManager string
	clientset    *kubernetes.Clientset
	savedObjects []ResourceMetadata
}

// NewConfigMapStore return a new Store instance configured with the provided factory that will persist
// data via a ConfigMap resource. The namespace is where the backing ConfigMap will be read and saved.
func NewConfigMapStore(factory util.ClientFactory, name, namespace, fieldManager string) (Store, error) {
	var err error
	clientset, err := factory.KubernetesClientSet()
	if err != nil {
		return nil, err
	}

	return &configMapStore{
		name:         name,
		namespace:    namespace,
		fieldManager: fieldManager,
		clientset:    clientset,
	}, err
}

// Save implement Store interface
func (s *configMapStore) Save(ctx context.Context, dryRun bool) error {
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

	return nil
}

// Save implement Store interface
func (s *configMapStore) SetObjects(objects []*unstructured.Unstructured) {
	savedObjects := make([]ResourceMetadata, 0, len(objects))
	for _, resource := range objects {
		savedObjects = append(savedObjects, resourceMetadataFromUnstructured(resource))
	}

	s.savedObjects = savedObjects
}

// Diff implement Store interface
func (s *configMapStore) Diff(ctx context.Context, objects []*unstructured.Unstructured) ([]ResourceMetadata, error) {
	remoteObjects, err := s.load(ctx)
	if err != nil {
		return []ResourceMetadata{}, err
	}

	// remove all objects that we can find inside the returned remote objects
	for _, obj := range objects {
		delete(remoteObjects, resourceMetadataFromUnstructured(obj))
	}

	// construct a slice from the map that is mimiking a set
	diffMetadata := make([]ResourceMetadata, 0, len(remoteObjects))
	for metadata := range remoteObjects {
		diffMetadata = append(diffMetadata, metadata)
	}

	return diffMetadata, nil
}

// load will read the remote storage to retrieve the saved metadata
func (s *configMapStore) load(ctx context.Context) (map[ResourceMetadata]struct{}, error) {
	metadataSet := make(map[ResourceMetadata]struct{}, 0)
	cm, err := s.clientset.CoreV1().ConfigMaps(s.namespace).Get(ctx, s.name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return metadataSet, nil
		}
		return nil, fmt.Errorf("failed to find inventory: %w", err)
	}

	for dataKey := range cm.Data {
		if ok, objMeta := parseObjectMetadataFromKey(dataKey); ok {
			metadataSet[objMeta] = struct{}{}
		}
	}

	return metadataSet, nil
}

// dataForStore create a ConfigMap data map based on the savedObjects inside store.
// The savedObjects would be encoded in a string format for easy storage.
func dataForStore(store *configMapStore) map[string]string {
	data := make(map[string]string)

	for _, objMeta := range store.savedObjects {
		data[keyFromObjectMetadata(objMeta)] = ""
	}

	return data
}

// keyFromObjectMetadata will return a valid and unique ConfigMap key string based on a kubernetes metadata
func keyFromObjectMetadata(obj ResourceMetadata) string {
	// objects in Kubernetes must follow some rules that you can read at the following url
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/
	// here an excerpt:
	// Names must be unique across all API versions of the same resource. API resources are distinguished by their
	// API group, resource type, namespace (for namespaced resources), and name.
	return obj.Namespace + separator +
		strings.ReplaceAll(obj.Name, ":", colonReplacement) + separator +
		obj.Group + separator +
		obj.Kind
}

// parseObjectMetadataFromKey will parse the given key for retrieving a ResourceMetadata, return false if the
// parsed string doesn't match an encoded ResourceMetadata
func parseObjectMetadataFromKey(key string) (bool, ResourceMetadata) {
	re := regexp.MustCompile(fmt.Sprintf(regexTemplate,
		regexNamespaceChars, separator,
		regexNameChars, separator,
		regexGroupChars, separator,
		regexKindChars,
	))

	if submatches := re.FindStringSubmatch(key); len(submatches) > 0 {
		return true, ResourceMetadata{
			Namespace: submatches[1],
			Name:      strings.ReplaceAll(submatches[2], colonReplacement, ":"),
			Group:     submatches[3],
			Kind:      submatches[4],
		}
	}

	return false, ResourceMetadata{}
}

func resourceMetadataFromUnstructured(obj *unstructured.Unstructured) ResourceMetadata {
	gvk := obj.GroupVersionKind()
	name := obj.GetName()
	namespace := obj.GetNamespace()
	return ResourceMetadata{
		Name:      name,
		Namespace: namespace,
		Kind:      gvk.Kind,
		Group:     gvk.Group,
	}
}
