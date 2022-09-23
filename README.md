# jpl

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

The tests will run against envtest using [setup-envtest] with a default Kubernetes version that can be
changed using the variable `ENVTEST_K8S_VERSION` before launching `make test`

We provide a devcontainer configuration that will setup the correct dependencies and predownload the tools used
for linting. Also if you use VSCode it will setup three extensions that we recommend.

## Linting

For linting your files make provide the following command:

```bash
make lint
```

This command will run `go vet` and `go mod tidy` for cleaning up the `go.mod` and `go.sum` files and stop if it senses
that the files are changed and where not already commited or added to the git staging area, this check is done forcing
the user to not forgetting this steps and for breaking the ci/cd on GitHub if those files are not
in the correct shape.  
Additionally the command will download and use the [`golangci-lint`][golangci-lint] cli for running various linters
on the code, the configuration used can be seen [here](/tools/.golangci.yml).

[setup-envtest]: https://github.com/kubernetes-sigs/controller-runtime/tree/HEAD/tools/setup-envtest
  (Tool that manages binaries for envtest)
[golangci-lint]: https://golangci-lint.run (Fast linters Runner for Go)
