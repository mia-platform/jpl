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
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	Annotation = "config.kubernetes.io/depends-on"

	// Number of fields for a cluster-scoped depends-on object value. Example:
	//   rbac.authorization.k8s.io/ClusterRole/my-cluster-role-name
	numFieldsClusterScoped = 3
	// Number of fields for a namespace-scoped depends-on object value. Example:
	//   apps/namespaces/my-namespace/Deployment/my-deployment-name
	numFieldsNamespacedScoped = 5
	// Used to separate multiple depends-on objects.
	annotationSeparator = ","
	// Used to separate the fields for a depends-on object value.
	fieldSeparator  = "/"
	namespacesField = "namespaces"
)

// ObjectExplicitDependencies return a set of explicitly set dependencies of the object if any are found or an error
// if one of the dependencies found in the annotation cannot be parsed correctly
func ObjectExplicitDependencies(obj *unstructured.Unstructured) ([]ObjectMetadata, error) {
	if !hasDepedencyAnnotation(obj) {
		return make([]ObjectMetadata, 0), nil
	}

	return unmarshalDepedenciesString(obj.GetAnnotations()[Annotation])
}

// SetObjectExplicitDependencies set an annotation on obj to contains the formatted dependencies as string.
// Return an error if one of the dependencies is malformed.
func SetObjectExplicitDependencies(obj *unstructured.Unstructured, dependencies []ObjectMetadata) error {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	if len(dependencies) == 0 {
		return nil
	}

	dependenciesString, err := marshalDepedenciesString(dependencies)
	if err != nil {
		return err
	}

	annotations[Annotation] = dependenciesString
	obj.SetAnnotations(annotations)
	return nil
}

// hasDepedencyAnnotation control if obj has the annotation for explicit dependencies
func hasDepedencyAnnotation(obj *unstructured.Unstructured) bool {
	annotations := obj.GetAnnotations()
	_, found := annotations[Annotation]
	return found
}

// unmarshalDepedenciesString take a depString and try to extract a set of ObjectMetadata.
// Return the parsed ObjectMetadatas or an error if depString cannot be parsed correctly
func unmarshalDepedenciesString(depString string) ([]ObjectMetadata, error) {
	objDependencies := make([]ObjectMetadata, 0)
	for _, objString := range strings.Split(depString, annotationSeparator) {
		obj, err := parseDependencyObjectMetadata(objString)
		if err != nil {
			return objDependencies, fmt.Errorf("failed to parse object reference: %w", err)
		}
		objDependencies = append(objDependencies, obj)
	}

	return objDependencies, nil
}

// marshalDepedenciesString return a string containing the formatted dependencies.
// Return error if there are malformed depedndecies in the given set.
func marshalDepedenciesString(dependencies []ObjectMetadata) (string, error) {
	depStrings := make([]string, 0, len(dependencies))
	for _, objMetadata := range dependencies {
		objString, err := formatDependencyObjectMetadata(objMetadata)
		if err != nil {
			return "", fmt.Errorf("failed to format object reference: %w", err)
		}
		depStrings = append(depStrings, objString)
	}

	return strings.Join(depStrings, annotationSeparator), nil
}

// parseDependencyObjectMetadata parse metadataString in an ObjectMetadata if possible,
// return error if we cannot extract it.
func parseDependencyObjectMetadata(metadataString string) (ObjectMetadata, error) {
	var group, kind, namespace, name string
	var objMetadata ObjectMetadata
	metadataString = strings.TrimSpace(metadataString)
	stringParts := strings.Split(metadataString, fieldSeparator)

	switch len(stringParts) {
	case numFieldsClusterScoped:
		group = stringParts[0]
		kind = stringParts[1]
		name = stringParts[2]
	case numFieldsNamespacedScoped:
		if stringParts[1] != namespacesField {
			return objMetadata, fmt.Errorf("unexpected string as namespaced resource: %s", metadataString)
		}

		group = stringParts[0]
		namespace = stringParts[2]
		kind = stringParts[3]
		name = stringParts[4]
	default:
		return objMetadata, fmt.Errorf("unexpected field composition: %s", metadataString)
	}

	objMetadata = ObjectMetadata{
		Kind:      kind,
		Group:     group,
		Name:      name,
		Namespace: namespace,
	}

	return objMetadata, nil
}

// formatDependencyObjectMetadata return a formatted string containing the objMetadata information,
// return error if objMetadata is malformed.
func formatDependencyObjectMetadata(objMetadata ObjectMetadata) (string, error) {
	if objMetadata.Kind == "" {
		return "", fmt.Errorf("invalid object metadata: missing resource kind")
	}

	if objMetadata.Name == "" {
		return "", fmt.Errorf("invalid object metadata: missing resource name")
	}

	kind := objMetadata.Kind
	group := objMetadata.Group
	name := objMetadata.Name
	namespace := objMetadata.Namespace
	var metadataString string
	if objMetadata.Namespace == "" {
		metadataString = group + fieldSeparator + kind + fieldSeparator + name
	} else {
		metadataString = group + fieldSeparator + namespacesField + fieldSeparator + namespace + fieldSeparator + kind + fieldSeparator + name
	}
	return metadataString, nil
}
