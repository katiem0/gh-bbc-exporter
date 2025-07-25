# gh-bbc-exporter

[![GitHub Release](https://img.shields.io/github/v/release/katiem0/gh-bbc-exporter?style=flat&logo=github)](https://github.com/katiem0/gh-bbc-exporter/releases)
[![PR Checks](https://github.com/katiem0/gh-bbc-exporter/actions/workflows/main.yml/badge.svg)](https://github.com/katiem0/gh-bbc-exporter/actions/workflows/main.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Go Report Card](https://goreportcard.com/badge/github.com/katiem0/gh-bbc-exporter)](https://goreportcard.com/report/github.com/katiem0/gh-bbc-exporter)
[![Go Version](https://img.shields.io/github/go-mod/go-version/katiem0/gh-bbc-exporter)](https://go.dev/)

A GitHub `gh` [CLI](https://cli.github.com/) extension for exporting Bitbucket Cloud
repositories into a format compatible with GitHub Enterprise migrations.

## Overview

This extension helps you migrate repositories from Bitbucket Cloud to GitHub Enterprise Cloud
by creating an export archive that matches the format expected by GitHub Enterprise Importer (GEI).

The exporter creates a complete migration archive containing:

- Repository metadata
- Git objects (commits, branches, tags)
- Pull requests with comments
- Pull request reviews
- User information

## Installation

```sh
gh extension install katiem0/gh-bbc-exporter
```

For more information: [`gh extension install`](https://cli.github.com/manual/gh_extension_install).

## Prerequisites

- [GitHub CLI](https://cli.github.com/) installed and authenticated
- Bitbucket Cloud workspace administration access
- Go 1.19 or higher (if building from source)

### Bitbucket Authentication Options

Bitbucket Cloud provides two authentication methods for their API:

- Basic Authentication
- Access Token (premium membership)

#### Basic Authentication

For basic authentication with this tool your account username and an app password
are needed. Your Bitbucket username can be found by following:

1. On the sidebar, click on the Profile picture
2. Select View profile
3. Click on "Settings"
4. Find Username under **Bitbucket profile settings**

To [create an app password][app-password]:

1. Under Personal settings, select Personal Bitbucket settings.
2. On the left sidebar, select App passwords.
3. Select Create app password.
4. Give the App password a name.
5. Select the following permissions:
   - `Account: Read`
   - `Workspace Membership: Read`
   - `Repositories: Read`
   - `Pull Requests: Read`
6. Select the Create button. The page will display the New app password dialog.

#### Workspace Access Token

A workspace-level access token is required to ensure a list of users is retrieved
to be able to associate metadata with their GitHub account.

The access token will require the following permissions:

- `Account: Read`
- `Repositories: Read`
- `Pull Requests: Read`

## Usage

The `gh-bbc-exporter` extension only supports the retrieval of repositories from Bitbucket Cloud:

```sh
gh bbc-exporter -h
Export repository and metadata from Bitbucket Cloud for GitHub Cloud import.

Usage:
  bbc-exporter [flags]

Flags:
  -p, --app-password string    Bitbucket app password for basic authentication
  -a, --bbc-api-url string     Bitbucket API to use (default "https://api.bitbucket.org/2.0")
  -d, --debug                  Enable debug logging
  -h, --help                   help for bbc-exporter
      --open-prs-only          Export only open pull requests and ignore closed/merged ones
  -o, --output string          Output directory for exported data (default: ./bitbucket-export-TIMESTAMP)
      --prs-from-date string   Export pull requests created on or after this date (format: YYYY-MM-DD)
  -r, --repo string            Name of the repository to export from Bitbucket Cloud
  -t, --token string           Bitbucket access token for authentication
  -u, --user string            Bitbucket username for basic authentication
  -w, --workspace string       Bitbucket workspace name
```

Example Command

```sh
gh bbc-exporter -w your-workspace -r your-repo -t your-bitbucket-token
```

Or with basic authentication:

```sh
gh bbc-exporter -w your-workspace -r your-repo -u your-username -p your-app-password
```

For migrations from BitBucket Data Center or Server, please see [GitHub's Official Documentation][bitbucket-server].

### Export Format

The exporter creates a directory or archive with the following structure:

```text
bitbucket-export-YYYYMMDD-HHMMSS/
├── schema.json
├── repositories_000001.json
├── users_000001.json
├── organizations_000001.json
├── pull_requests_000001.json
├── issue_comments_000001.json
├── pull_request_review_comments_000001.json
├── pull_request_review_threads_000001.json
├── pull_request_reviews_000001.json
└── repositories/
    └── <workspace>/
        └── <repository>.git/
            ├── objects/
            ├── refs/
            └── info/
                ├── nwo
                └── last-sync
```

## Importing to GitHub Enterprise CLoud

After generating the migration archive, you can import it to GitHub Enterprise Cloud
using GitHub owned storage and GEI. Detailed documentation can be found in
[Importing Bitbucket Cloud Archive to GitHub Enterprise Cloud](./docs/GHImport.md).

## Limitations

- Wiki content is not included in the export
- Issues are not exported (Bitbucket issues have a different structure from GitHub issues)
- Repository and Pull request labels have not been implemented
- User information is limited to what's available from Bitbucket API
- [Archives larger than 40 GiB][storage-increase] are not supported by GitHub-owned storage
- GitHub Enterprise Cloud with data residency is not supported

## Troubleshooting

### Common Issues

1. **Authentication Errors**
   Make sure your Bitbucket app password has the necessary permissions to access repositories.
2. **Export Fails with Network Errors**
   Bitbucket API may have rate limits. Try running the export with the `--debug` flag to see
   detailed error messages.
3. **Empty Repository Export**
   If the repository can't be cloned, the exporter creates an empty repository structure.
   Check that the repository exists and is accessible.
4. **Migration Fails in GitHub Enterprise Importer**
   Check the error logging repository that's created during migration for detailed
   information about any failures.

## Development

### Building from Source

1. Clone the repository:

   ```sh
   git clone https://github.com/katiem0/gh-bbc-exporter.git
   cd gh-bbc-exporter
   ```

2. Build the extension:

   ```sh
   go build -o gh-bbc-exporter
   ```

3. Install locally for testing:

   ```sh
   gh extension install .
   ```

### Running Tests

```sh
go test ./...
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

<!-- link reference section -->

[app-password]: https://support.atlassian.com/bitbucket-cloud/docs/create-an-app-password/
[storage-increase]: https://github.blog/changelog/2025-06-03-increasing-github-enterprise-importers-repository-size-limits/
[bitbucket-server]: https://docs.github.com/en/migrations/using-github-enterprise-importer/migrating-from-bitbucket-server-to-github-enterprise-cloud/about-migrations-from-bitbucket-server-to-github-enterprise-cloud
