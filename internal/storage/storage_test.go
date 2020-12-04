package storage

import (
	"testing"

	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/brocaar/chirpstack-fuota-server/internal/test"
)

type StorageTestSuite struct {
	suite.Suite
	tx *sqlx.Tx
}

func (s *StorageTestSuite) SetupSuite() {
	conf := test.GetConfig()
	if err := Setup(&conf); err != nil {
		panic(err)
	}
}

func (s *StorageTestSuite) SetupTest() {
	tx, err := DB().Beginx()
	if err != nil {
		panic(err)
	}
	s.tx = tx

	if err := MigrateDown(DB()); err != nil {
		panic(err)
	}
	if err := MigrateUp(DB()); err != nil {
		panic(err)
	}
}

func (s *StorageTestSuite) TearDownTest() {
	if err := s.tx.Rollback(); err != nil {
		panic(err)
	}
}

func (s *StorageTestSuite) Tx() sqlx.Ext {
	return s.tx
}

func TestStorage(t *testing.T) {
	suite.Run(t, new(StorageTestSuite))
}
