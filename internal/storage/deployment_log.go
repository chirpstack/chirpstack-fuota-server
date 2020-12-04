package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/gofrs/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq/hstore"
	log "github.com/sirupsen/logrus"

	"github.com/brocaar/lorawan"
)

// DeploymentLog contains a deployment log item.
type DeploymentLog struct {
	ID           int64         `db:"id"`
	CreatedAt    time.Time     `db:"created_at"`
	DeploymentID uuid.UUID     `db:"deployment_id"`
	DevEUI       lorawan.EUI64 `db:"dev_eui"`
	FPort        uint8         `db:"f_port"`
	Command      string        `db:"command"`
	Fields       hstore.Hstore `db:"fields"`
}

// CreateDeploymentLog creates the given DeploymentLog.
func CreateDeploymentLog(ctx context.Context, db sqlx.Queryer, dl *DeploymentLog) error {
	dl.CreatedAt = time.Now()

	err := sqlx.Get(db, &dl.ID, `
		insert into deployment_log (
			created_at,
			deployment_id,
			dev_eui,
			f_port,
			command,
			fields
		) values (
			$1, $2, $3, $4, $5, $6)
		returning
			id`,
		dl.CreatedAt,
		dl.DeploymentID,
		dl.DevEUI,
		dl.FPort,
		dl.Command,
		dl.Fields,
	)
	if err != nil {
		return fmt.Errorf("sql create error: %w", err)
	}

	log.WithFields(log.Fields{
		"deployment_id": dl.DeploymentID,
		"dev_eui":       dl.DevEUI,
		"command":       dl.Command,
	}).Info("storage: deployment log created")

	return nil
}

// GetDeploymentLogsForDevice returns the deployment logs given a deployment ID and DevEUI.
func GetDeploymentLogsForDevice(ctx context.Context, db sqlx.Queryer, deploymentID uuid.UUID, devEUI lorawan.EUI64) ([]DeploymentLog, error) {
	var logs []DeploymentLog
	err := sqlx.Select(db, &logs, `
		select
			*
		from
			deployment_log
		where
			deployment_id = $1
			and dev_eui = $2
		order by
			created_at`,
		deploymentID,
		devEUI,
	)
	if err != nil {
		return nil, fmt.Errorf("sql select error: %w", err)
	}

	return logs, nil
}
