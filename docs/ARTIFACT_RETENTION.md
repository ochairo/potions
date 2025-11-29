# Artifact Retention and Security Policy

## Workflow Artifact Retention

Configure retention periods for different artifact types to balance security and storage costs:

```yaml
# Build artifacts (short retention - replaced by releases)
retention-days: 3

# Release reports (medium retention - audit trail)
retention-days: 30

# Security attestations (long retention - compliance)
retention-days: 90
```

## Current Retention Settings

| Artifact Type | Retention | Workflow | Reason |
|--------------|-----------|----------|--------|
| `*-builds` | 3 days | Build workflows | Replaced by GitHub Releases |
| `*-complete` | 1 day | Build workflows | Temporary build status markers |
| `*-failures` | 30 days | Build workflows | Debugging and failure tracking |
| `failure-tracking-*` | 30 days | Build workflows | Historical failure data |
| `release-report` | 30 days | Release workflow | Audit trail |

## Security Implications

### Short Retention (1-3 days)
- **Build artifacts**: Minimize attack surface - artifacts are quickly moved to releases
- **Temporary markers**: No long-term storage of build state
- **Cost optimization**: Reduces storage costs for transient data

### Medium Retention (30 days)
- **Failure reports**: Sufficient for debugging and failure pattern analysis
- **Release reports**: Audit trail for recent releases
- **Balance**: Long enough for investigation, short enough for security

### Long Retention (90+ days)
- **Security attestations**: Should match GitHub Release retention (indefinite)
- **Provenance**: Critical for supply chain security audits
- **SLSA attestations**: Required for compliance verification

## Recommendations

1. **Enable GitHub Release Retention**: Keep releases indefinitely
   - Releases contain all critical artifacts
   - Includes checksums, SBOM, provenance, signatures

2. **Automated Cleanup**: GitHub Actions artifacts auto-delete after retention period
   - No manual cleanup needed
   - Reduces storage costs

3. **Backup Critical Artifacts**: For compliance, consider:
   - Storing SLSA provenance in external system
   - Backing up signing keys to secure vault
   - Archiving security attestations externally

4. **Monitor Storage Usage**:
   ```bash
   # Check artifact storage usage
   gh api repos/ochairo/potions/actions/artifacts --jq '.total_count'
   ```

## Cleanup Script (Manual)

If needed, manually clean old artifacts:

```bash
#!/bin/bash
# cleanup-old-artifacts.sh
# WARNING: This deletes artifacts older than 30 days

REPO="ochairo/potions"
CUTOFF_DATE=$(date -d "30 days ago" +%Y-%m-%d 2>/dev/null || date -v-30d +%Y-%m-%d)

gh api repos/$REPO/actions/artifacts \
  --jq ".artifacts[] | select(.created_at < \"$CUTOFF_DATE\") | .id" | \
while read -r artifact_id; do
  echo "Deleting artifact $artifact_id"
  gh api -X DELETE repos/$REPO/actions/artifacts/$artifact_id
done
```

## Compliance Considerations

For SLSA Level 3 compliance:
- ✅ Provenance must be available for verification
- ✅ Attestations must be retrievable
- ✅ Audit trail must be maintained

Our approach:
- Provenance stored in GitHub Releases (indefinite)
- Attestations attached to release artifacts (.attestation.jsonl)
- Build logs available via GitHub Actions (90 days default)
