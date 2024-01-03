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
	"io"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// StdinPath is the string expected for reading resources from reader instead of a local path
const StdinPath = "-"

// Reader defines the interface for reading a set of data in Resource objects.
type Reader interface {
	// Read reads data from the Reader and parse them in Resource objects.
	Read() ([]*unstructured.Unstructured, error)
}

// ReaderConfigs common configurations between different Readers
type ReaderConfigs struct {
	Mapper           meta.RESTMapper
	Namespace        string
	EnforceNamespace bool
}

// Builder defines the interface for creating the correct Reader and cofigure it
type Builder interface {
	// ResourceReader return a Reader implementation based on the reader and path passed as arguments
	ResourceReader(reader io.Reader, path string) (Reader, error)
}
