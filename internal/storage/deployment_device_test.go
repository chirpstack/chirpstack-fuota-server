package storage

import (
	"context"
	"testing"
	"time"

	"github.com/brocaar/lorawan"
	"github.com/stretchr/testify/require"
)

func (ts *StorageTestSuite) TestDeploymentDevice() {
	assert := require.New(ts.T())

	d := Deployment{}
	assert.NoError(CreateDeployment(context.Background(), ts.Tx(), &d))

	ts.T().Run("Create", func(t *testing.T) {
		assert := require.New(t)

		dd := DeploymentDevice{
			DeploymentID: d.ID,
			DevEUI:       lorawan.EUI64{1, 2, 3, 4, 5, 6, 7, 8},
		}
		assert.NoError(CreateDeploymentDevice(context.Background(), ts.Tx(), &dd))

		dd.CreatedAt = dd.CreatedAt.UTC()
		dd.UpdatedAt = dd.UpdatedAt.UTC()

		t.Run("Get", func(t *testing.T) {
			assert := require.New(t)

			ddGet, err := GetDeploymentDevice(context.Background(), ts.Tx(), d.ID, dd.DevEUI)
			assert.NoError(err)

			ddGet.CreatedAt = ddGet.CreatedAt.UTC()
			ddGet.UpdatedAt = ddGet.UpdatedAt.UTC()
			assert.Equal(dd, ddGet)
		})

		t.Run("Update", func(t *testing.T) {
			assert := require.New(t)

			now := time.Now().Round(time.Millisecond)
			dd.MCGroupSetupCompletedAt = &now
			dd.MCSessionCompletedAt = &now
			dd.FragSessionSetupCompletedAt = &now
			dd.FragStatusCompletedAt = &now

			assert.NoError(UpdateDeploymentDevice(context.Background(), ts.Tx(), &dd))

			ddGet, err := GetDeploymentDevice(context.Background(), ts.Tx(), d.ID, dd.DevEUI)
			assert.NoError(err)

			assert.True(ddGet.MCGroupSetupCompletedAt.Equal(now))
			assert.True(ddGet.MCSessionCompletedAt.Equal(now))
			assert.True(ddGet.FragSessionSetupCompletedAt.Equal(now))
			assert.True(ddGet.FragStatusCompletedAt.Equal(now))
		})

		t.Run("GetDeploymentDevices", func(t *testing.T) {
			assert := require.New(t)

			list, err := GetDeploymentDevices(context.Background(), ts.Tx(), d.ID)
			assert.NoError(err)
			assert.Len(list, 1)
		})
	})
}
