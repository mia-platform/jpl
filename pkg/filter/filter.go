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

package filter

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Interface defines the interface for a filter that can choose to remove or keep objects from application
type Interface interface {
	// Filter receive a resource and return if has to be filtered out or an error if something has gone wrong
	Filter(*unstructured.Unstructured) (bool, error)
}
