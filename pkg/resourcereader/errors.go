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

package resourcereader

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// keep it to always check if EnforcedNamespaceError implement correctly the error interface
var _ error = EnforcedNamespaceError{}

// EnforcedNamespaceError is used if a resource is found with a different namespace then EnforcedNamespace
type EnforcedNamespaceError struct {
	EnforcedNamespace string
	NamespaceFound    string
	ResourceGVK       schema.GroupVersionKind
}

// Error implements the error interface
func (e EnforcedNamespaceError) Error() string {
	return fmt.Sprintf("found resource %q in namespace %q, but all resources must be in namespace %q",
		e.ResourceGVK, e.NamespaceFound, e.EnforcedNamespace)
}
