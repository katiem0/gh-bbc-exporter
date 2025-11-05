# Importing Bitbucket Cloud Archive to GitHub Enterprise Cloud

This guide covers importing Bitbucket Cloud repositories to GitHub Enterprise
Cloud using GitHub Enterprise Importer (GEI).

## Migration Methods

There are three primary methods to perform the migration:

1. **[Method 1: Automated with GitHub Actions (Recommended)](#method-1-automated-with-github-actions)**
2. **[Method 2: Using the GitHub CLI Manually](#method-2-using-the-github-cli-manually)**
3. **[Method 3: Using API Calls Directly](#method-3-using-api-calls-directly)**

---

### Method 1: Automated with GitHub Actions

This approach automates the entire process of exporting from Bitbucket and importing into GitHub.

A sample workflow is available at [`sample-migration-via-actions.yml`](sample-migration-via-actions.yml).

**Setup:**

1. Add the workflow file to your repository (e.g., `.github/workflows/migrate-repo.yml`).
2. Configure the following secrets (or authentication method secrets of your choice) in
   your GitHub repository's settings:
   - `GH_PAT`: A GitHub Personal Access Token with `repo` scope.
   - `BITBUCKET_API_TOKEN`: A Bitbucket API token with `repository:read` and `pullrequest:read` permissions.
   - `BITBUCKET_EMAIL`: The Atlassian account email associated with the API token.

**Execution:**

Trigger the workflow manually via the "Actions" tab in your GitHub repository, providing
the required inputs for the Bitbucket workspace, repository, and target GitHub organization.

---

### Method 2: Using the GitHub CLI Manually

This method is suitable if you prefer to run the steps on your local machine.

#### Storage Options Comparison

| Storage Type | Archive Location | Manual Upload | Path Type |
|:-------------|:-----------------|:--------------|:----------|
| **GitHub-owned** | Local filesystem | No - automatic | Local file path |
| **Azure Blob** | Azure container | Yes - before migration | Blob path |
| **AWS S3** | S3 bucket | Yes - before migration | S3 object key |

#### Important Limitations

- [GEI][gei-limitations] does not support archives larger than 30 GiB.
- Not available for GitHub Enterprise Cloud with data residency.
- Azure/AWS require you to generate short-lived, publicly accessible URLs for the archives.

#### Option A: GitHub-owned storage (Recommended)

> [!Important]
> The archive is automatically uploaded during migration. No pre-upload needed.

```sh
gh gei migrate-repo \
  --github-source-org SOURCE_ORG \
  --source-repo SOURCE_REPO \
  --github-target-org TARGET_ORG \
  --target-repo TARGET_REPO \
  --git-archive-path PATH_TO_LOCAL_ARCHIVE.tar.gz \
  --metadata-archive-path PATH_TO_LOCAL_ARCHIVE.tar.gz \
  --use-github-storage
```

#### Option B: Azure Storage Blobs

> [!Important]
> **Prerequisites:**
>
> 1. Upload your `bitbucket-export-*.tar.gz` to Azure Blob Storage.
> 2. Configure your [connection string][azure-string].
> 3. Note the blob path for the archive.

```sh
gh gei migrate-repo \
  --github-source-org SOURCE_ORG \
  --source-repo SOURCE_REPO \
  --github-target-org TARGET_ORG \
  --target-repo TARGET_REPO \
  --azure-storage-connection-string "YOUR_CONNECTION_STRING" \
  --git-archive-path AZURE_BLOB_PATH.tar.gz \
  --metadata-archive-path AZURE_BLOB_PATH.tar.gz
```

#### Option C: AWS S3 Buckets

> [!Important]
> **Prerequisites:**
>
> 1. Upload your `bitbucket-export-*.tar.gz` to an S3 bucket.
> 2. Configure [AWS credentials][aws-credentials].
> 3. Note the S3 object key for the archive.

```sh
gh gei migrate-repo \
  --github-source-org SOURCE_ORG \
  --source-repo SOURCE_REPO \
  --github-target-org TARGET_ORG \
  --target-repo TARGET_REPO \
  --aws-bucket-name YOUR_BUCKET_NAME \
  --git-archive-path S3_OBJECT_KEY.tar.gz \
  --metadata-archive-path S3_OBJECT_KEY.tar.gz
```

---

### Method 3: Using API Calls Directly

For advanced users who need granular control over the migration process.

#### Prerequisites

- [Required access and prerequisites][recs-and-prereqs]
- Personal access token with [appropriate scopes][PAT-scopes]
- [Migrator or organization owner role][migrate-roles]

#### Step 1: Get Organization ID

Use the following GraphQL query to get the `id` and `databaseId` for your target organization.

```graphql
query GetOrgInfo($login: String!) {
  organization(login: $login) {
    login
    id
    databaseId
  }
}
```

#### Step 2: Create Migration Source

Create a migration source to represent Bitbucket Cloud.

```graphql
mutation createMigrationSource($name: String!, $url: String!, $ownerId: ID!) {
  createMigrationSource(input: {
    name: $name
    url: $url
    ownerId: $ownerId
    type: GITHUB_ARCHIVE
  }) {
    migrationSource {
      id
      name
      type
    }
  }
}
```

**Variables:**

```json
{
  "name": "Bitbucket Cloud Source",
  "url": "https://bitbucket.org",
  "ownerId": "YOUR_ORG_ID_FROM_STEP_1"
}
```

#### Step 3: Upload Archive to GitHub-owned Storage

##### For archives under 5 GiB (Single POST)

```sh
curl --request POST \
  --header "Authorization: Bearer $GITHUB_TOKEN" \
  --header "Content-Type: application/octet-stream" \
  --data-binary '@bitbucket-export.tar.gz' \
  'https://uploads.github.com/organizations/{org_database_id}/gei/archive?name=bitbucket-export.tar.gz'
```

The response will contain a `uri` (e.g., `gei://archive/...`) to use in the next step.

##### For archives 5-30 GiB (Multipart Upload)

This involves starting an upload, sending file parts, and completing the upload. For
details, refer to the [Ruby script example][private-storage].

#### Step 4: Start Repository Migration

Use the `startRepositoryMigration` mutation with the GEI URI from the previous step.

```graphql
mutation startRepositoryMigration(...) {
  startRepositoryMigration(input: {
    sourceId: "...",
    ownerId: "...",
    repositoryName: "...",
    continueOnError: true,
    githubPat: "...",
    accessToken: "...",
    gitArchiveUrl: "GEI_URI_FROM_STEP_3",
    metadataArchiveUrl: "GEI_URI_FROM_STEP_3",
    sourceRepositoryUrl: "https://bitbucket.org/{workspace}/{repo}",
    targetRepoVisibility: "private"
  }) {
    repositoryMigration {
      id
    }
  }
}
```

#### Step 5: Check Migration Status

Use the `getMigration` query with the `id` from the previous step to monitor the status.

```graphql
query getMigration($id: ID!) {
  node(id: $id) {
    ... on Migration {
      id
      state
      failureReason
    }
  }
}
```

---

## Post-Migration Steps

After a successful migration:

1. **[Review migration logs][migration-logs]** for any warnings or issues.
2. **[Reclaim mannequins][reclaim-mannequin]** to map migrated users to their GitHub accounts.
3. **Configure workflows** and CI/CD pipelines.
4. **Verify repository contents** and settings.

---

## Additional Resources

- [AWS credentials configuration][aws-credentials]
- [Azure connection string setup][azure-string]
- [GitHub-owned storage documentation][private-storage]
- [Migration prerequisites][recs-and-prereqs]
- [Access management for migrations][migrate-roles]
- [Required PAT scopes][PAT-scopes]
- [Migration state reference][migration-state]

<!-- Links -->
[aws-credentials]: https://docs.github.com/en/migrations/using-github-enterprise-importer/migrating-between-github-products/migrating-repositories-from-github-enterprise-server-to-github-enterprise-cloud?tool=cli#configuring-aws-s3-credentials-in-the-github-cli
[azure-string]: https://docs.github.com/en/migrations/using-github-enterprise-importer/migrating-between-github-products/migrating-repositories-from-github-enterprise-server-to-github-enterprise-cloud?tool=cli#configuring-azure-blob-storage-account-credentials-in-the-github-cli
[private-storage]: https://github.com/orgs/community/discussions/144948
[recs-and-prereqs]: https://docs.github.com/en/migrations/using-github-enterprise-importer/migrating-between-github-products/migrating-repositories-from-github-enterprise-server-to-github-enterprise-cloud#prerequisites
[migrate-roles]: https://docs.github.com/en/migrations/using-github-enterprise-importer/migrating-from-bitbucket-server-to-github-enterprise-cloud/managing-access-for-a-migration-from-bitbucket-server
[PAT-scopes]: https://docs.github.com/en/migrations/using-github-enterprise-importer/migrating-from-bitbucket-server-to-github-enterprise-cloud/managing-access-for-a-migration-from-bitbucket-server#required-scopes-for-personal-access-tokens
[migration-state]: https://docs.github.com/en/enterprise-cloud@latest/graphql/reference/enums#migrationstate
[migration-logs]: https://docs.github.com/en/migrations/using-github-enterprise-importer/completing-your-migration-with-github-enterprise-importer/accessing-your-migration-logs-for-github-enterprise-importer
[reclaim-mannequin]: https://docs.github.com/en/migrations/using-github-enterprise-importer/completing-your-migration-with-github-enterprise-importer/reclaiming-mannequins-for-github-enterprise-importer
[gei-limitations]: https://docs.github.com/en/migrations/using-github-enterprise-importer/migrating-between-github-products/about-migrations-between-github-products#limitations-on-migrated-data
