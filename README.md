# jpl

[![Common Changelog](https://common-changelog.org/badge.svg)](https://common-changelog.org)

`jpl` is a library for managing the connection and application of Kubernetes manifest to a cluster.

The library will also provide functions for adding additional behaviours during the apply.

`jpl` is the acronym for Jet Propulsion Laboratory, the NASA branch that is responsible for the construction and
operation of planetary robotic spacecraft.

## Testing

`jpl` provides various make command to handle various tasks that you may need during development, but you need at
least these dependencies installed on your machine:

- make
- bash
- golang, for the exact version see the [.go-version](/.go-version) file in the repository

Once you have all the correct dependencies installed and the code pulled you can run the library tests with:

```bash
make test
```

Other than unit tests we also have integrations tests that  will run against envtest using [setup-envtest]
with a default Kubernetes version that can be changed using the variable `ENVTEST_K8S_VERSION`
with the `make test-integration` command:

```bash
make test-integration ENVTEST_K8S_VERSION=1.24
```

We provide a devcontainer configuration that will setup the correct dependencies and predownload the tools used
for linting. Also if you use VSCode it will setup three extensions that we recommend.

## Linting

For linting your files make provide the following command:

```bash
make lint
```

This command will run `go mod tidy` for cleaning up the `go.mod` and `go.sum` files.  
Additionally the command will download and use the [`golangci-lint`][golangci-lint] cli for running various linters
on the code, the configuration used can be seen [here](/tools/.golangci.yml).

[setup-envtest]: https://github.com/kubernetes-sigs/controller-runtime/tree/HEAD/tools/setup-envtest
  (Tool that manages binaries for envtest)
[golangci-lint]: https://golangci-lint.run (Fast linters Runner for Go)
