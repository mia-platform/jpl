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
	"io"

	"github.com/mia-platform/jpl/pkg/util"
)

// NewResourceReaderBuilder returns an instance of Builder.
func NewResourceReaderBuilder(f util.ClientFactory) Builder {
	return &builder{
		factory: f,
	}
}

type builder struct {
	factory util.ClientFactory
}

// ResourceReader implement the Builder interface
func (b *builder) ResourceReader(reader io.Reader, path string) (Reader, error) {
	namespace, enforceNamespace, err := b.factory.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return nil, fmt.Errorf("error while reading kubernetes config: %w", err)
	}

	mapper, err := b.factory.ToRESTMapper()
	if err != nil {
		return nil, fmt.Errorf("error while retrieving mapper: %w", err)
	}

	readerConfig := ReaderConfigs{
		Mapper:           mapper,
		Namespace:        namespace,
		EnforceNamespace: enforceNamespace,
	}

	var resourceReader Reader
	if path == StdinPath {
		resourceReader = &StreamReader{
			Reader:        reader,
			ReaderConfigs: readerConfig,
		}
	} else {
		resourceReader = &FilepathReader{
			Path:          path,
			ReaderConfigs: readerConfig,
		}
	}

	return resourceReader, nil
}
