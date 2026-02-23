package main

import (
	"dblocker_control/internal/infrastructure/mqtt"
	"dblocker_control/internal/models"
	"dblocker_control/internal/service"
	"fmt"
)

const (
	brokerURL = "tcp://localhost:1883"
	serialNum = "250001"
	clientID  = "dblocker-config-publisher"
)

// publishDBlockerConfig accepts 14 boolean values in this exact bit order:
// [
//
//	ch1_gps, ch1_ctrl,
//	ch2_gps, ch2_ctrl,
//	ch3_gps, ch3_ctrl,
//	fan_master,
//	ch4_gps, ch4_ctrl,
//	ch5_gps, ch5_ctrl,
//	ch6_gps, ch6_ctrl,
//	fan_slave,
//
// ]
//
// Then it converts to the same bitmask format as UpdateDBlockerConfig and publishes to MQTT.
func publishDBlockerConfig(flags ...bool) error {
	if len(flags) != 14 {
		return fmt.Errorf("invalid flags length: expected 14, got %d", len(flags))
	}

	cfg := []models.DBlockerConfig{
		{SignalGPS: flags[0], SignalCtrl: flags[1]},
		{SignalGPS: flags[2], SignalCtrl: flags[3]},
		{SignalGPS: flags[4], SignalCtrl: flags[5]},
		{SignalGPS: flags[7], SignalCtrl: flags[8]},
		{SignalGPS: flags[9], SignalCtrl: flags[10]},
		{SignalGPS: flags[11], SignalCtrl: flags[12]},
	}

	fanMaster := flags[6]
	fanSlave := flags[13]

	mask, err := service.DBlockerConfigToBitmask(cfg, fanMaster, fanSlave)
	if err != nil {
		return err
	}

	payload := []byte{
		byte(mask >> 8),
		byte(mask),
	}

	topic := fmt.Sprintf("dbl/%s/cmd", serialNum)

	client, err := mqtt.New(brokerURL, clientID)
	if err != nil {
		return err
	}
	defer client.Close()

	if err := client.Publish(topic, 1, true, payload); err != nil {
		return err
	}

	fmt.Printf("published topic=%s mask=%d (0x%04X) payload=%v\n", topic, mask, mask, payload)
	return nil
}

func main() {
	// Example: 14 booleans in bit order.
	err := publishDBlockerConfig(
		true, true,
		true, true,
		true, true,
		true,
		true, true,
		true, true,
		true, true,
		true,
	)
	if err != nil {
		panic(err)
	}
}
