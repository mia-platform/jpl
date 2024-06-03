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

package poller

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
)

type Builder interface {
	NewPoller(dynamic.Interface, meta.RESTMapper) StatusPoller
}

type DefaultBuilder struct{}

func (b *DefaultBuilder) NewPoller(client dynamic.Interface, mapper meta.RESTMapper) StatusPoller {
	return NewDefaultStatusPoller(client, mapper)
}
