# Contributing

Thank you for your interest in contributing to the `gh-bbc-exporter` project! This document provides guidelines and instructions for contributing.

## How to Contribute

### Reporting Bugs

1. Ensure the bug hasn't already been reported by searching GitHub Issues
2. If you can't find an existing issue, open a new one using the bug report template
3. Include detailed steps to reproduce the bug and any relevant environment details

### Feature Requests

1. Check existing issues to see if the feature has already been requested
2. If not, open a new feature request issue using the feature request template
3. Describe the feature clearly and explain why it would be valuable

### Submitting Changes

1. Fork the repository
2. Create a new branch: `git checkout -b feature/your-feature-name`
3. Make your changes
4. Run the tests: `go test ./...`
5. Commit your changes following conventional commits pattern
6. Push to your fork: `git push origin feature/your-feature-name`
7. Submit a Pull Request against the main branch

### Pull Request Process

1. Update the README.md or documentation with details of changes if applicable
2. Update any examples or version numbers in files if applicable
3. The PR should work with all supported Go versions
4. PR title should follow conventional commits pattern (e.g., `feat: add new feature`)
5. A maintainer will review your PR and provide feedback

## Development Setup

1. Clone the repository: `git clone https://github.com/katiem0/gh-bbc-exporter.git`
2. Navigate to the project directory: `cd gh-bbc-exporter`
3. Install dependencies: `go mod download`
4. Build the project: `go build`
5. Run tests: `go test ./...`

## Coding Standards

- Follow Go standard formatting and idioms
- Use `go fmt` to format your code
- Use `golint` and `go vet` to check your code quality
- Write meaningful commit messages following [Conventional Commits](https://www.conventionalcommits.org/)

Thank you for contributing!