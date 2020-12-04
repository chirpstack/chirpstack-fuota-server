package storage

import (
	"context"
	"database/sql"
	"testing"

	"github.com/brocaar/lorawan"
	"github.com/lib/pq/hstore"
	"github.com/stretchr/testify/require"
)

func (ts *StorageTestSuite) TestDeploymentLog() {
	assert := require.New(ts.T())

	d := Deployment{}
	assert.NoError(CreateDeployment(context.Background(), ts.Tx(), &d))

	ts.T().Run("Create", func(t *testing.T) {
		assert := require.New(t)

		dl := DeploymentLog{
			DeploymentID: d.ID,
			DevEUI:       lorawan.EUI64{1, 2, 3, 4, 5, 6, 7, 8},
			FPort:        10,
			Command:      "FooReq",
			Fields: hstore.Hstore{
				Map: map[string]sql.NullString{
					"foo": sql.NullString{String: "bar", Valid: true},
				},
			},
		}

		assert.NoError(CreateDeploymentLog(context.Background(), ts.Tx(), &dl))

		t.Run("GetDeploymentLogsForDevice", func(t *testing.T) {
			assert := require.New(t)

			logs, err := GetDeploymentLogsForDevice(context.Background(), ts.Tx(), d.ID, dl.DevEUI)
			assert.NoError(err)
			assert.Len(logs, 1)
		})
	})
}
