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
			deployment_id = $2`,
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
