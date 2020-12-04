package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/gofrs/uuid"
	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"
)

// Deployment represents a FUOTA deployment.
type Deployment struct {
	ID                          uuid.UUID  `db:"id"`
	CreatedAt                   time.Time  `db:"created_at"`
	UpdatedAt                   time.Time  `db:"updated_at"`
	MCGroupSetupCompletedAt     *time.Time `db:"mc_group_setup_completed_at"`
	MCSessionCompletedAt        *time.Time `db:"mc_session_completed_at"`
	FragSessionSetupCompletedAt *time.Time `db:"frag_session_setup_completed_at"`
	EnqueueCompletedAt          *time.Time `db:"enqueue_completed_at"`
	FragStatusCompletedAt       *time.Time `db:"frag_status_completed_at"`
}

// CreateDeployment creates the given Deployment.
func CreateDeployment(ctx context.Context, db sqlx.Execer, d *Deployment) error {
	if d.ID == uuid.Nil {
		id, err := uuid.NewV4()
		if err != nil {
			return fmt.Errorf("new uuid error: %w", err)
		}

		d.ID = id
	}

	now := time.Now().Round(time.Millisecond)
	d.CreatedAt = now
	d.UpdatedAt = now

	_, err := db.Exec(`
		insert into deployment (
			id,
			created_at,
			updated_at,
			mc_group_setup_completed_at,
			mc_session_completed_at,
			frag_session_setup_completed_at,
			enqueue_completed_at,
			frag_status_completed_at
		) values ($1, $2, $3, $4, $5, $6, $7, $8)`,
		d.ID,
		d.CreatedAt,
		d.UpdatedAt,
		d.MCGroupSetupCompletedAt,
		d.MCSessionCompletedAt,
		d.FragSessionSetupCompletedAt,
		d.EnqueueCompletedAt,
		d.FragStatusCompletedAt,
	)
	if err != nil {
		return fmt.Errorf("sql exec error: %w", err)
	}

	log.WithFields(log.Fields{
		"id": d.ID,
	}).Info("storage: deployment created")

	return nil
}

// GetDeployment returns the Deployment given an ID.
func GetDeployment(ctx context.Context, db sqlx.Queryer, id uuid.UUID) (Deployment, error) {
	var d Deployment
	err := sqlx.Get(db, &d, "select * from deployment where id = $1", id)
	if err != nil {
		return d, fmt.Errorf("sql select error: %w", err)
	}

	return d, nil
}

// UpdateDeployment updates the given Deployment.
func UpdateDeployment(ctx context.Context, db sqlx.Execer, d *Deployment) error {
	d.UpdatedAt = time.Now()

	res, err := db.Exec(`
		update deployment set
			updated_at = $2,
			mc_group_setup_completed_at = $3,
			mc_session_completed_at = $4,
			frag_session_setup_completed_at = $5,
			enqueue_completed_at = $6,
			frag_status_completed_at = $7
		where
			id = $1`,
		d.ID,
		d.UpdatedAt,
		d.MCGroupSetupCompletedAt,
		d.MCSessionCompletedAt,
		d.FragSessionSetupCompletedAt,
		d.EnqueueCompletedAt,
		d.FragStatusCompletedAt,
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
		"id": d.ID,
	}).Info("storage: deployment updated")

	return nil
}
