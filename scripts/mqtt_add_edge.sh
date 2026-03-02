#!/usr/bin/env bash
set -euo pipefail

MOSQ_CONTAINER="mosquitto"
MOSQ_CONFIG_DIR="/opt/mosquitto/config"
PASSFILE="$MOSQ_CONFIG_DIR/passwordfile"
ACLFILE="$MOSQ_CONFIG_DIR/aclfile"

MODE="restart"   # default
EDGE_ID=""
TOPIC_PREFIX=""

usage() {
  echo ""
  echo "Usage:"
  echo "  $0 <edge_id> [--hup|--restart]"
  echo ""
  echo "Example:"
  echo "  $0 edgeC"
  echo "  $0 site01-edge01 --hup"
  echo ""
  exit 1
}

# ----------------------------
# Parse arguments
# ----------------------------
if [[ $# -lt 1 ]]; then
  usage
fi

EDGE_ID="$1"
shift || true

if [[ "${1:-}" == "--hup" ]]; then
  MODE="hup"
elif [[ "${1:-}" == "--restart" ]]; then
  MODE="restart"
fi

TOPIC_PREFIX="talos/${EDGE_ID}/#"

# ----------------------------
# Validation
# ----------------------------
if [[ ! "$EDGE_ID" =~ ^[A-Za-z0-9._-]+$ ]]; then
  echo "❌ Invalid edge_id (only A-Z a-z 0-9 . _ - allowed)"
  exit 2
fi

# ----------------------------
# Ensure files exist
# ----------------------------
sudo mkdir -p "$MOSQ_CONFIG_DIR"
sudo touch "$PASSFILE" "$ACLFILE"
sudo chmod 600 "$PASSFILE"

echo "=============================="
echo " Adding MQTT user: $EDGE_ID"
echo " Topic prefix: $TOPIC_PREFIX"
echo " Mode: $MODE"
echo "=============================="

# ----------------------------
# Add or update password
# ----------------------------
docker run --rm -it \
  -v "$MOSQ_CONFIG_DIR:/mosquitto/config" \
  eclipse-mosquitto:2 \
  sh -c "mosquitto_passwd /mosquitto/config/$(basename "$PASSFILE") '$EDGE_ID'"

# ----------------------------
# Add ACL if not exists
# ----------------------------
if ! grep -qE "^user $EDGE_ID\$" "$ACLFILE"; then
  {
    echo ""
    echo "user $EDGE_ID"
    echo "topic readwrite $TOPIC_PREFIX"
  } | sudo tee -a "$ACLFILE" >/dev/null
  echo "✅ ACL added"
else
  echo "ℹ️  User already exists in ACL"
fi

# ----------------------------
# Reload broker
# ----------------------------
if [[ "$MODE" == "hup" ]]; then
  echo "🔄 Reloading broker (HUP)"
  docker kill -s HUP "$MOSQ_CONTAINER"
else
  echo "🔄 Restarting broker"
  docker restart "$MOSQ_CONTAINER" >/dev/null
fi

echo ""
echo "🎉 DONE: $EDGE_ID ready"
echo ""
