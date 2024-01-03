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
