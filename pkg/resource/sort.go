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
	"sort"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SortableObjects internal type for applying the sort interface to an array of Unstructured
type SortableObjects []*unstructured.Unstructured

// keep it to always check if sortableObjects implement correctly the sort interface
var _ sort.Interface = SortableObjects{}

type SortableMetadatas []ObjectMetadata

// Len implement sort.Interface
func (objs SortableObjects) Len() int { return len(objs) }

// Swap implement sort.Interface
func (objs SortableObjects) Swap(i, j int) { objs[i], objs[j] = objs[j], objs[i] }

// Less implement sort.Interface
func (objs SortableObjects) Less(i, j int) bool {
	firstMetadata := ObjectMetadataFromUnstructured(objs[i])
	secondMetadata := ObjectMetadataFromUnstructured(objs[j])
	return less(firstMetadata, secondMetadata)
}

var _ sort.Interface = SortableMetadatas{}

// Len implement sort.Interface
func (objs SortableMetadatas) Len() int { return len(objs) }

// Swap implement sort.Interface
func (objs SortableMetadatas) Swap(i, j int) { objs[i], objs[j] = objs[j], objs[i] }

// Less implement sort.Interface
func (objs SortableMetadatas) Less(i, j int) bool {
	return less(objs[i], objs[j])
}

func less(i, j ObjectMetadata) bool {
	firstGK := schema.GroupKind{Kind: i.Kind, Group: i.Group}
	secondGK := schema.GroupKind{Kind: j.Kind, Group: j.Group}
	if firstGK != secondGK {
		return lessGK(firstGK, secondGK)
	}

	// if objects has same GroupKind order by namespace and name
	firstNamespace := i.Namespace
	secondNamespace := j.Namespace
	if firstNamespace != secondNamespace {
		return firstNamespace < secondNamespace
	}

	return i.Name < j.Name
}

// lessGK implement the sorting between two GroupKinds it will use a fixed order for well known basic types of
// Kubernetes resources, and than will order the unknown ones usign the alphabetical order
func lessGK(i, j schema.GroupKind) bool {
	indexI := defaultGroupKindSortMap[i]
	indexJ := defaultGroupKindSortMap[j]
	if indexI != indexJ {
		return indexI < indexJ
	}
	if i.Group != j.Group {
		return i.Group < j.Group
	}
	return i.Kind < j.Kind
}

var defaultGroupKindSortMap = func() map[schema.GroupKind]int {
	orderList := []schema.GroupKind{
		// namespace and quota before anything
		{Group: "", Kind: "Namespace"},
		{Group: "", Kind: "ResourceQuota"},
		{Group: "", Kind: "LimitRange"},

		// configure custom resources
		{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition"},

		// configure cluster classes
		{Group: "storage.k8s.io", Kind: "StorageClass"},
		{Group: "networking.k8s.io", Kind: "IngressClass"},
		{Group: "scheduling.k8s.io", Kind: "PriorityClass"},
		{Group: "gateway.networking.k8s.io", Kind: "GatewayClass"},

		// configure authorization/authentication resources
		{Group: "", Kind: "ServiceAccount"},
		{Group: "rbac.authorization.k8s.io", Kind: "Role"},
		{Group: "rbac.authorization.k8s.io", Kind: "ClusterRole"},
		{Group: "rbac.authorization.k8s.io", Kind: "RoleBinding"},
		{Group: "rbac.authorization.k8s.io", Kind: "ClusterRoleBinding"},

		// configure workload storage
		{Group: "", Kind: "Secret"},
		{Group: "", Kind: "ConfigMap"},
		{Group: "", Kind: "PersistentVolume"},
		{Group: "", Kind: "PersistentVolumeClaim"},

		// configure workloads network policies
		{Group: "networking.k8s.io", Kind: "NetworkPolicy"},

		// configure workloads
		{Group: "", Kind: "ReplicationController"},
		{Group: "apps", Kind: "StatefulSet"},
		{Group: "apps", Kind: "Deployment"},
		{Group: "apps", Kind: "Daemonset"},
		{Group: "batch", Kind: "CronJob"},
		{Group: "batch", Kind: "Job"},
		{Group: "", Kind: "Pod"},

		// configure workloads service
		{Group: "", Kind: "Service"},
		{Group: "", Kind: "Endpoints"},
		{Group: "discovery.k8s.io", Kind: "EndpointSlice"},

		// configure ingress networking
		{Group: "networking.k8s.io", Kind: "Ingress"},
		{Group: "gateway.networking.k8s.io", Kind: "Gateway"},
		{Group: "gateway.networking.k8s.io", Kind: "HTTPRoute"},
		{Group: "gateway.networking.k8s.io", Kind: "ReferenceGrant"},

		// configure pod behaviour
		{Group: "policy", Kind: "PodDisruptionBudget"},
		{Group: "autoscaling", Kind: "HorizontalPodAutoscaler"},
		{Group: "autoscaling.k8s.io", Kind: "VerticalPodAutoscaler"},

		// configure webhooks and APIServices as last ones because they will likely depends on workloads to be ready
		{Group: "apiregistration.k8s.io", Kind: "APIService"},
		{Group: "admissionregistration.k8s.io", Kind: "MutatingWebhookConfiguration"},
		{Group: "admissionregistration.k8s.io", Kind: "ValidatingWebhookConfiguration"},
	}
	orderListLength := len(orderList)
	defaultGroupKindSortMap := make(map[schema.GroupKind]int, orderListLength)

	for i, n := range orderList {
		defaultGroupKindSortMap[n] = -orderListLength + i
	}

	return defaultGroupKindSortMap
}()
