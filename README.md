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
- [Compatibility: jpl <-> Kubernetes clusters](#compatibility-jpl---kubernetes-clusters)
  - [Compatibility matrix](#compatibility-matrix)
- [How to get it](#how-to-get-it)

### What's included

- the `client` package contains the client to apply the resources to a Kubernetes API server
- the `event` package contains the various events that the `client` will return to tell the user what is happening
- the `flowcontrol` package contains the checks necessary to know if the Kubernetes API server has the flowcontrol enabled
- the `generator` package contain built-in generators that can be used to generate new resources from other manifests
- the `invetnory` package is used to keep track of the resources deployed in precedent apply to compute the
	necessary pruning actions
- the `resource` package contains useful utils function to work with Unstructured data
- the `resourcereader` package is useful for parsing valid kubernetes resource manifests from a folder of yaml file
	or via stdin
- the `runner` package contains a queue like executor of a series of tasks sequentially
- the `testing` package contains utils for testing the other packages
- the `util` package contains utility resources

### Compatibility: jpl <-> Kubernetes clusters

Since `jpl` will use the Kuberntes packages to execute calls, every version of the library is compatible with
the versions of Kubernetes that are compatible with them.

#### Compatibility matrix

|             | Kubernetes 1.24 | Kubernetes 1.25 | Kubernetes 1.26 | Kubernetes 1.27 | Kubernetes 1.28 | Kubernetes 1.29 | Kubernetes 1.30 |
| ------------| --------------- | --------------- | --------------- | --------------- | --------------- | --------------- | --------------- |
| `jpl-0.1.x` | +-              | ✓               | +-              | +-              | +-              | +-              | +-              |
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
go get github.com/mia-platform/jpl@v0.20.4
```

[go-report-card]: https://goreportcard.com/badge/github.com/mia-platform/jpl
[go-report-card-link]: https://goreportcard.com/report/github.com/mia-platform/jpl
[go-package-link]: https://pkg.go.dev/github.com/mia-platform/jpl
[go-package-svg]: https://pkg.go.dev/badge/github.com/mia-platform/jpl.svg
