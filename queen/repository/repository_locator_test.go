// SPDX-License-Identifier: AGPL-3.0-or-later

package repository

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"plexobject.com/formicary/queen/config"
)

// TestSQLiteWALPragmas verifies that NewLocator opens a SQLite database with
// WAL journal mode enabled.  We query the live PRAGMA value so this test
// catches regressions in the DSN-building logic without relying on string
// matching against the DSN itself.
func TestSQLiteWALPragmas(t *testing.T) {
	serverCfg := config.TestServerConfig()
	serverCfg.DB.Type = "sqlite"
	serverCfg.DB.DataSource = fmt.Sprintf("/tmp/formicary_wal_test_%d.sqlite", rand.Int())
	require.NoError(t, serverCfg.Validate())

	locator, err := NewLocator(serverCfg)
	require.NoError(t, err)
	require.NotNil(t, locator)

	sqlDB, err := locator.DB.DB()
	require.NoError(t, err)

	var journalMode string
	row := sqlDB.QueryRow("PRAGMA journal_mode")
	require.NoError(t, row.Scan(&journalMode))
	require.Equal(t, "wal", journalMode,
		"expected WAL journal mode; check sqliteDSN() in repository_locator.go")
}
