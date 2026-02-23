package service

import (
	"dblocker_control/internal/models"
	"errors"
)

// DBlockerConfigToBitmask converts config into a 14-bit mask
func DBlockerConfigToBitmask(
	cfg []models.DBlockerConfig,
	fanMaster bool,
	fanSlave bool,
) (uint16, error) {

	if len(cfg) != 6 {
		return 0, errors.New("invalid config length: expected 6")
	}

	var mask uint16
	bit := 0

	set := func(v bool) {
		if v {
			mask |= 1 << bit
		}
		bit++
	}

	// ⚠️ ORDER MUST MATCH MCU SIDE
	set(cfg[0].SignalGPS)
	set(cfg[0].SignalCtrl)

	set(cfg[1].SignalGPS)
	set(cfg[1].SignalCtrl)

	set(cfg[2].SignalGPS)
	set(cfg[2].SignalCtrl)

	set(fanMaster)

	set(cfg[3].SignalGPS)
	set(cfg[3].SignalCtrl)

	set(cfg[4].SignalGPS)
	set(cfg[4].SignalCtrl)

	set(cfg[5].SignalGPS)
	set(cfg[5].SignalCtrl)

	set(fanSlave)

	return mask, nil
}
