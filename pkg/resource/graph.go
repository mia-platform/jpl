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
	"maps"
	"sort"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
)

type DependencyGraph struct {
	edges map[*unstructured.Unstructured]sets.Set[*unstructured.Unstructured]
}

func (g *DependencyGraph) addVertex(obj *unstructured.Unstructured) {
	if _, found := g.edges[obj]; !found {
		g.edges[obj] = sets.New[*unstructured.Unstructured]()
	}
}

func (g *DependencyGraph) addEdge(from, to *unstructured.Unstructured) {
	g.addVertex(from)
	g.addVertex(to)

	edges := g.edges[from]
	edges.Insert(to)
}

func (g *DependencyGraph) SortedResourceGroups() [][]*unstructured.Unstructured {
	edges := maps.Clone(g.edges)

	groups := make([][]*unstructured.Unstructured, 0)
	for len(edges) > 0 {
		group := make([]*unstructured.Unstructured, 0)
		for vertex, vertexEdges := range edges {
			if len(vertexEdges) == 0 {
				group = append(group, vertex)
			}
		}

		if len(group) == 0 {
			return groups
		}

		for _, resource := range group {
			delete(edges, resource)
			for vertex, vertexEdges := range edges {
				edges[vertex] = vertexEdges.Delete(resource)
			}
		}

		sort.Sort(SortableObjects(group))
		groups = append(groups, group)
	}

	return groups
}

func NewDependencyGraph(objs []*unstructured.Unstructured) *DependencyGraph {
	graph := &DependencyGraph{
		edges: make(map[*unstructured.Unstructured]sets.Set[*unstructured.Unstructured]),
	}

	if len(objs) == 0 {
		return graph
	}

	crds := make(map[schema.GroupKind]*unstructured.Unstructured)
	namespaces := make(map[string]*unstructured.Unstructured)
	webhooks := make(map[ObjectMetadata][]*unstructured.Unstructured)

	for _, obj := range objs {
		graph.addVertex(obj)
		switch {
		case IsCRD(obj):
			var typedCRD apiextv1.CustomResourceDefinition
			if err := runtime.DefaultUnstructuredConverter.FromUnstructuredWithValidation(obj.Object, &typedCRD, true); err != nil {
				continue
			}

			crds[schema.GroupKind{Group: typedCRD.Spec.Group, Kind: typedCRD.Spec.Names.Kind}] = obj
		case IsNamespace(obj):
			namespaces[obj.GetName()] = obj
		case IsRegistrationWebhook(obj):
			for _, svc := range servicesMetadataFromWebhook(obj) {
				webhooks[svc] = append(webhooks[svc], obj)
			}
		}
	}

	for _, obj := range objs {
		objMeta := ObjectMetadataFromUnstructured(obj)
		if crd, found := crds[schema.GroupKind{Group: objMeta.Group, Kind: objMeta.Kind}]; found {
			graph.addEdge(obj, crd)
		}

		if ns, found := namespaces[objMeta.Namespace]; found {
			graph.addEdge(obj, ns)
		}

		if webhook, found := webhooks[objMeta]; found {
			for _, webhook := range webhook {
				graph.addEdge(webhook, obj)
			}
		}
	}

	return graph
}
