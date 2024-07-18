# jpl

[![Go Report Card][go-report-card]][go-report-card-link]
[![Go Package Reference Site][go-package-svg]][go-package-link]

`JPL` is a library to simplify the application of a series of kuberntes resources saved on files to a remote cluster.  
It also add some nice to have features that lacks in kubectl:

- resources ordering
- automatic waiting of current statuses
- generating new resources from manifests
- check if flowcontrol api is enabled on the remote cluster
- keep inventory of applied resoruces for automatic pruning

The fastest way to add this library to a project is to run `go get github.com/mia-platform/jpl@latest` with go1.16+

## Table of Contents

- [What's included](#whats-included)
- [Features](#features)
- [Compatibility: jpl <-> Kubernetes clusters](#compatibility-jpl---kubernetes-clusters)
  - [Compatibility matrix](#compatibility-matrix)
- [How to get it](#how-to-get-it)

### What's included

- the `client` package contains the client to apply the resources to a Kubernetes API server
- the `event` package contains the various events that the `client` will return to tell the user what is happening
- the `filter` package contain a filter interface for omit resources from the current apply action
- the `flowcontrol` package contains the checks necessary to know if the Kubernetes API server has the flowcontrol enabled
- the `generator` package contain built-in generators that can be used to generate new resources from other manifests
- the `inventory` package is used to keep track of the resources deployed in precedent apply to compute the
	necessary pruning actions
- the `mutator` package contain built-in mutators that can be used to modify resources before applying them
- the `resource` package contains useful utils function to work with Unstructured data
- the `resourcereader` package is useful for parsing valid kubernetes resource manifests from a folder of yaml file
	or via stdin
- the `runner` package contains a queue like executor of a series of tasks sequentially
- the `testing` package contains utils for testing the other packages
- the `util` package contains utility resources

### Features

#### Pruning

The Applier automatically deletes objects that were previously applied and then removed from the input set on
a subsequent apply.

The current implementation of `kubectl apply --prune` is an alpha, and it is improbable that it will graduate to beta.
`jpl` attempts to address the current deficiencies by storing the set of previously applied objects in an **inventory**
object which is applied to the cluster. The reference implementation uses a `ConfigMap` as an **inventory** object
and references to the applied objects are stored in the `data` section of the `ConfigMap` that is generated and
recovered at every run.

#### Waiting for Reconciliation

The Applier automatically watches applied objects and tracks their status, blocking until the objects have reconciled
or failed.

This functionality is similar to `kubectl delete <resource> <name> --wait`, in that it waits for all finalizers
to complete, except it works for creates and updates.

While there is a `kubectl apply <resource> <name> --wait`, it only waits for deletes when combined with `--prune`.
`jpl` provides an alternative that works for all spec changes, waiting for reconciliation, the convergence of
status to the desired specification. After reconciliation, it is expected that the object has reached a steady state
until the specification is changed again.

#### Resource Ordering

The Applier use resource type to determine which order to apply and delete objects.

In contrast, when using `kubectl apply`, the objects are applied in alphanumeric order of their file names,
and top to bottom in each file. With `jpl`, this manual sorting is unnecessary for many common use cases.

##### Implicit Dependency Ordering

`jpl` automatically detects some implicit dependencies that includes:

1. Namespace-scoped resource objects depend on their Namespace
1. Custom resource objects depend on their Custom Resource Definition
1. Validation and Mutating Webhooks depends on their Services

Like resource ordering, implicit dependency ordering improves the apply and delete experience to reduce the need to
manually specify ordering for many common use cases. This allows more objects to be applied together all at once,
with less manual orchestration.

##### Explicit Dependency Ordering

In addition to implicit depedendencies sometimes the user would like to determine certain resources ordering.
In these cases, the user can use explicit dependency ordering by adding a
`config.kubernetes.io/depends-on: <OBJECT_REFERENCE>` annotation to an object.

The Applier use these explicit dependency directives to build a dependency tree and flatten it for determining apply
ordering.

In addition to ordering the applies, dependency ordering also waits for dependency reconciliation when applying.
This ensures that dependencies are not just applied first, but have reconciled before their dependents are applied.

In the following example, the `config.kubernetes.io/depends-on` annotation identifies that `nginx` must be successfully
applied prior to `workload` actuation:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: workload
  annotations:
    config.kubernetes.io/depends-on: /namespaces/default/Pod/nginx
spec:
  containers:
    - name: workload
      image: registry.k8s.io/pause:2.0
```

### Compatibility: jpl <-> Kubernetes clusters

Since `jpl` will use the Kuberntes packages to execute calls, every version of the library is compatible with
the versions of Kubernetes that are compatible with them.

#### Compatibility matrix

|             | Kubernetes 1.24 | Kubernetes 1.25 | Kubernetes 1.26 | Kubernetes 1.27 | Kubernetes 1.28 | Kubernetes 1.29 | Kubernetes 1.30 |
| ------------| --------------- | --------------- | --------------- | --------------- | --------------- | --------------- | --------------- |
| `jpl-0.1.x` | +-              | ✓               | +-              | +-              | +-              | +-              | +-              |
| `jpl-0.2.x` | +-              | +-              | +-              | +-              | ✓               | +-              | +-              |
| `jpl-0.3.x` | +-              | +-              | +-              | +-              | ✓               | +-              | +-              |
| `HEAD`      | +-              | +-              | +-              | +-              | ✓               | +-              | +-              |

Key:

- `✓` the Kubernetes version officially sypported by the packages versions
- `+` kubernetes packages can have features or API objects that may not be present in the Kubernetes cluster,
	either due to that client-go has additional new API, or that the server has removed old API. However,
	everything they have in common (i.e., most APIs) will work. Please note that alpha APIs may vanish
	or change significantly in a single release.
- `-` The Kubernetes cluster has features that the kubernetes packages can't use, either due to the server has
	additional new API, or that client-go has removed old API. However, everything they share in common
	(i.e., most APIs) will work.

See the [CHANGELOG](./CHANGELOG.md) for a detailed description of changes between jpl versions.

### How to get it

To get the latest version, use go1.16+ and fetch using the `go get` command. For example:

```sh
go get github.com/mia-platform/jpl@latest
```

To get a specific version, use go1.11+ and fetch the desired version using the `go get` command. For example:

```sh
go get github.com/mia-platform/jpl@v0.2.0
```

[go-report-card]: https://goreportcard.com/badge/github.com/mia-platform/jpl
[go-report-card-link]: https://goreportcard.com/report/github.com/mia-platform/jpl
[go-package-link]: https://pkg.go.dev/github.com/mia-platform/jpl
[go-package-svg]: https://pkg.go.dev/badge/github.com/mia-platform/jpl.svg
