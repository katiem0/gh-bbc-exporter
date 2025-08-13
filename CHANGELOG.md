# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [2.0.0] - 2025-01-01

### 🚨 BREAKING CHANGES

This release introduces significant changes to authentication methods and command-line interface. Please review the migration guide below.

#### Command-Line Flag Changes

- **BREAKING**: The `--token` / `-t` flag has been renamed to `--access-token` / `-t`
  - **Before**: `gh bbc-exporter --token your-token`
  - **After**: `gh bbc-exporter --access-token your-token`

#### Environment Variable Changes

- **BREAKING**: The `BITBUCKET_TOKEN` environment variable has been renamed to `BITBUCKET_ACCESS_TOKEN`
  - **Before**: `export BITBUCKET_TOKEN="your-token"`
  - **After**: `export BITBUCKET_ACCESS_TOKEN="your-token"`

#### New Authentication Methods

- **NEW**: API Token authentication (recommended method)
  - Use `--api-token` flag with `--email` flag or environment variables `BITBUCKET_API_TOKEN` and `BITBUCKET_EMAIL`
  - Example: `gh bbc-exporter --api-token your-api-token --email your-email@example.com`

#### Authentication Method Priority

When multiple authentication methods are provided, the following priority order is used:
1. **Workspace Access Token** (highest priority)
2. **API Token with Email**
3. **Basic Authentication with App Password** (deprecated)

#### Deprecation Notices

- **DEPRECATED**: App password authentication is deprecated and will be discontinued after September 9, 2025
  - Existing app passwords will stop working on June 9, 2026
  - Please migrate to API tokens for future compatibility

### 📋 Migration Guide

#### From v1.x to v2.0.0

1. **Update Command-Line Usage**:
   ```bash
   # OLD (v1.x)
   gh bbc-exporter --token your-workspace-token -w workspace -r repo
   
   # NEW (v2.0.0) - Workspace Access Token
   gh bbc-exporter --access-token your-workspace-token -w workspace -r repo
   
   # NEW (v2.0.0) - API Token (Recommended)
   gh bbc-exporter --api-token your-api-token --email your-email@example.com -w workspace -r repo
   ```

2. **Update Environment Variables**:
   ```bash
   # OLD (v1.x)
   export BITBUCKET_TOKEN="your-workspace-token"
   
   # NEW (v2.0.0) - Workspace Access Token
   export BITBUCKET_ACCESS_TOKEN="your-workspace-token"
   
   # NEW (v2.0.0) - API Token (Recommended)
   export BITBUCKET_API_TOKEN="your-api-token"
   export BITBUCKET_EMAIL="your-email@example.com"
   ```

3. **Update CI/CD Pipelines**:
   - Replace any usage of `--token` with `--access-token`
   - Replace any usage of `BITBUCKET_TOKEN` with `BITBUCKET_ACCESS_TOKEN`
   - Consider migrating to API tokens for better security

4. **Create API Tokens** (Recommended):
   - Follow the [API Token creation guide](https://support.atlassian.com/bitbucket-cloud/docs/create-an-api-token/) in the README
   - Required scopes: `read:account`, `read:pullrequest:bitbucket`, `read:repository:bitbucket`, `read:workspace:bitbucket`

### ✨ Added

- New API token authentication method with email support
- Enhanced authentication validation with clear error messages
- Multiple authentication method detection with priority handling
- Deprecation warnings for app passwords
- Support for `BITBUCKET_API_TOKEN` and `BITBUCKET_EMAIL` environment variables

### 🔧 Changed

- Renamed `--token` flag to `--access-token` for clarity
- Renamed `BITBUCKET_TOKEN` environment variable to `BITBUCKET_ACCESS_TOKEN`
- Improved authentication logging with method-specific messages
- Enhanced command help text with clearer authentication option descriptions

### 🗑️ Deprecated

- App password authentication (will be discontinued after September 9, 2025)
- Basic authentication with username/app-password combination
- Note: Existing app passwords will continue to work until June 9, 2026

### 🛡️ Security

- API tokens provide more granular permissions compared to app passwords
- Enhanced authentication method validation prevents mixed-auth confusion
- Clear deprecation path for legacy authentication methods

---

## [1.4.5] - 2024-08-05

### 🐛 Fixed

- Fixed failed test specifications
- Addressed golang lint issues
- Improved test reliability

---

*For older versions, please see the [GitHub Releases](https://github.com/katiem0/gh-bbc-exporter/releases) page.*