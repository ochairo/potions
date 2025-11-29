# GitHub Environment Protection Setup

## Required Environment Configurations

### 1. Production Release Environment

Create a `production` environment with the following protection rules:

**Repository Settings → Environments → New Environment → "production"**

#### Protection Rules
- ✅ **Required reviewers**: At least 1 admin approval required
- ✅ **Wait timer**: 0 minutes (optional: add delay for manual verification)
- ✅ **Deployment branches**: Only `main` branch

#### Environment Secrets
```yaml
GPG_PRIVATE_KEY: <Your GPG private key in ASCII armor format>
GPG_PASSPHRASE: <Your GPG key passphrase (if protected)>
```

#### Environment Variables
```yaml
GPG_SIGNING_ENABLED: "true"  # Enable GPG signing (set to "false" to disable)
```

### 2. Workflow Updates Needed

Update release workflow to use the protected environment:

```yaml
jobs:
  release-packages:
    name: Release Packages
    runs-on: ubuntu-latest
    environment: production  # ← Add this line
    timeout-minutes: 120
```

### 3. Security Benefits

- **Manual Approval**: Releases require human review before executing
- **Audit Trail**: All approvals are logged
- **Secrets Protection**: GPG keys only accessible in production environment
- **Branch Protection**: Only main branch can trigger production releases

### 4. Setup Instructions

1. Go to **Repository Settings**
2. Click **Environments** (left sidebar)
3. Click **New environment**
4. Name it `production`
5. Enable **Required reviewers** and add yourself
6. Set **Deployment branches** to `Selected branches` → Add `main`
7. Add secrets: `GPG_PRIVATE_KEY`, `GPG_PASSPHRASE` (optional)
8. Add variable: `GPG_SIGNING_ENABLED` = `true`

### 5. GPG Key Generation (If Needed)

```bash
# Generate new GPG key
gpg --full-generate-key
# Choose: RSA and RSA, 4096 bits, no expiration

# Export private key (ASCII armor format)
gpg --armor --export-secret-keys YOUR_EMAIL > private-key.asc

# Copy contents to GitHub secret GPG_PRIVATE_KEY
cat private-key.asc

# Get key ID for verification
gpg --list-secret-keys --keyid-format=long YOUR_EMAIL

# Export public key for users
gpg --armor --export YOUR_EMAIL > KEYS

# Securely delete local private key file
shred -u private-key.asc
```

### 6. Testing

After setup, test with a manual workflow dispatch:
```bash
# Trigger manual release
gh workflow run scheduled-release.yml -f package=kubectl -f version=v1.28.0
```

The workflow will pause at the release step and require approval.

## Security Checklist

- [ ] Production environment created
- [ ] Required reviewers configured (minimum 1)
- [ ] GPG signing enabled (if desired)
- [ ] GPG keys added to environment secrets
- [ ] Deployment branch restricted to `main`
- [ ] Workflow updated to use `environment: production`
- [ ] Test release with approval flow
- [ ] Document public GPG key location (KEYS file)
