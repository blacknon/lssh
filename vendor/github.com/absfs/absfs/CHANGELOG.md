# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed
- **CRITICAL**: Fixed `OS_ALL_RWX` constant which incorrectly only granted execute permission to group instead of all users (filemode.go:109)
- Improved `RemoveAll` error handling and code clarity
- Simplified defer logic in `removeAll` function for better maintainability

### Added
- Comprehensive test suite with 89% code coverage
- Tests for all permission constants to prevent regression
- Tests for invalidfile.go (100% coverage)
- Tests for fileadapter.go edge cases
- Tests for filesystem.go with both optional and required interfaces
- CI/CD configuration with GitHub Actions
- Support for multiple Go versions (1.20, 1.21, 1.22)
- Cross-platform testing (Linux, macOS, Windows)
- Code coverage reporting with Codecov
- Security policy (SECURITY.md)
- Architecture documentation (ARCHITECTURE.md)
- Benchmarks for core operations
- Godoc examples for common use cases

### Changed
- `removeAll` implementation now has clearer error handling and flow
- Test coverage increased from 22.7% to 89.1%

## [Previous Releases]

See git history for changes prior to this changelog.
