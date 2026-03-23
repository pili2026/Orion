# Orion MQTT Topics

This document defines all MQTT topics used between Orion (broker subscriber) and Talos Edge devices (publishers), including topic patterns, payload schemas, QoS, and expected frequencies.

---

## Overview

| Direction     | Publisher         | Subscriber        |
| ------------- | ----------------- | ----------------- |
| Talos → Orion | Talos Edge device | Orion server      |
| Orion → Talos | Orion server      | Talos Edge device |

All Talos-originated topics follow the pattern `talos/{mqtt_username}/{type}`, where `mqtt_username` is the unique identifier provisioned via the Dynamic Security Plugin.

---

## Talos → Orion Topics

### `talos/{mqtt_username}/telemetry`

Periodic sensor and device readings.

| Field         | Value             |
| ------------- | ----------------- |
| QoS           | 1 (at least once) |
| Frequency     | TBD               |
| Orion Handler | `handleTelemetry` |
| Status        | ✅ Implemented    |

**Payload Schema**

```json
{
  "ts": 1710000000000,
  "readings": [
    {
      "type": "SE",
      "device_code": "TECO_1",
      "value": 220.5,
      "unit": "V"
    },
    {
      "type": "ST",
      "device_code": "ADAM-4117_2",
      "pin": 1,
      "value": 25.3,
      "unit": "°C"
    }
  ]
}
```

| Field                    | Type    | Required               | Description                                                       |
| ------------------------ | ------- | ---------------------- | ----------------------------------------------------------------- |
| `ts`                     | int64   | ✅                     | Device-side Unix timestamp (milliseconds)                         |
| `readings`               | array   | ✅                     | One or more readings in this payload                              |
| `readings[].type`        | string  | ✅                     | Device type code (`SE` / `CI` / `SF` / `ST` / `SP` / `SR` / `SO`) |
| `readings[].device_code` | string  | ✅                     | Device identifier                                                 |
| `readings[].value`       | float64 | ✅                     | Measured value                                                    |
| `readings[].unit`        | string  | ✅                     | Unit of measurement                                               |
| `readings[].pin`         | int     | ✅ (sensor types only) | Port index; required for `ST` / `SP` / `SR` / `SO`                |

---

### `talos/{mqtt_username}/status`

Device heartbeat and connectivity status.

| Field         | Value                                  |
| ------------- | -------------------------------------- |
| QoS           | 1 (at least once)                      |
| Frequency     | TBD                                    |
| Orion Handler | `handleStatus`                         |
| Status        | 🔴 Stub only — handler not implemented |

**Payload Schema**

TBD — to be designed. Expected fields:

| Field      | Type   | Description                                     |
| ---------- | ------ | ----------------------------------------------- |
| `ts`       | int64  | Device-side Unix timestamp (milliseconds)       |
| `status`   | string | Connectivity status (e.g. `online` / `offline`) |
| `ip`       | string | (Optional) Device IP address                    |
| `firmware` | string | (Optional) Firmware version                     |

**Orion Actions (on receipt)**

- Update `gateways.last_seen_at`
- Update `gateways.network_status`

---

### `talos/{mqtt_username}/event`

Asynchronous event notifications (alarms, threshold breaches, state changes).

| Field         | Value                                  |
| ------------- | -------------------------------------- |
| QoS           | 1 (at least once)                      |
| Frequency     | Event-driven                           |
| Orion Handler | `handleEvent`                          |
| Status        | 🔴 Stub only — handler not implemented |

**Payload Schema**

TBD

---

### `talos/{mqtt_username}/response`

Acknowledgement or result messages in response to commands sent by Orion.

| Field         | Value                                  |
| ------------- | -------------------------------------- |
| QoS           | 1 (at least once)                      |
| Frequency     | On-demand (triggered by command)       |
| Orion Handler | `handleResponse`                       |
| Status        | 🔴 Stub only — handler not implemented |

**Payload Schema**

TBD

---

## Orion → Talos Topics

> These topics are provisioned in Mosquitto Dynamic Security but handlers are not yet implemented.

| Topic                           | Purpose                         | Status             |
| ------------------------------- | ------------------------------- | ------------------ |
| `talos/{mqtt_username}/command` | Send control commands to device | 🔴 Not implemented |
| `talos/{mqtt_username}/config`  | Push configuration updates      | 🔴 Not implemented |
| `talos/{mqtt_username}/ota`     | Trigger OTA firmware update     | 🔴 Not implemented |
| `orion/broadcast/{channel}`     | Broadcast to all Edge devices   | 🔴 Not implemented |

---

## Notes

- `{mqtt_username}` maps to `gateways.mqtt_username` in the Orion database.
- Orion subscribes with QoS 1 (`talos/#`). Talos publishes with QoS TBD (to be confirmed per topic).
- `SetupMQTTSubscribers()` is called on every broker reconnect to restore subscriptions after restart.
- Mosquitto Dynamic Security Plugin expands `%u` to the connecting client's username in ACL rules, ensuring each Talos device can only publish to its own topics.
