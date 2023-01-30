package eventhandler

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/protobuf/ptypes"
	log "github.com/sirupsen/logrus"

	"github.com/brocaar/lorawan"
	"github.com/brocaar/lorawan/applayer/clocksync"
	"github.com/brocaar/lorawan/gps"
	"github.com/chirpstack/chirpstack-fuota-server/v4/internal/client/as"
	"github.com/chirpstack/chirpstack/api/go/v4/api"
	"github.com/chirpstack/chirpstack/api/go/v4/integration"
)

func handleClockSyncCommand(ctx context.Context, pl integration.UplinkEvent) error {
	var devEUI lorawan.EUI64
	if err := devEUI.UnmarshalText([]byte(pl.GetDeviceInfo().GetDevEui())); err != nil {
		return err
	}

	// get uplink time
	var timeSinceGPSEpoch time.Duration
	var timeField time.Time
	var err error

	for _, rxInfo := range pl.RxInfo {
		if rxInfo.TimeSinceGpsEpoch != nil {
			timeSinceGPSEpoch, err = ptypes.Duration(rxInfo.TimeSinceGpsEpoch)
			if err != nil {
				log.WithError(err).Error("eventhandler: time since gps epoch to duration error")
				continue
			}
		} else if rxInfo.Time != nil {
			timeField, err = ptypes.Timestamp(rxInfo.Time)
			if err != nil {
				log.WithError(err).Error("eventhandler: time to timeestamp error")
				continue
			}
		}
	}

	// fallback on time field when time since GPS epoch is not available
	if timeSinceGPSEpoch == 0 {
		// fallback on current server time when time field is not available
		if timeField.IsZero() {
			timeField = time.Now()
		}
		timeSinceGPSEpoch = gps.Time(timeField).TimeSinceGPSEpoch()
	}

	var cmd clocksync.Command
	if err := cmd.UnmarshalBinary(true, pl.Data); err != nil {
		return fmt.Errorf("unmarshal command error: %w", err)
	}

	log.WithFields(log.Fields{
		"dev_eui": devEUI,
		"cid":     cmd.CID,
	}).Info("eventhandler: clocksync command received")

	switch cmd.CID {
	case clocksync.AppTimeReq:
		pl, ok := cmd.Payload.(*clocksync.AppTimeReqPayload)
		if !ok {
			return fmt.Errorf("expected *clocksync.AppTimeReqPayload expected, got: %T", cmd.Payload)
		}
		return handleClockSyncAppTimeReq(ctx, devEUI, timeSinceGPSEpoch, pl)
	}

	return nil
}

func handleClockSyncAppTimeReq(ctx context.Context, devEUI lorawan.EUI64, timeSinceGPSEpoch time.Duration, pl *clocksync.AppTimeReqPayload) error {
	deviceGPSTime := int64(pl.DeviceTime)
	networkGPSTime := int64((timeSinceGPSEpoch / time.Second) % (1 << 32))

	log.WithFields(log.Fields{
		"dev_eui":      devEUI,
		"device_time":  pl.DeviceTime,
		"ans_required": pl.Param.AnsRequired,
		"token_req":    pl.Param.TokenReq,
	}).Info("eventhandler: AppTimeReq received")

	ans := clocksync.Command{
		CID: clocksync.AppTimeAns,
		Payload: &clocksync.AppTimeAnsPayload{
			TimeCorrection: int32(networkGPSTime - deviceGPSTime),
			Param: clocksync.AppTimeAnsPayloadParam{
				TokenAns: pl.Param.TokenReq,
			},
		},
	}
	b, err := ans.MarshalBinary()
	if err != nil {
		return fmt.Errorf("marshal command error: %w", err)
	}

	// enqueue response
	_, err = as.DeviceClient().Enqueue(ctx, &api.EnqueueDeviceQueueItemRequest{
		QueueItem: &api.DeviceQueueItem{
			DevEui: devEUI.String(),
			FPort:  uint32(clocksync.DefaultFPort),
			Data:   b,
		},
	})
	if err != nil {
		return fmt.Errorf("enqueue payload error: %w", err)
	}

	return nil
}
