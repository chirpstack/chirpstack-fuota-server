package storage

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

func UpdateDeviceStatus(ctx context.Context, db sqlx.Execer, deviceId int, status int) error {

	res, err := db.Exec(`
		update device set
			status = $1,
		where
			deviceId = $2`,
		status,
		deviceId,
	)
	if err != nil {
		return fmt.Errorf("sql update error: %w", err)
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected error: %w", err)
	}
	if ra == 0 {
		return ErrDoesNotExist
	}

	log.WithFields(log.Fields{
		"deviceId": deviceId,
		"status":   status,
	}).Info("storage: device status updated")

	return nil
}

func UpdateDeviceFirmwareVersion(ctx context.Context, db sqlx.Execer, deviceId int, firmwareVersion string) error {

	res, err := db.Exec(`
		update device set
			firmwareVersion = $1,
		where
			deviceId = $2`,
		firmwareVersion,
		deviceId,
	)
	if err != nil {
		return fmt.Errorf("sql update error: %w", err)
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected error: %w", err)
	}
	if ra == 0 {
		return ErrDoesNotExist
	}

	log.WithFields(log.Fields{
		"deviceId":        deviceId,
		"firmwareVersion": firmwareVersion,
	}).Info("storage: device status updated")

	return nil
}

func InsertDevice(ctx context.Context, db sqlx.Execer, deviceId int, deviceCode string, modelId int, profileId int, firmwareVersion string, region string, macVersion string, regionParameter string, status int) error {

	insertQuery := `
		INSERT INTO device (
			deviceId, 
			deviceCode, 
			modelId, 
			profileId, 
			firmwareVersion, 
			region, 
			macVersion, 
			regionParameter, 
			status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	res, err := db.Exec(insertQuery,
		deviceId,
		deviceCode,
		modelId,
		profileId,
		firmwareVersion,
		region,
		macVersion,
		regionParameter,
		status,
	)

	if err != nil {
		return fmt.Errorf("sql update error: %w", err)
	}
	ra, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected error: %w", err)
	}
	if ra == 0 {
		return ErrDoesNotExist
	}

	log.WithFields(log.Fields{
		"deviceId":        deviceId,
		"deviceCode":      deviceCode,
		"modelId":         modelId,
		"profileId":       profileId,
		"firmwareVersion": firmwareVersion,
		"region":          region,
		"macVersion":      macVersion,
		"regionParameter": regionParameter,
		"status":          status,
	}).Info("storage: device inserted")

	return nil
}
