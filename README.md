# gh-bbc-exporter

A GitHub `gh` [CLI](https://cli.github.com/) extension for exporting BitBucket Cloud repositories into a format compatible with GitHub Enterprise migrations.

## Overview

This extension helps you migrate repositories from BitBucket Cloud to GitHub Enterprise Cloud by creating an export archive that matches the format expected by GHE migration tools.

The exporter creates a complete migration archive containing:

- Repository metadata
- Git objects (commits, branches, tags)
- Pull requests with comments
- Pull request reviews
- User information

## Installation

1. Install the `gh` CLI - see the [installation](https://github.com/cli/cli#installation) instructions.
2. Install the extension:
    ```sh
    gh extension install katiem0/gh-bbc-exporter
    ```

For more information: [`gh extension install`](https://cli.github.com/manual/gh_extension_install).


## Prerequisites

- [GitHub CLI](https://cli.github.com/) installed and authenticated
- BitBucket Cloud username and [app password](https://support.atlassian.com/bitbucket-cloud/docs/app-passwords/) with repository read access
- Go 1.19 or higher (if building from source)

## Usage

The `gh-bbc-exporter` extension only supports the retrievable of repositories from BitBucket Cloud:

```sh
gh bbc-exporter -h
Export repository and metadata from BitBucket Cloud for GitHub Cloud import.

Usage:
  bbc-exporter [flags]

Flags:
  -p, --app-password string   BitBucket app password for basic authentication
  -a, --bbc-api-url string    BitBucket API to use (default "https://api.bitbucket.org/2.0")
  -d, --debug                 Enable debug logging
  -h, --help                  help for bbc-exporter
  -o, --output string         Output directory for exported data (default: ./bitbucket-export-TIMESTAMP)
  -r, --repo string           Name of the repository to export from BitBucket Cloud
  -t, --token string          BitBucket access token for authentication
  -u, --user string           BitBucket username for basic authentication
  -w, --workspace string      BitBucket workspace (or username for personal accounts)
```


For migrations from BitBucket Data Center or Server, please see [GitHub's Official Documentation](https://docs.github.com/en/migrations/using-github-enterprise-importer/migrating-from-bitbucket-server-to-github-enterprise-cloud/about-migrations-from-bitbucket-server-to-github-enterprise-cloud).

### Export Format 

The exporter creates a directory or archive with the following structure:

```
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

After generating the migration archive, you can import it to GitHub Enterprise Cloud using GitHub owned storage and GEI.

## Limitations

- Wiki content is not included in the export
- Issues are not exported (BitBucket issues have a different structure from GitHub issues)
- Labels in repositories are set to an empty array for compatibility
- User information is limited to what's available from BitBucket API

## Troubleshooting

### Common Issues

1. **Authentication Errors**
   Make sure your BitBucket app password has the necessary permissions to access repositories.
2. **Export Fails with Network Errors**
   BitBucket API may have rate limits. Try running the export with the `--debug` flag to see detailed error messages.
3. **Empty Repository Export**
   If the repository can't be cloned, the exporter creates an empty repository structure. Check that the repository exists and is accessible.


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
