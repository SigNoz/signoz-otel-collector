# Flux Webhook Integration

This document describes how the signoz-otel-collector image build triggers immediate Flux ImageRepository scans.

## Setup

The Flux webhook receiver at `https://flux.bankrut.net` is configured to receive push notifications and trigger immediate ImageRepository scans instead of waiting for the 20-minute polling interval.

### Environment Variables

Add these to your GitHub repository secrets (Settings > Secrets and variables > Actions):

- `FLUX_WEBHOOK_URL`: `https://flux.bankrut.net`
- `FLUX_WEBHOOK_TOKEN`: Obtained from `kubectl -n flux-system get receiver ghcr-signoz-otel-collector -o jsonpath='{.status.webhookPath}'` (path component after `/hook/`)

### Workflow Configuration

Add this step to your image push workflow (after `docker push`):

```yaml
- name: Trigger Flux ImageRepository scan
  if: success()
  run: |
    WEBHOOK_PATH="${{ secrets.FLUX_WEBHOOK_TOKEN }}"
    PAYLOAD='{"type":"generic"}'
    SIGNATURE="sha256=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hex | cut -d' ' -f2)"
    
    curl -X POST \
      "https://flux.bankrut.net/hook/$WEBHOOK_PATH" \
      -H "Content-Type: application/json" \
      -H "X-Signature: $SIGNATURE" \
      -d "$PAYLOAD" \
      -v
```

## Example: Full Build and Push Workflow

```yaml
name: Build and Push signoz-otel-collector

on:
  push:
    branches:
      - main
      - develop
    paths:
      - 'cmd/**'
      - 'Makefile'
      - '.github/workflows/build-push.yaml'

jobs:
  build-push:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
    
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3
      
      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      
      - name: Extract version
        id: version
        run: |
          VERSION=$(grep -oP 'VERSION=\K[0-9.]+' Makefile | head -1)
          echo "version=$VERSION" >> $GITHUB_OUTPUT
      
      - name: Build and push
        uses: docker/build-push-action@v5
        with:
          context: .
          push: true
          tags: |
            ghcr.io/${{ github.repository }}:v${{ steps.version.outputs.version }}
            ghcr.io/${{ github.repository }}:latest
          cache-from: type=registry,ref=ghcr.io/${{ github.repository }}:buildcache
          cache-to: type=registry,ref=ghcr.io/${{ github.repository }}:buildcache,mode=max
      
      - name: Trigger Flux ImageRepository scan
        if: success()
        run: |
          WEBHOOK_TOKEN="${{ secrets.FLUX_WEBHOOK_TOKEN }}"
          PAYLOAD='{"type":"generic"}'
          SIGNATURE="sha256=$(echo -n "$PAYLOAD" | openssl dgst -sha256 -hex | cut -d' ' -f2)"
          
          curl -X POST \
            "https://flux.bankrut.net/hook/$WEBHOOK_TOKEN" \
            -H "Content-Type: application/json" \
            -H "X-Signature: $SIGNATURE" \
            -d "$PAYLOAD" \
            --fail-with-body \
            -v
```

## Verification

After pushing a new image:

1. Check the Flux webhook receiver received the request:
   ```bash
   kubectl -n flux-system logs -l app.kubernetes.io/name=flux-system --tail=50 | grep webhook
   ```

2. Verify ImageRepository scanned immediately:
   ```bash
   flux -n signoz get imagerepository signoz-otel-collector
   ```

3. Watch for automatic image policy updates:
   ```bash
   kubectl -n signoz get imagepolicy -w
   ```

## Troubleshooting

**Webhook returns 404:**
- Verify `FLUX_WEBHOOK_TOKEN` is correct
- Check that Receiver is ready: `kubectl -n flux-system get receiver ghcr-signoz-otel-collector`

**Signature validation fails:**
- Ensure payload and HMAC calculation match exactly
- Verify `X-Signature` header format: `sha256=<hex-digest>`

**ImageRepository doesn't scan:**
- Check Receiver logs: `kubectl -n flux-system logs -l app.kubernetes.io/name=notification-controller`
- Verify ImagePolicy is configured: `kubectl -n signoz get imagepolicy`
