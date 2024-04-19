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

package resource

import (
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	separator        = "_"
	colonReplacement = "__"
	regexTemplate    = "^(%s)%s(%s)%s(%s)%s(%s)$"
	namespaceChars   = "[[:alnum:]-.]*[[:alnum:]]?"
	nameChars        = "[[:alnum:]][[:alnum:]-._]*[[:alnum:]]?"
	groupChars       = "[[:alnum:]-.]*[[:alnum:]]?"
	kindChars        = "[[:alnum:]]*"
)

var emptyMetadata = ObjectMetadata{}

// ObjectMetadata reppresent the minimum subset of data to uniquely identify a resource deployed on a remote cluster
type ObjectMetadata struct {
	Name      string
	Namespace string
	Group     string
	Kind      string
}

// ObjectMetadataFromUnstructured transform an unstructured data to a subset of metadata useful identification
func ObjectMetadataFromUnstructured(obj *unstructured.Unstructured) ObjectMetadata {
	gvk := obj.GroupVersionKind()
	return ObjectMetadata{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Kind:      gvk.Kind,
		Group:     gvk.Group,
	}
}

// ToString return a string rappresentation of the metadata
func (m ObjectMetadata) ToString() string {
	// objects in Kubernetes must follow some rules that you can read at the following url
	// https://kubernetes.io/docs/concepts/overview/working-with-objects/names/
	// here an excerpt:
	// Names must be unique across all API versions of the same resource. API resources are distinguished by their
	// API group, resource type, namespace (for namespaced resources), and name.
	return m.Namespace + separator +
		strings.ReplaceAll(m.Name, ":", colonReplacement) + separator +
		m.Group + separator +
		m.Kind
}

// ObjectMetadataFromString will parse the given string and will try to extract all the metadata encoded in it.
// Will return true if the string is compatible with an ObjectMetadata string rappresentation, false if the string
// is something else
func ObjectMetadataFromString(str string) (bool, ObjectMetadata) {
	regexString := fmt.Sprintf(regexTemplate,
		namespaceChars, separator,
		nameChars, separator,
		groupChars, separator,
		kindChars)

	re := regexp.MustCompile(regexString)
	if submatches := re.FindStringSubmatch(str); len(submatches) > 0 {
		return true, ObjectMetadata{
			Namespace: submatches[1],
			Name:      strings.ReplaceAll(submatches[2], colonReplacement, ":"),
			Group:     submatches[3],
			Kind:      submatches[4],
		}
	}

	return false, emptyMetadata
}
