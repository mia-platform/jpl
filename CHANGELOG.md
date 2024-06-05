# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- overhauled the library

### Added

- inventory package to keep track of what resources are deployed in the cluster
- flowcontrol package for quering remote api-server on the activation state of the flowcontrol feature
- resourcereader package for reading kubernetes manifests from path or buffer
- runner package for handling a queue of tasks to execute in order
- client package contains a brand new Applier client for apply local manifests against a remote api-server
- internal/poller package will poll the remote api-server for getting the status of a set of resources

## [v0.1.2] - 2022-10-28

### Fixed

- Fix generation of last applied config annotations during updates for resources that doesn't have it

## [v0.1.1] - 2022-10-25

### Fixed

- Fix resource check against cluster definition when choosing if is namespaced or not

## [v0.1.0] - 2022-10-19

### Added

- Lifted code from mlp to a separate module

[Unreleased]: https://github.com/mia-platform/jpl/compare/v0.1.2...HEAD
[v0.1.2]: https://github.com/mia-platform/jpl/compare/v0.1.1...v0.1.2
[v0.1.1]: https://github.com/mia-platform/jpl/compare/v0.1.0...v0.1.1
[v0.1.0]: https://github.com/mia-platform/jpl/releases/tag/v0.1.0
