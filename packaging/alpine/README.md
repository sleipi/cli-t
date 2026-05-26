# Alpine APK Packaging

## Files

- `APKBUILD` — for future aports submission (not used in release workflow)
- `clitest.rsa` — private signing key (gitignored, also stored as `APK_PRIVATE_KEY` GitHub secret)
- `clitest.rsa.pub` — public signing key (shipped with releases)
- `.gitignore` — excludes private key

## Local Testing

```bash
# Build snapshot (produces .apk in dist/)
NFPM_APK_KEY_FILE=packaging/alpine/clitest.rsa goreleaser release --snapshot --clean

# Verify install in Alpine container (use arm64 on Apple Silicon, amd64 on x86)
podman run --rm \
  -v ./dist:/pkg \
  -v ./packaging/alpine/clitest.rsa.pub:/etc/apk/keys/clitest.rsa.pub \
  alpine sh -c "apk add /pkg/clitest_*_arm64.apk && clitest --version"
```

## Regenerating Keys

```bash
cd packaging/alpine
openssl genrsa -traditional -out clitest.rsa 4096
openssl rsa -in clitest.rsa -pubout -out clitest.rsa.pub
```

After regenerating, update the `APK_PRIVATE_KEY` GitHub Actions secret with `cat clitest.rsa`.
