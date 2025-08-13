# Breaking Changes in v2.0.0

This document provides detailed information about breaking changes introduced in version 2.0.0 of `gh-bbc-exporter`.

## Overview

Version 2.0.0 introduces significant improvements to authentication handling, including support for Bitbucket's new API tokens and better security practices. These changes require updates to existing usage patterns.

## Breaking Changes Summary

| Category | Change | Impact |
|----------|--------|---------|
| CLI Flags | `--token` → `--access-token` | **HIGH** - All existing scripts need updates |
| Environment Variables | `BITBUCKET_TOKEN` → `BITBUCKET_ACCESS_TOKEN` | **HIGH** - All existing configurations need updates |
| Authentication Methods | Added API token support | **MEDIUM** - New recommended method available |
| Deprecations | App passwords deprecated | **LOW** - Still works until 2026 |

## Detailed Breaking Changes

### 1. Command-Line Interface Changes

#### Flag Rename: `--token` to `--access-token`

**What changed**: The primary authentication flag has been renamed for clarity.

**Before (v1.x)**:
```bash
gh bbc-exporter --token your-workspace-token -w workspace -r repository
```

**After (v2.0.0)**:
```bash
gh bbc-exporter --access-token your-workspace-token -w workspace -r repository
```

**Why this changed**: The new name better reflects that this is specifically for workspace access tokens, not the new API tokens.

#### New Authentication Flags

**Added in v2.0.0**:
- `--api-token`: For Bitbucket API tokens (recommended)
- `--email`: Required when using API tokens

**Example usage**:
```bash
gh bbc-exporter --api-token your-api-token --email you@example.com -w workspace -r repository
```

### 2. Environment Variable Changes

#### Variable Rename: `BITBUCKET_TOKEN` to `BITBUCKET_ACCESS_TOKEN`

**What changed**: The environment variable for workspace tokens has been renamed.

**Before (v1.x)**:
```bash
export BITBUCKET_TOKEN="your-workspace-token"
gh bbc-exporter -w workspace -r repository
```

**After (v2.0.0)**:
```bash
export BITBUCKET_ACCESS_TOKEN="your-workspace-token"
gh bbc-exporter -w workspace -r repository
```

#### New Environment Variables

**Added in v2.0.0**:
- `BITBUCKET_API_TOKEN`: For API token authentication
- `BITBUCKET_EMAIL`: Required with API tokens

**Example usage**:
```bash
export BITBUCKET_API_TOKEN="your-api-token"
export BITBUCKET_EMAIL="you@example.com"
gh bbc-exporter -w workspace -r repository
```

### 3. Authentication Method Changes

#### New Authentication Priority

When multiple authentication methods are configured, the tool now follows this priority:

1. **Workspace Access Token** (if `--access-token` or `BITBUCKET_ACCESS_TOKEN` is set)
2. **API Token** (if `--api-token` and `--email` or `BITBUCKET_API_TOKEN` and `BITBUCKET_EMAIL` are set)
3. **Basic Authentication** (if `--user` and `--app-password` or `BITBUCKET_USERNAME` and `BITBUCKET_APP_PASSWORD` are set)

#### Mixed Authentication Detection

The tool now detects and warns about mixed authentication methods:

```
Warning: Multiple authentication methods detected. Workspace access token authentication will be used.
```

#### Authentication Validation

Enhanced validation now requires complete authentication sets:
- API tokens require an email address
- Basic authentication requires both username and app password

## Deprecation Notices

### App Password Deprecation

**What's deprecated**: Bitbucket app passwords and basic authentication

**Timeline**:
- **September 9, 2025**: No new app passwords can be created
- **June 9, 2026**: Existing app passwords stop working

**Warning message**:
```
Warning: Bitbucket app passwords are deprecated and will be discontinued after September 9, 2025.
Please consider switching to Bitbucket API tokens instead.
```

**Migration path**: Switch to API tokens before September 2025.

