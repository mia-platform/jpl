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
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	// Scheme is the default instance of runtime.Scheme to which types in the Kubernetes API are already registered
	Scheme = runtime.NewScheme()
	// Codecs provides access to encoding and decoding for the scheme
	Codecs = serializer.NewCodecFactory(Scheme)

	codec = Codecs.LegacyCodec(Scheme.PrioritizedVersionsAllGroups()...)
)

// UnstructuredFromFile returns an Unstructured resource reading it from file at path
func UnstructuredFromFile(t *testing.T, path string) *unstructured.Unstructured {
	t.Helper()
	data := ReadBytesFromFile(t, path)
	unst := unstructured.Unstructured{}
	if err := runtime.DecodeInto(codec, data, &unst); err != nil {
		require.FailNow(t, err.Error())
	}
	return &unst
}

// ReadBytesFromFile wrap the login of reading raw bytes from a file at path
func ReadBytesFromFile(t *testing.T, path string) []byte {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		require.FailNow(t, err.Error())
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		require.FailNow(t, err.Error())
	}

	return data
}
