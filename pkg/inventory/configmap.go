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

// ConfigMapStore is an inventory store backed by a ConfigMap saved on the remote server where the
// operations are performed. It only keep track of what resources have been deployed but not their contents.
type ConfigMapStore struct {
	name         string
	namespace    string
	clientset    *kubernetes.Clientset
	savedObjects []ResourceMetadata
}

// NewConfigMapStore return a new ConfigMapClient configured with the provided factory. The namespace is where
// the ConfigMap store will be read and saved.
func NewConfigMapStore(factory util.ClientFactory, name, namespace string) (*ConfigMapStore, error) {
	var err error
	clientset, err := factory.KubernetesClientSet()
	if err != nil {
		return nil, err
	}

	return &ConfigMapStore{
		name:      name,
		namespace: namespace,
		clientset: clientset,
	}, err
}

func (s *ConfigMapStore) Save(ctx context.Context, fieldManager string) error {
	opts := metav1.ApplyOptions{
		Force:        true,
		FieldManager: fieldManager,
	}

	cm := clientv1.ConfigMap(s.name, s.namespace).WithData(dataForStore(s))
	_, err := s.clientset.CoreV1().ConfigMaps(s.namespace).Apply(ctx, cm, opts)
	return err
}

func (s *ConfigMapStore) Load(ctx context.Context) ([]ResourceMetadata, error) {
	cm, err := s.clientset.CoreV1().ConfigMaps(s.namespace).Get(ctx, s.name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return []ResourceMetadata{}, nil
		}
		return nil, fmt.Errorf("failed to find inventory: %w", err)
	}

	metadata := make([]ResourceMetadata, 0)
	for dataKey := range cm.Data {
		if ok, objMeta := parseObjectMetadataFromKey(dataKey); ok {
			metadata = append(metadata, objMeta)
		}
	}

	return metadata, nil
}

// dataForStore create a ConfigMap data map based on the savedObjects inside store.
// The savedObjects would be encoded in a string format for easy storage.
func dataForStore(store *ConfigMapStore) map[string]string {
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
