package storage

import (
	"context"
	"testing"
	"time"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

func (ts *StorageTestSuite) TestDeployment() {
	ts.T().Run("Create", func(t *testing.T) {
		assert := require.New(t)

		d := Deployment{}
		assert.NoError(CreateDeployment(context.Background(), ts.Tx(), &d))
		assert.NotEqual(uuid.Nil, d.ID)
		assert.Less(int64(time.Now().Sub(d.CreatedAt)), int64(time.Second))
		assert.Less(int64(time.Now().Sub(d.UpdatedAt)), int64(time.Second))

		d.CreatedAt = d.CreatedAt.UTC()
		d.UpdatedAt = d.UpdatedAt.UTC()

		t.Run("Get", func(t *testing.T) {
			assert := require.New(t)

			dGet, err := GetDeployment(context.Background(), ts.Tx(), d.ID)
			assert.NoError(err)
			assert.Equal(d, dGet)

			dGet.CreatedAt = dGet.CreatedAt.UTC()
			dGet.UpdatedAt = dGet.UpdatedAt.UTC()
			assert.Nil(dGet.MCGroupSetupCompletedAt)
			assert.Nil(dGet.MCSessionCompletedAt)
			assert.Nil(dGet.FragSessionSetupCompletedAt)
			assert.Nil(dGet.EnqueueCompletedAt)
			assert.Nil(dGet.FragStatusCompletedAt)
		})

		t.Run("Update", func(t *testing.T) {
			assert := require.New(t)

			now := time.Now().Round(time.Millisecond).UTC()
			d.MCGroupSetupCompletedAt = &now
			d.MCSessionCompletedAt = &now
			d.FragSessionSetupCompletedAt = &now
			d.EnqueueCompletedAt = &now
			d.FragStatusCompletedAt = &now

			assert.NoError(UpdateDeployment(context.Background(), ts.Tx(), &d))

			dGet, err := GetDeployment(context.Background(), ts.Tx(), d.ID)
			assert.NoError(err)

			assert.True(dGet.MCGroupSetupCompletedAt.Equal(now))
			assert.True(dGet.MCSessionCompletedAt.Equal(now))
			assert.True(dGet.FragSessionSetupCompletedAt.Equal(now))
			assert.True(dGet.EnqueueCompletedAt.Equal(now))
			assert.True(dGet.FragStatusCompletedAt.Equal(now))
		})
	})
}
