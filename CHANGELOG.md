# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- explicit dependencies declaration via object annotation for object in the current resource set

### Changed

- update k8s.io packages to v0.28.12

## [v0.3.0] - 2024-07-12

### Changed

- update k8s.io packages to v0.28.11
- introduced a caching mechanism to avoid multiple calls to the remote api-server
- generator now accept also a remote getter for using data from the remote api-server if needed, for feature
	parity with the new mutator interface

### Added

- mutator package for mutating objects before sending them to the remote api-server
- filter package for filtering objects before sending them to the remote api-server

### Fixed

- without a timeout set in the case of a failed apply the wait task will keep wating for an event that will never come

## [v0.2.0] - 2024-06-07

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

[Unreleased]: https://github.com/mia-platform/jpl/compare/v0.3.0...HEAD
[v0.3.0]: https://github.com/mia-platform/jpl/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/mia-platform/jpl/compare/v0.1.2...v0.2.0
[v0.1.2]: https://github.com/mia-platform/jpl/compare/v0.1.1...v0.1.2
[v0.1.1]: https://github.com/mia-platform/jpl/compare/v0.1.0...v0.1.1
[v0.1.0]: https://github.com/mia-platform/jpl/releases/tag/v0.1.0
