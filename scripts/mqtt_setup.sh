#!/bin/bash
# mqtt_setup.sh — One-time initialisation of Mosquitto Dynamic Security.
#
# Run this once after the broker starts for the first time, or after a full
# reset of dynamic-security.json.
#
# Usage:
#   ./scripts/mqtt_setup.sh <admin_password> <orion_mqtt_password>
#
# Example:
#   ./scripts/mqtt_setup.sh myAdminPass myOrionPass
#
set -euo pipefail

CAFILE=/mosquitto/certs/ca.crt
HOST=localhost
PORT=8883
ADMIN_USER=admin
ADMIN_PASS="${1:?Usage: $0 <admin_password> <orion_mqtt_password>}"
ORION_PASS="${2:?Usage: $0 <admin_password> <orion_mqtt_password>}"

CMD="docker exec mosquitto mosquitto_ctrl --cafile $CAFILE --insecure -h $HOST -p $PORT -u $ADMIN_USER -P $ADMIN_PASS"

echo "=== 初始化 Dynamic Security ==="
docker exec mosquitto mosquitto_ctrl \
  --cafile $CAFILE --insecure \
  -h $HOST -p $PORT \
  dynsec init /mosquitto/data/dynamic-security.json $ADMIN_USER $ADMIN_PASS

echo "=== 設定預設 ACL（拒絕所有未明確允許的 publish）==="
# publishClientSend must be deny so that only roles with explicit allow ACLs
# can publish. Without this, any authenticated client can publish anywhere.
$CMD dynsec setDefaultACLAccess publishClientSend deny
$CMD dynsec setDefaultACLAccess publishClientReceive allow
$CMD dynsec setDefaultACLAccess subscribe deny
$CMD dynsec setDefaultACLAccess unsubscribe allow

echo "=== 建立 orion-server Role ==="
$CMD dynsec createRole orion-server
$CMD dynsec addRoleACL orion-server subscribePattern 'talos/#' allow
$CMD dynsec addRoleACL orion-server publishClientSend 'talos/+/command' allow
$CMD dynsec addRoleACL orion-server publishClientSend 'talos/+/config' allow
$CMD dynsec addRoleACL orion-server publishClientSend 'talos/+/ota' allow
$CMD dynsec addRoleACL orion-server publishClientSend 'orion/broadcast/#' allow

echo "=== 建立 edge Role ==="
# %u is expanded by the Dynamic Security Plugin to the connecting client's
# username, ensuring each Edge device can only publish to its own topics.
$CMD dynsec createRole edge
$CMD dynsec addRoleACL edge publishClientSend 'talos/%u/telemetry' allow
$CMD dynsec addRoleACL edge publishClientSend 'talos/%u/status' allow
$CMD dynsec addRoleACL edge publishClientSend 'talos/%u/event' allow
$CMD dynsec addRoleACL edge publishClientSend 'talos/%u/response' allow
$CMD dynsec addRoleACL edge subscribeLiteral 'talos/%u/command' allow
$CMD dynsec addRoleACL edge subscribeLiteral 'talos/%u/config' allow
$CMD dynsec addRoleACL edge subscribeLiteral 'talos/%u/ota' allow
$CMD dynsec addRoleACL edge subscribePattern 'orion/broadcast/#' allow

echo "=== 建立 edge-devices Group ==="
$CMD dynsec createGroup edge-devices
$CMD dynsec addGroupRole edge-devices edge

echo "=== 建立 orion Client ==="
$CMD dynsec createClient orion -p "$ORION_PASS"
$CMD dynsec addClientRole orion orion-server

echo ""
echo "=== 完成！驗證 orion client ==="
$CMD dynsec getClient orion
echo ""
echo "Mosquitto Dynamic Security 初始化完成。"
echo "使用 ./scripts/mqtt_add_edge.sh 來新增 Edge 設備。"