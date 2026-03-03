#!/bin/bash
set -e

CAFILE=/mosquitto/certs/ca.crt
HOST=localhost
PORT=8883
ADMIN_USER=admin
ADMIN_PASS=$1  # 第一個參數帶入密碼

if [ -z "$ADMIN_PASS" ]; then
  echo "Usage: $0 <admin_password>"
  exit 1
fi

CMD="docker exec -it mosquitto mosquitto_ctrl --cafile $CAFILE --insecure -h $HOST -p $PORT -u $ADMIN_USER -P $ADMIN_PASS"

echo "=== 初始化 Dynamic Security ==="
docker exec -it mosquitto mosquitto_ctrl \
  --cafile $CAFILE --insecure \
  -h $HOST -p $PORT \
  dynsec init /mosquitto/data/dynamic-security.json $ADMIN_USER $ADMIN_PASS

echo "=== 建立 orion-server Role ==="
$CMD dynsec createRole orion-server
$CMD dynsec addRoleACL orion-server subscribePattern "talos/#" allow
$CMD dynsec addRoleACL orion-server publishClientSend "talos/+/command" allow
$CMD dynsec addRoleACL orion-server publishClientSend "talos/+/config" allow
$CMD dynsec addRoleACL orion-server publishClientSend "talos/+/ota" allow
$CMD dynsec addRoleACL orion-server publishClientSend "orion/broadcast/#" allow

echo "=== 建立 edge Role ==="
$CMD dynsec createRole edge
$CMD dynsec addRoleACL edge publishClientSend "talos/%u/telemetry" allow
$CMD dynsec addRoleACL edge publishClientSend "talos/%u/status" allow
$CMD dynsec addRoleACL edge publishClientSend "talos/%u/event" allow
$CMD dynsec addRoleACL edge publishClientSend "talos/%u/response" allow
$CMD dynsec addRoleACL edge subscribeLiteral "talos/%u/command" allow
$CMD dynsec addRoleACL edge subscribeLiteral "talos/%u/config" allow
$CMD dynsec addRoleACL edge subscribeLiteral "talos/%u/ota" allow
$CMD dynsec addRoleACL edge subscribePattern "orion/broadcast/#" allow

echo "=== 建立 edge-devices Group ==="
$CMD dynsec createGroup edge-devices
$CMD dynsec addGroupRole edge-devices edge

echo "=== 建立 orion Client ==="
$CMD dynsec createClient orion -p $2  # 第二個參數為 orion 密碼
$CMD dynsec addClientRole orion orion-server

echo "=== 完成！==="
$CMD dynsec getClient orion
