# GitHub Secret Scanning Custom Patterns

# Enable this via: Repository Settings → Security → Code security and analysis

## Custom Secret Patterns

### GPG Private Key Pattern
```regex
-----BEGIN PGP PRIVATE KEY BLOCK-----[\s\S]*?-----END PGP PRIVATE KEY BLOCK-----
```
**Description**: Detects GPG private keys in ASCII armor format

### Cosign Private Key Pattern
```regex
-----BEGIN ENCRYPTED COSIGN PRIVATE KEY-----[\s\S]*?-----END ENCRYPTED COSIGN PRIVATE KEY-----
```
**Description**: Detects Cosign signing keys

### GitHub Personal Access Token (Classic)
```regex
ghp_[0-9a-zA-Z]{36}
```
**Description**: Detects classic GitHub PAT tokens

### GitHub Personal Access Token (Fine-grained)
```regex
github_pat_[0-9a-zA-Z_]{82}
```
**Description**: Detects fine-grained GitHub PAT tokens

## How to Configure

1. Go to **Repository Settings**
2. Click **Code security and analysis**
3. Enable **Secret scanning**
4. Enable **Push protection**
5. Click **Custom patterns**
6. Add each pattern above

## Push Protection

Push protection will:
- ✅ Block commits containing secrets
- ✅ Prevent accidental secret exposure
- ✅ Alert contributors before push

## False Positives

If you get false positives from test data:

1. Add to `.gitignore`:
   ```
   **/test/fixtures/keys/*
   **/test/testdata/gpg/*
   ```

2. Or use secret scanning allowlist:
   - Repository Settings → Secret scanning → Allowlist
   - Add specific file paths to exclude

## Existing Secrets Remediation

If secrets are found in history:

1. **Rotate immediately**: Generate new keys/tokens
2. **Remove from history**: Use `git-filter-repo` or GitHub support
3. **Update workflows**: Update GitHub secrets with new values
4. **Audit access**: Check if compromised credentials were used

## Monitoring

GitHub will send alerts for:
- Secrets found in code
- Secrets found in commit history
- Secrets leaked in pull requests

Subscribe to security alerts:
- Repository → Settings → Notifications → Security alerts