## Migration Strategies

### 1. Immediate Migration (Recommended)

**For Workspace Access Tokens**:
1. Update all scripts: `--token` → `--access-token`
2. Update environment variables: `BITBUCKET_TOKEN` → `BITBUCKET_ACCESS_TOKEN`
3. Test the changes

**For API Tokens (Best Practice)**:
1. Create API tokens in Bitbucket with required scopes
2. Update scripts to use `--api-token` and `--email`
3. Update environment variables to use `BITBUCKET_API_TOKEN` and `BITBUCKET_EMAIL`
4. Test the changes

### 2. Gradual Migration

**Phase 1**: Update flag/variable names only
- Keep using workspace access tokens
- Update `--token` to `--access-token`
- Update `BITBUCKET_TOKEN` to `BITBUCKET_ACCESS_TOKEN`

**Phase 2**: Migrate to API tokens
- Create API tokens in Bitbucket
- Switch to `--api-token` and `--email` flags
- Update environment variables accordingly

### 3. CI/CD Pipeline Updates

**GitHub Actions Example**:

Before (v1.x):
```yaml
- name: Export from Bitbucket
  env:
    BITBUCKET_TOKEN: ${{ secrets.BITBUCKET_TOKEN }}
  run: |
    gh bbc-exporter --token $BITBUCKET_TOKEN -w workspace -r repo
```

After (v2.0.0) - Workspace Token:
```yaml
- name: Export from Bitbucket
  env:
    BITBUCKET_ACCESS_TOKEN: ${{ secrets.BITBUCKET_ACCESS_TOKEN }}
  run: |
    gh bbc-exporter --access-token $BITBUCKET_ACCESS_TOKEN -w workspace -r repo
```

After (v2.0.0) - API Token (Recommended):
```yaml
- name: Export from Bitbucket
  env:
    BITBUCKET_API_TOKEN: ${{ secrets.BITBUCKET_API_TOKEN }}
    BITBUCKET_EMAIL: ${{ secrets.BITBUCKET_EMAIL }}
  run: |
    gh bbc-exporter --api-token $BITBUCKET_API_TOKEN --email $BITBUCKET_EMAIL -w workspace -r repo
```

## Testing Your Migration

### 1. Verify Authentication Works

Test your new authentication setup:

```bash
# Test workspace access token
gh bbc-exporter --access-token your-token -w workspace -r repo --debug

# Test API token
gh bbc-exporter --api-token your-api-token --email you@example.com -w workspace -r repo --debug
```

### 2. Check for Warnings

Run with debug logging to ensure no unexpected warnings:
- No mixed authentication warnings
- No deprecation warnings (unless using app passwords)

### 3. Validate Export Functionality

Ensure your exports still work correctly:
- Repository data is exported
- Pull requests are included
- User mappings are correct

## Support

If you encounter issues during migration:

1. **Check the logs**: Use `--debug` flag for detailed logging
2. **Verify credentials**: Ensure your tokens have the correct permissions
3. **Review authentication method**: Confirm you're using the intended auth method
4. **Check documentation**: Refer to the main README for setup instructions

## Frequently Asked Questions

### Q: Do I need to update immediately?

**A**: For the flag/variable renames, yes. For migrating from app passwords to API tokens, you have until September 2025, but we recommend migrating soon for better security.

### Q: Can I use both workspace tokens and API tokens?

**A**: Yes, but the tool will use workspace tokens with higher priority and warn about mixed authentication.

### Q: What if I forget to update and use the old flags?

**A**: The tool will return an authentication error since the old `--token` flag no longer exists.

### Q: Are API tokens more secure than workspace tokens?

**A**: API tokens provide more granular permissions and are the recommended approach by Atlassian. Workspace tokens are still secure but may provide broader access.

### Q: How do I create API tokens?

**A**: Follow the detailed guide in the main README.md file under "API Tokens" section.