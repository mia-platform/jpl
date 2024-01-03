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

package testing

import (
	"net/http"

	"k8s.io/apimachinery/pkg/runtime"
)

// DefaultHeaders return a base http.Header configured to mimik an api-server response headers
func DefaultHeaders() http.Header {
	headers := http.Header{}
	headers.Set("Content-Type", runtime.ContentTypeJSON)
	return headers
}
