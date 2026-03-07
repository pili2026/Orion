package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// DynsecService manages Mosquitto Dynamic Security clients over MQTT.
// It publishes commands to $CONTROL/dynamic-security/v1 — the official
// Mosquitto dynsec API — instead of shelling out to mosquitto_ctrl.
type DynsecService struct {
	mqtt mqtt.Client
}

// NewDynsecService creates a new DynsecService.
func NewDynsecService(mqttClient mqtt.Client) *DynsecService {
	return &DynsecService{mqtt: mqttClient}
}

const (
	dynsecTopic         = "$CONTROL/dynamic-security/v1"
	dynsecResponseTopic = "$CONTROL/dynamic-security/v1/response"
	dynsecEdgeGroup     = "edge-devices"
	dynsecTimeout       = 5 * time.Second
)

// dynsecCommand is the envelope Mosquitto expects on the control topic.
type dynsecCommand struct {
	Commands []map[string]any `json:"commands"`
}

// CreateEdgeClient registers a new Edge device in Mosquitto Dynamic Security:
//  1. Creates the MQTT client with the given username and password.
//  2. Adds the client to the "edge-devices" group so it inherits the
//     "edge" role ACLs (publish telemetry/status/event/response,
//     subscribe command/config/ota/broadcast).
func (d *DynsecService) CreateEdgeClient(edgeID, password string) error {
	cmd := dynsecCommand{
		Commands: []map[string]any{
			{
				"command":  "createClient",
				"username": edgeID,
				"password": password,
			},
			{
				"command":   "addGroupClient",
				"groupname": dynsecEdgeGroup,
				"username":  edgeID,
			},
		},
	}

	if err := d.publish(cmd); err != nil {
		return fmt.Errorf("dynsec CreateEdgeClient %q: %w", edgeID, err)
	}

	slog.Info("Dynsec: edge client created", slog.String("edge_id", edgeID))
	return nil
}

// DeleteEdgeClient removes the Edge device client from Mosquitto.
// The client is first removed from the group (ACL revocation), then deleted.
func (d *DynsecService) DeleteEdgeClient(edgeID string) error {
	cmd := dynsecCommand{
		Commands: []map[string]any{
			{
				"command":   "removeGroupClient",
				"groupname": dynsecEdgeGroup,
				"username":  edgeID,
			},
			{
				"command":  "deleteClient",
				"username": edgeID,
			},
		},
	}

	if err := d.publish(cmd); err != nil {
		return fmt.Errorf("dynsec DeleteEdgeClient %q: %w", edgeID, err)
	}

	slog.Info("Dynsec: edge client deleted", slog.String("edge_id", edgeID))
	return nil
}

// publish serialises cmd to JSON and publishes it to the dynsec control topic.
// QoS 1 ensures the broker acknowledges delivery.
func (d *DynsecService) publish(cmd dynsecCommand) error {
	payload, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("marshal dynsec command: %w", err)
	}

	token := d.mqtt.Publish(dynsecTopic, 1, false, payload)
	if !token.WaitTimeout(dynsecTimeout) {
		return fmt.Errorf("dynsec publish timed out after %s", dynsecTimeout)
	}
	if token.Error() != nil {
		return fmt.Errorf("dynsec publish: %w", token.Error())
	}
	return nil
}
