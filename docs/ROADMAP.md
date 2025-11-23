# Potions Roadmap

## Overview
This roadmap outlines the path to making Potions robust for binary distribution and package management.

## Phase 1: Core Reliability (High Priority)

### 1.1 Download Resilience
- [ ] Add retry logic with exponential backoff to `downloadFile()` in downloader
- [ ] Support multiple mirror URLs per artifact
- [ ] Implement automatic failover between mirrors
- [ ] Add connection timeout and error handling improvements

### 1.2 Mirror Infrastructure
- [ ] Set up Wasabi S3-compatible storage ($6/month for 1TB)
- [ ] Set up Cloudflare R2 storage (free tier: 10GB)
- [ ] Create GitHub Actions workflow to sync releases to mirrors
- [ ] Implement mirror health monitoring

### 1.3 Artifact Manifest
- [ ] Generate manifest JSON with all mirror URLs
- [ ] Include checksums (SHA256) for verification
- [ ] Add metadata (version, platform, build date)
- [ ] Publish manifest to mirrors alongside artifacts

### 1.4 Source Fallback
- [ ] Enhance existing `source_build` support
- [ ] Implement automatic fallback when binary download fails
- [ ] Add build caching to speed up rebuilds
- [ ] Document source build requirements per recipe

## Phase 2: Build Quality (Medium Priority)

### 2.1 Reproducible Builds
- [ ] Standardize build environments using Docker
- [ ] Pin all build tool versions
- [ ] Generate reproducible build metadata
- [ ] Add build verification tests

### 2.2 Binary Verification
- [ ] Implement smoke tests for built binaries
- [ ] Add platform-specific validation
- [ ] Verify runtime dependencies
- [ ] Test extracted binaries before packaging

### 2.3 Security Enhancements
- [x] SBOM generation
- [x] Provenance attestation
- [ ] Add vulnerability scanning with Grype/Trivy
- [ ] Implement signing of artifacts

## Phase 3: Distribution Optimization (Medium Priority)

### 3.1 CDN Integration
- [ ] Configure Cloudflare CDN in front of mirrors
- [ ] Set up cache rules for static assets
- [ ] Implement cache invalidation for bad releases
- [ ] Add geo-routing for optimal performance

### 3.2 Bandwidth Optimization
- [ ] Implement artifact compression
- [ ] Add delta/incremental update support
- [ ] Optimize tarball structure
- [ ] Implement parallel downloads for large files

### 3.3 Partial Failure Handling
- [ ] Allow release with subset of platforms
- [ ] Mark unavailable platforms clearly
- [ ] Implement platform-specific retry logic
- [ ] Add graceful degradation strategies

## Phase 4: Monitoring & Operations (Low Priority)

### 4.1 Observability
- [ ] Track download success/failure metrics
- [ ] Monitor mirror availability and latency
- [ ] Set up alerting for build failures
- [ ] Create operational dashboard

### 4.2 Cost Management
- [ ] Monitor storage costs across providers
- [ ] Implement lifecycle policies (archive old versions)
- [ ] Track bandwidth usage
- [ ] Optimize storage tier selection

### 4.3 Maintenance Automation
- [ ] Automated mirror health checks
- [ ] Automatic failover on mirror failure
- [ ] Self-healing for transient issues
- [ ] Cleanup of stale artifacts

## Success Metrics

### Reliability
- **Target:** 99.9% artifact availability
- **Target:** < 1% download failure rate
- **Target:** Average download time < 10s per artifact

### Coverage
- **Target:** 95% of recipes have binaries for all 4 platforms
- **Target:** 100% of recipes have source fallback

### Performance
- **Target:** Build time < 15 minutes per platform
- **Target:** Total release pipeline < 1 hour

## Dependencies

### External Services
- GitHub Actions (CI/CD)
- Wasabi (primary mirror)
- Cloudflare R2 (secondary mirror)
- Cloudflare CDN (edge caching)

### Tools
- rclone or AWS CLI (mirror sync)
- Docker (reproducible builds)
- Cosign (artifact signing - future)

## Notes

- This roadmap focuses on **binary distribution** (this repo)
- CLI features (install, upgrade, doctor, etc.) are tracked in the CLI repo
- Priorities may shift based on user feedback and production issues
- Success metrics should be reviewed quarterly
