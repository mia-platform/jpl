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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/kyaml/kio"
)

// keep it to always check if FilepathReader implement correctly the Reader interface
var _ Reader = &FilepathReader{}

type FilepathReader struct {
	ReaderConfigs

	Path string
}

// Read implement the Reader interface
func (r *FilepathReader) Read() ([]*unstructured.Unstructured, error) {
	reader := &kio.LocalPackageReader{
		PackagePath: r.Path,

		OmitReaderAnnotations: true,
	}

	objs, err := objectsFromReader(reader)
	if err != nil {
		return objs, fmt.Errorf("fail to read from path %q: %w", r.Path, err)
	}

	err = setNamespace(r.Mapper, objs, r.Namespace, r.EnforceNamespace)
	return objs, err
}
