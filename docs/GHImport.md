# Importing Bitbucket Cloud Archive to GitHub Enterprise Cloud

This import process relies on the use of
[GitHub Enterprise Importer with GitHub owned blob storage](https://github.blog/changelog/2024-11-21-migrate-repositories-with-github-enterprise-importer-using-github-owned-blob-storage-private-preview/),
as outlined in this [discussion post](https://github.com/orgs/community/discussions/144948).

> [!Note]
> This functionality is not supported by the main migration pathway, and is subject to change.

## Limitations

1. Migrations using GitHub-owned blob storage with GEI are gated by a feature
   flag during private preview. If you’re interested in participating in this
   private preview, please reach out to your GitHub account manager or contact
   our sales team to have this feature enabled for your enterprise.
2. This feature is not available for migration to GitHub Enterprise Cloud with
   data residency at this time.

## Using GitHub-owned storage via individual API calls

The import process follows the same process documented in
[Migrating repositories from GitHub Enterprise Server to GitHub Enterprise Cloud](https://docs.github.com/en/migrations/using-github-enterprise-importer/migrating-between-github-products/migrating-repositories-from-github-enterprise-server-to-github-enterprise-cloud?tool=api).

### Prerequisites

As noted in our public documentation, the same
[recommendations and prerequisites](https://docs.github.com/en/migrations/using-github-enterprise-importer/migrating-between-github-products/migrating-repositories-from-github-enterprise-server-to-github-enterprise-cloud?tool=api#prerequisites)
are needed for performing this migration.

#### GitHub Enterprise Cloud Access

As with other imports to GitHub, the ability to utilize
[migrator or organization owner roles](https://docs.github.com/en/migrations/using-github-enterprise-importer/migrating-from-bitbucket-server-to-github-enterprise-cloud/managing-access-for-a-migration-from-bitbucket-server#required-roles-for-github)
still exists.

Personal Access tokens will require different scopes based on the role. More
information can be found in
[Required scopes for personal access tokens](https://docs.github.com/en/migrations/using-github-enterprise-importer/migrating-from-bitbucket-server-to-github-enterprise-cloud/managing-access-for-a-migration-from-bitbucket-server#required-scopes-for-personal-access-tokens).

### Get Owner ID for Destination Organization

As an organization owner in GitHub Enterprise Cloud, use the `GetOrgInfo` query to
return the `ownerId`, also called the organization ID, for the organization you
want to own the migrated repositories. You'll need the `ownerId` to identify your
migration destination.

### `GetOrgInfo` query

```graphql
query(
  $login: String!
){
  organization (login: $login)
  {
    login
    id
    name
    databaseId
  }
}
```

| Query variable | Description|
|:---------------|:-----------|
|`login` | Your organization name. |

### `GetOrgInfo` response

```json
{
  "data": {
    "organization": {
      "login": "Octo",
      "id": "MDEyOk9yZ2FuaXphdGlvbjU2MTA=",
      "name": "Octo-org",
      "databaseId": 5610
    }
  }
}
```

In this example, `MDEyOk9yZ2FuaXphdGlvbjU2MTA` is the organization ID or `ownerId`,
which we'll use in the next step.

### Set Up Migration Source in GitHub Enterprise Cloud

You can set up a migration source using the`createMigrationSource` query. You'll need to supply
the `ownerId`, or organization ID, gathered from the `GetOrgInfo` query.

### `createMigrationSource` mutation

```graphql
mutation createMigrationSource($name: String!, $url: String!, $ownerId: ID!) {
  createMigrationSource(input: {name: $name, url: $url, ownerId: $ownerId, type: GITHUB_ARCHIVE}) {
    migrationSource {
      id
      name
      url
      type
    }
  }
}
```

>[!Note]
>Make sure to use `GITHUB_ARCHIVE` for the migration source `type`.

<!-- markdownlint-disable MD013 -->
|Query variable|Description|
|:--------------|:-----------|
| `name`| A name for your migration source. This name is for your own reference, so you can use any string. |
| `ownerId` | The organization ID of your organization on GitHub Enterprise Cloud.|
| `url` | The URL for Bitbucket Cloud: `https://bitbucket.org` should be used. |
<!-- markdownlint-enable MD013 -->

### `createMigrationSource` response

```json
{
  "data": {
    "createMigrationSource": {
      "migrationSource": {
        "id": "MS_kgDaACRjZTY5NGQ1OC1mNDkyLTQ2NjgtOGE1NS00MGUxYTdlZmQwNWQ",
        "name": "Bitbucket Cloud Source",
        "url": "https://bitbucket.org",
        "type": "GITHUB_ARCHIVE"
      }
    }
  }
}
```

In this example, `MS_kgDaACRjZTY5NGQ1OC1mNDkyLTQ2NjgtOGE1NS00MGUxYTdlZmQwNWQ` is the migration
source ID, which we'll use in a later step.

### Ensure Bitbucket Cloud Migration Archive exists

An archive should be accessible in the format `bitbucket-export-YYYYMMDD-HHMMSS.tar.gz` or
another specified archive name.

### Upload Archive to GitHub-Owned Storage

Following documentation provided as part of the private preview for
[(GEI) can migrate repositories with GitHub owned blob storage](https://github.com/orgs/community/discussions/144948).

There are two ways to upload your archives: single POST requests for archives up to 5 GiB,
and via a multipart upload flow for archives between 5 MiB and 30 GiB. Depending on the
migration, it may use one archive, or separate archives for Git data and repository metadata
(i.e. pull requests, issues, etc.). In any case, perform the uploads as necessary, and keep
track of the GEI URIs from your uploads to continue with a migration.

#### Performing single uploads to GitHub-owned storage (step 7, <5 GiB)

To perform a single upload, simply submit a POST request with your archive as the POST data to:

- `https://uploads.github.com/organizations/{organization_id}/gei/archive?name={bitbucket-export-YYYYMMDD-HHMMSS.tar.gz}`

Substitute `{organization_id}` with your organization database ID. The ID can be found in the
`"id"` key from [https://api.github.com/orgs/your-organization-login](https://api.github.com/orgs/your-organization-login),
or `databaseID` from Step 3. Please note that the database ID should be an integer
(i.e. `9919`), and not a GraphQL node ID (i.e. `MDEyOk9yZ2FuaXphdGlvbjk5MTk=`).

Also substitute `{bitbucket-export-YYYYMMDD-HHMMSS.tar.gz}` with the filename of your
choice that represents your archive. This value sets the `"name"` field on the archive
metadata for easier management as a user, and the value makes no functional difference
in your migrations. Make sure to keep the `@` in the `--data-binary` value so the archive's
contents are uploaded, and not a literal name of the file.

Make sure to also include your PAT and a content type in the request headers, too.
The content type is required, although it can be always set to `application/octet-stream`.
Here's an example of the request headers necessary for the POST request:

```sh
Authorization: Bearer ghp_12345
Content-Type: application/octet-stream
```

> [!NOTE]
> If you are using Insomnia, please be aware that imported cURL requests will
> incorrectly configure a multipart form that will include extra data in the
> request body and will not upload archives correctly. When creating this request,
> make sure to select "HTTP Request", set the request type to "POST", and in the
> "Body" tab, ensure that the "File" type is selected. Ensure that the headers
> above have been added, too.

Sample Request:

```sh
curl --request POST \
  --header "Authorization: Bearer $GITHUB_TOKEN" \
  --header "Content-Type: application/octet-stream" \
  --data-binary '@{bitbucket-export-YYYYMMDD-HHMMSS.tar.gz}' \
  'https://uploads.github.com/organizations/{organization_id}/gei/archive?name={bitbucket-export-YYYYMMDD-HHMMSS.tar.gz}'
```

The response body will include a JSON object, like so:

```json
{
  "guid": "363b2659-b8a3-4878-bfff-eed4bcb54d35",
  "node_id": "MA_kgDaACQzNjNiMjY1OS1iOGEzLTQ4NzgtYmZmZi1lZWQ0YmNiNTRkMzU",
  "name": "bitbucket-export-YYYYMMDD-HHMMSS.tar.gz",
  "size": 33287,
  "uri": "gei://archive/363b2659-b8a3-4878-bfff-eed4bcb54d35",
  "created_at": "2024-11-13T12:35:45.761-08:00"
}
```

The `"uri"` value contains the GEI URI! This URI represents the uploaded archive,
and will be used to enqueue migrations in step 8!

#### Performing multipart uploads to GitHub-owned storage (step 7, 5-30 GiB)

Multipart uploads are a little more involved, and follow a high-level flow like so:

1. Create the multipart upload with a POST request.
2. Upload 100 MiB of the archive as a PATCH request to a file part.
3. Repeat step 2 with additional parts until the file is fully uploaded.
4. Submit a PUT request to complete the migration.

Here are the steps in more depth! For all requests, ensure to include
credentials in your headers with your PAT, i.e.

```graphql
Authorization: Bearer ghp_12345
```

1. **Start a multipart upload**. Submit a POST request to
   `https://uploads.github.com/organizations/{organization_id}/gei/archive/blobs/uploads`,
   substituting `{organization_id}` with your organization ID. Include a JSON body like
   below with the archive name and size. The content type can remain as `"application/octet-stream"`
   for all uploads.

    ```json
    {
      "content_type": "application/octet-stream",
      "name": "git-archive.tar.gz",
      "size": 262144000
    }
    ```

    This will return a `202` with an empty response body. In the response headers,
    the Location will look like this:

    ```sh
    /organizations/{organization_id}/gei/archive/blobs/uploads?part_number=1&guid=<guid>&upload_id=<upload_id>
    ```

    Keep this path handy, as it'll be used to upload your first file part. We'll call
    it the "next path." Also remember the GUID value, as it'll be used to enqueue a
    migration with the uploaded archive later.

2. **Upload a file part**. Upload 100 MiB of your file to
   `https://uploads.github.com/{location}`, substituting `{location}` with
   the "next path" value. This will return a `202` with an empty response body.
   In the response headers, the Location will look like this:

    ```sh
    /organizations/{organization_id}/gei/archive/blobs/uploads?part_number=2&guid=<guid>&upload_id=<upload_id>
    ```

    Notice that this Location value is identical to the initial Location on step 1,
    except the `part_number` value is incremented. If this is the last file part
    necessary to upload the entire file, we'll need to make a request to the previous
    Location path (not the new one), so keep the old path as "last path," and replace
    "next path" with our new Location path.

3. **Repeat step 2 until the upload is complete**. Ensure that you are reading
   up to 100 MiB of the file at a time, and submitting requests to the new
   Location values with the incremented `part_number` values.

4. **Complete the multipart upload**. Submit a PUT request to the "last path
   value from step 2 with an empty body. If all is well, you'll receive a 201,
   and your upload to GitHub-owned storage is complete! Your GEI URI can be
   constructed with the GUID from step 1 like this: `gei://archive/{guid}`.

> [!Note]
> Example of a Ruby script that will perform the above flow using Faraday and
> Addressable can be found posted [here](https://github.com/orgs/community/discussions/144948).

### Start a Repository Migration

When you start a migration, a single repository and its accompanying data
migrates into a brand new GitHub repository that you identify.

If you want to move multiple repositories at once from the same source
organization,you can queue multiple migrations. You can run up to 5 repository
migrations at the same time.

### `startRepositoryMigration` mutation

```graphql
mutation startRepositoryMigration (
  $sourceId: ID!,
  $ownerId: ID!,
  $repositoryName: String!,
  $continueOnError: Boolean!,
  $accessToken: String!,
  $githubPat: String!,
  $gitArchiveUrl: String!,
  $metadataArchiveUrl: String!,
  $sourceRepositoryUrl: URI!,
  $targetRepoVisibility: String!
){
  startRepositoryMigration( input: {
    sourceId: $sourceId,
    ownerId: $ownerId,
    repositoryName: $repositoryName,
    continueOnError: $continueOnError,
    accessToken: $accessToken,
    githubPat: $githubPat,
    targetRepoVisibility: $targetRepoVisibility
    gitArchiveUrl: $gitArchiveUrl,
    metadataArchiveUrl: $metadataArchiveUrl,
    sourceRepositoryUrl: $sourceRepositoryUrl,
  }) {
    repositoryMigration {
      id
      migrationSource {
        id
        name
        type
      }
      sourceUrl
    }
  }
}
```

>[!Note]
> All GraphQL queries and mutations require the private preview access
> GitHub-owned blob storage header added:
>
> ```graphql
> GraphQL-Features:"octoshift_github_owned_storage"
> ```

<!-- markdownlint-disable MD013 -->
| Query variable|Description|
|:----------------|:-----------|
|`sourceId`|Your migration source `id` returned from the createMigrationSource mutation.|
|`ownerId`|The organization ID of your organization on GitHub Enterprise Cloud.|
|`repositoryName`|A custom unique repository name not currently used by any of your repositories owned by the organization on GitHub Enterprise Cloud. An error-logging issue will be created in this repository when your migration is complete or has stopped. |
|`continueOnError`|Migration setting that allows the migration to continue when encountering errors that don't cause the migration to fail. Must be `true` or `false`. We highly recommend setting `continueOnError` to `true` so that your migration will continue unless the Importer can't move Git source or the Importer has lost connection and cannot reconnect to complete the migration. |
|`githubPat`|The personal access token for your destination organization on GitHub Enterprise Cloud. |
|`accessToken`|The personal access token for your source. |
|`targetRepoVisibility`|The visibility of the new repository. Must be `private`, `public`, or `internal`. If not set, your repository is migrated as private.|
|`gitArchiveUrl`|The storage blob URL from Step 7 |
|`metadataArchiveUrl`| The storage blob URL from Step 7|
|`sourceRepositoryUrl`|The URL for your repository from Bitbucket Cloud, in the format `https://bitbucket.org/{workspace}/{repository}`. This is required for archive validation, but GitHub Enterprise Cloud will not communicate directly. |
<!-- markdownlint-enable MD013 -->

### Check the Migration Status

To detect any migration failures and ensure your migration is working, you can check your migration
status using the `getMigration` query. You can also check the status of multiple migrations
with getMigrations.

The `getMigration` query will return with a status to let you know if the migration is `queued`,
`in progress`, `failed`, or `completed`. If your migration failed, the Importer will provide a reason
for the failure.

### `getMigration` query

```graphql
query (
  $id: ID!
){
  node( id: $id ) {
    ... on Migration {
      id
      sourceUrl
      migrationSource {
        name
      }
      state
      failureReason
    }
  }
}
```

>[!Note]
> All GraphQL queries and mutations require the private preview access header added:
>
> ```graphql
> GraphQL-Features: "octoshift_github_owned_storage"
> ```

| Query variable | Description |
|:---------------|:------------|
| `id` | The `id` that the `startRepositoryMigration` mutation returned. |

Here are the possible values for [**`MigrationState`**](https://docs.github.com/en/enterprise-cloud@latest/graphql/reference/enums#migrationstate)

| Enumeration              | Description                                            |
|:-------------------------|:-------------------------------------------------------|
| **`FAILED`**             | The migration has failed.                              |
| **`FAILED_VALIDATION`**  | The migration has invalid credentials.                 |
| **`IN_PROGRESS`**        | The migration is in progress.                          |
| **`NOT_STARTED`**        | The migration has not started.                         |
| **`PENDING_VALIDATION`** | The migration needs to have its credentials validated. |
| **`QUEUED`**             | The migration has been queued.                         |
| **`SUCCEEDED`**          | The migration has succeeded.                           |

### Validate Migration and Check the Error Log

Ensure the migrated resources are correctly setup and configure your workflows and CI/CD environments
to get your operations up and running. Additional post-migration steps that should be completed:

- [Accessing your migration logs](https://docs.github.com/en/migrations/using-github-enterprise-importer/completing-your-migration-with-github-enterprise-importer/accessing-your-migration-logs-for-github-enterprise-importer)
- [Reclaiming mannequins](https://docs.github.com/en/migrations/using-github-enterprise-importer/completing-your-migration-with-github-enterprise-importer/reclaiming-mannequins-for-github-enterprise-importer)
