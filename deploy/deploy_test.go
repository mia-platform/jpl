// Copyright 2022 Mia srl
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

package jpl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicFake "k8s.io/client-go/dynamic/fake"
)

func TestEnsureNamespaceExistance(t *testing.T) {
	t.Run("Ensure Namespace existance", func(t *testing.T) {
		namespaceName := "foo"
		scheme := runtime.NewScheme()
		_ = corev1.AddToScheme(scheme)
		dynamicClient := dynamicFake.NewSimpleDynamicClient(scheme)
		clients := K8sClients{dynamic: dynamicClient}

		err := ensureNamespaceExistence(&clients, namespaceName)
		require.Nil(t, err, "No errors when namespace does not exists")

		_, err = dynamicClient.Resource(gvrNamespaces).
			Get(context.Background(), namespaceName, metav1.GetOptions{})
		require.Nil(t, err)

		err = ensureNamespaceExistence(&clients, namespaceName)
		require.Nil(t, err, "No errors when namespace already exists")
		_, err = dynamicClient.Resource(gvrNamespaces).
			Get(context.Background(), namespaceName, metav1.GetOptions{})
		require.Nil(t, err)
	})
}

func TestInitK8sClients(t *testing.T) {
	t.Run("Initialize K8s clients", func(t *testing.T) {
		opts := NewOptions()
		clients := InitRealK8sClients(opts)
		require.NotNil(t, clients, "The returned K8s clients struct should not be nil")
	})
}
