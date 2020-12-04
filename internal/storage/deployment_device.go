package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/gofrs/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"

	"github.com/brocaar/lorawan"
)

// DeploymentDevice represents a device within a FUOTA deployment.
type DeploymentDevice struct {
	DeploymentID                uuid.UUID     `db:"deployment_id"`
	DevEUI                      lorawan.EUI64 `db:"dev_eui"`
	CreatedAt                   time.Time     `db:"created_at"`
	UpdatedAt                   time.Time     `db:"updated_at"`
	MCGroupSetupCompletedAt     *time.Time    `db:"mc_group_setup_completed_at"`
	MCSessionCompletedAt        *time.Time    `db:"mc_session_completed_at"`
	FragSessionSetupCompletedAt *time.Time    `db:"frag_session_setup_completed_at"`
	FragStatusCompletedAt       *time.Time    `db:"frag_status_completed_at"`
}

// CreateDeploymentDevice creates the given DeploymentDevice.
func CreateDeploymentDevice(ctx context.Context, db sqlx.Execer, dd *DeploymentDevice) error {
	now := time.Now().Round(time.Millisecond)
	dd.CreatedAt = now
	dd.UpdatedAt = now

	_, err := db.Exec(`
		insert into deployment_device (
			deployment_id,
			dev_eui,
			created_at,
			updated_at,
			mc_group_setup_completed_at,
			mc_session_completed_at,
			frag_session_setup_completed_at,
			frag_status_completed_at
		) values ($1, $2, $3, $4, $5, $6, $7, $8)`,
		dd.DeploymentID,
		dd.DevEUI,
		dd.CreatedAt,
		dd.UpdatedAt,
		dd.MCGroupSetupCompletedAt,
		dd.MCSessionCompletedAt,
		dd.FragSessionSetupCompletedAt,
		dd.FragStatusCompletedAt,
	)
	if err != nil {
		return fmt.Errorf("sql exec error: %w", err)
	}

	log.WithFields(log.Fields{
		"deployment_id": dd.DeploymentID,
		"dev_eui":       dd.DevEUI,
	}).Info("storage: deployment device created")

	return nil
}

// GetDeploymentDevice returns the DeploymentDevice for the given Deployment ID and DevEUI.
func GetDeploymentDevice(ctx context.Context, db sqlx.Queryer, deploymentID uuid.UUID, devEUI lorawan.EUI64) (DeploymentDevice, error) {
	var dd DeploymentDevice
	err := sqlx.Get(db, &dd, "select * from deployment_device where deployment_id = $1 and dev_eui = $2", deploymentID, devEUI)
	if err != nil {
		return dd, fmt.Errorf("sql select error: %w", err)
	}

	return dd, nil
}

// UpdateDeploymentDevice updates the given DeploymentDevice.
func UpdateDeploymentDevice(ctx context.Context, db sqlx.Execer, dd *DeploymentDevice) error {
	dd.UpdatedAt = time.Now()

	res, err := db.Exec(`
		update deployment_device set
			updated_at = $3,
			mc_group_setup_completed_at = $4,
			mc_session_completed_at = $5,
			frag_session_setup_completed_at = $6,
			frag_status_completed_at = $7
		where
			deployment_id = $1 and dev_eui = $2`,
		dd.DeploymentID,
		dd.DevEUI,
		dd.UpdatedAt,
		dd.MCGroupSetupCompletedAt,
		dd.MCSessionCompletedAt,
		dd.FragSessionSetupCompletedAt,
		dd.FragStatusCompletedAt,
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
		"deployment_id": dd.DeploymentID,
		"dev_eui":       dd.DevEUI,
	}).Info("storage: deployment device updated")

	return nil
}

// GetDeploymentDevices returns the DeploymentDevices for the given Deployment ID.
func GetDeploymentDevices(ctx context.Context, db sqlx.Queryer, deploymentID uuid.UUID) ([]DeploymentDevice, error) {
	var dds []DeploymentDevice
	err := sqlx.Select(db, &dds, "select * from deployment_device where deployment_id = $1", deploymentID)
	if err != nil {
		return nil, fmt.Errorf("sql select error: %w", err)
	}

	return dds, nil
}
