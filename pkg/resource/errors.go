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

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// keep it to always check if UnknownResourceTypeError implement correctly the error interface
var _ error = UnknownResourceTypeError{}

// UnknownResourceTypeError is used to signal that an unknown resource has been found
type UnknownResourceTypeError struct {
	ResourceGVK schema.GroupVersionKind
}

// Error implement error interface
func (e UnknownResourceTypeError) Error() string {
	return fmt.Sprintf("unknown resource type: %q", e.ResourceGVK.String())
}

// keep it to always check if ExternalDependencyError implement correctly the error interface
var _ error = ExternalDependencyError{}

// ExternalDependencyError is used to signal that an external explicit dependency has been found
type ExternalDependencyError struct {
	dependency Dependency
}

// Error implement error interface
func (e ExternalDependencyError) Error() string {
	return fmt.Sprintf("external dependency from %s to %s",
		formatObjectMetadata(e.dependency.from),
		formatObjectMetadata(e.dependency.to),
	)
}

func formatObjectMetadata(metadata ObjectMetadata) string {
	gk := fmt.Sprintf("%s/%s", metadata.Group, metadata.Kind)
	if metadata.Group == "" {
		gk = metadata.Kind
	}

	nameRef := fmt.Sprintf("%s/%s", metadata.Namespace, metadata.Name)
	if metadata.Namespace == "" {
		nameRef = metadata.Name
	}

	return fmt.Sprintf("%s %s", gk, nameRef)
}

// keep it to always check if CycleDependenciesError implement correctly the error interface
var _ error = CyclicDependencyError{}

type CyclicDependencyError struct {
	dependencies []Dependency
}

// Error implement error interface
func (e CyclicDependencyError) Error() string {
	builder := new(strings.Builder)
	builder.WriteString("cyclical dependencies:")
	for _, dependency := range e.dependencies {
		builder.WriteString(
			fmt.Sprintf("\n- from %s to %s",
				formatObjectMetadata(dependency.from),
				formatObjectMetadata(dependency.to),
			),
		)
	}

	return builder.String()
}

type Dependency struct {
	from ObjectMetadata
	to   ObjectMetadata
}
