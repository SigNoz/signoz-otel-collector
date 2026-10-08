# Flux Webhook Integration

This document describes how the signoz-otel-collector image build triggers immediate Flux ImageRepository scans.

Placeholders in angle brackets (`<...>`) and `example.com` hostnames must be replaced with your own values.

## Setup

The Flux webhook receiver at `https://flux-webhook.example.com` is configured to receive push notifications and trigger immediate ImageRepository scans instead of waiting for the default polling interval.

### Environment Variables

Add these to your GitHub repository secrets (Settings > Secrets and variables > Actions):

- `FLUX_WEBHOOK_URL`: `https://flux-webhook.example.com`
- `FLUX_WEBHOOK_TOKEN`: the webhook path component of the Flux Receiver, i.e. the part after `/hook/` in its `status.webhookPath`. Obtain it from the cluster administrator, or with `kubectl --context <context> -n <flux-namespace> get receiver <receiver-name> -o jsonpath='{.status.webhookPath}'`.

### Workflow Configuration

Add this step to your image push workflow (after `docker push`):

```yaml
- name: Trigger Flux ImageRepository scan
  if: success()
  run: |
    WEBHOOK_TOKEN="${{ secrets.FLUX_WEBHOOK_TOKEN }}"

    curl -X POST \
      "${{ secrets.FLUX_WEBHOOK_URL }}/hook/$WEBHOOK_TOKEN" \
      -H "Content-Type: application/json" \
      -d '{"type":"generic"}' \
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

          curl -X POST \
            "${{ secrets.FLUX_WEBHOOK_URL }}/hook/$WEBHOOK_TOKEN" \
            -H "Content-Type: application/json" \
            -d '{"type":"generic"}' \
            --fail-with-body \
            -v
```

## Verification

After pushing a new image:

1. Check the Flux notification controller received the request:
   ```bash
   kubectl --context <context> -n <flux-namespace> logs -l app.kubernetes.io/name=notification-controller --tail=50 | grep webhook
   ```

2. Verify the ImageRepository scanned immediately:
   ```bash
   flux --context <context> -n <image-namespace> get imagerepository <image-repository>
   ```

3. Watch for automatic image policy updates:
   ```bash
   kubectl --context <context> -n <image-namespace> get imagepolicy -w
   ```

## Troubleshooting

**Webhook returns 404:**
- Verify `FLUX_WEBHOOK_TOKEN` is correct
- Check that the Receiver is ready: `kubectl --context <context> -n <flux-namespace> get receiver <receiver-name>`
- The webhook path is derived from the token, Receiver name, and namespace. Re-read `status.webhookPath` if the Receiver was recreated.

**Receiver rejects requests:**
- The Receiver must be `type: generic`. The `generic-hmac` type requires an `X-Signature` header, which this workflow does not send.

**ImageRepository doesn't scan:**
- Check Receiver logs: `kubectl --context <context> -n <flux-namespace> logs -l app.kubernetes.io/name=notification-controller`
- Verify the ImagePolicy is configured: `kubectl --context <context> -n <image-namespace> get imagepolicy`
