#!/bin/bash
# WhatsApp Bridge V2 daemon launcher
# Used by launchd to start the bridge as a background service.

BRIDGE_DIR="$(cd "$(dirname "$0")" && pwd)"
BINARY="$BRIDGE_DIR/whatsapp-bridge"
CONFIG="$BRIDGE_DIR/config.yaml"

cd "$BRIDGE_DIR"
exec "$BINARY" -config "$CONFIG" -daemon
