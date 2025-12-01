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
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pkgtesting "github.com/mia-platform/jpl/pkg/testing"
)

func TestBuilder(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		testFactory       *pkgtesting.TestClientFactory
		expectedNamespace string
		expectedType      interface{}
		reader            io.Reader
		path              string
	}{
		"new stream reader": {
			testFactory:  pkgtesting.NewTestClientFactory(),
			reader:       strings.NewReader(""),
			path:         StdinPath,
			expectedType: &StreamReader{},
		},
		"new filepath reader": {
			testFactory:  pkgtesting.NewTestClientFactory().WithNamespace("test-namespace"),
			reader:       strings.NewReader(""),
			path:         filepath.Join("a", "valid", "path"),
			expectedType: &FilepathReader{},
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			t.Parallel()

			builder := NewResourceReaderBuilder(testCase.testFactory)
			require.NotNil(t, builder)

			reader, err := builder.ResourceReader(testCase.reader, testCase.path)
			require.NoError(t, err)
			assert.IsType(t, testCase.expectedType, reader)
		})
	}
}
