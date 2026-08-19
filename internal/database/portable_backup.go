package database

import (
	"fmt"
	"os"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// portableBackupModels intentionally mirrors the database model list in the
// upstream MHSanaei/3x-ui source. Fork-only analytics/history tables remain in
// the live database but are excluded from portable panel backups.
//
// IMPORTANT: this list is used only against a temporary backup snapshot. The
// live database is never modified by BackupPortableSQLite.
func portableBackupModels() []any {
	return []any{
		&model.User{},
		&model.Inbound{},
		&model.OutboundTraffics{},
		&model.Setting{},
		&model.InboundClientIps{},
		&xray.ClientTraffic{},
		&model.HistoryOfSeeders{},
		&model.Node{},
		&model.ApiToken{},
		&model.ClientRecord{},
		&model.ClientInbound{},
		&model.ClientExternalLink{},
		&model.ClientGroup{},
		&model.InboundFallback{},
		&model.Host{},
		&model.NodeClientTraffic{},
		&model.NodeClientIp{},
		&model.ClientGlobalTraffic{},
		&model.OutboundSubscription{},
	}
}

// BackupPortableSQLite creates a consistent SQLite snapshot and removes only
// fork-specific tables from that disposable snapshot. Configuration, users,
// inbounds, clients, standard traffic state, settings, nodes, hosts, groups,
// tokens and other upstream models are retained exactly as in a normal backup.
func BackupPortableSQLite(dstPath string) error {
	if IsPostgres() {
		return fmt.Errorf("portable SQLite backup is unavailable for PostgreSQL")
	}

	if err := BackupSQLite(dstPath); err != nil {
		return err
	}

	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(dstPath)
			_ = os.Remove(dstPath + "-wal")
			_ = os.Remove(dstPath + "-shm")
		}
	}()

	if err := compactPortableSQLiteBackup(dstPath); err != nil {
		return err
	}
	if err := ValidateSQLiteDB(dstPath); err != nil {
		return fmt.Errorf("portable backup integrity check failed: %w", err)
	}

	ok = true
	return nil
}

func compactPortableSQLiteBackup(dbPath string) error {
	// The temporary copy must be self-contained. DELETE journal mode prevents a
	// downloaded backup from depending on WAL/SHM sidecars and lets VACUUM
	// rewrite the database into a compact single file.
	dsn := dbPath + "?_busy_timeout=10000&_journal_mode=DELETE"
	gdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return fmt.Errorf("open temporary backup: %w", err)
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		return fmt.Errorf("get temporary backup connection: %w", err)
	}
	closed := false
	defer func() {
		if !closed {
			_ = sqlDB.Close()
		}
	}()

	keep, err := portableBackupTableSet(gdb)
	if err != nil {
		return err
	}

	var tables []string
	if err := gdb.Raw(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name`).Scan(&tables).Error; err != nil {
		return fmt.Errorf("list backup tables: %w", err)
	}

	// Foreign keys are disabled only on the disposable snapshot. This allows a
	// fork-only reporting table that references an upstream table to be dropped
	// without touching or rewriting any live data.
	if err := gdb.Exec("PRAGMA foreign_keys=OFF").Error; err != nil {
		return fmt.Errorf("disable foreign keys on backup copy: %w", err)
	}

	for _, table := range tables {
		if _, isCore := keep[table]; isCore {
			continue
		}
		quoted := `"` + strings.ReplaceAll(table, `"`, `""`) + `"`
		if err := gdb.Exec("DROP TABLE IF EXISTS " + quoted).Error; err != nil {
			return fmt.Errorf("drop non-portable backup table %q: %w", table, err)
		}
	}

	// Refuse to return a backup if any upstream/core table disappeared. This is
	// intentionally strict: a small backup is never worth risking a broken one.
	for table := range keep {
		var count int64
		if err := gdb.Raw(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count).Error; err != nil {
			return fmt.Errorf("verify core table %q: %w", table, err)
		}
		if count != 1 {
			return fmt.Errorf("portable backup is missing required core table %q", table)
		}
	}

	// DROP TABLE releases pages to SQLite's freelist but does not shrink the
	// file. VACUUM rewrites only retained upstream data, removing the bloat that
	// made fork backups difficult to transfer.
	if err := gdb.Exec("VACUUM").Error; err != nil {
		return fmt.Errorf("vacuum portable backup: %w", err)
	}
	if err := gdb.Exec("PRAGMA optimize").Error; err != nil {
		return fmt.Errorf("optimize portable backup: %w", err)
	}

	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("close portable backup: %w", err)
	}
	closed = true

	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")
	return nil
}

func portableBackupTableSet(gdb *gorm.DB) (map[string]struct{}, error) {
	models := portableBackupModels()
	keep := make(map[string]struct{}, len(models))
	for _, mdl := range models {
		stmt := &gorm.Statement{DB: gdb}
		if err := stmt.Parse(mdl); err != nil {
			return nil, fmt.Errorf("resolve portable backup model %T: %w", mdl, err)
		}
		if stmt.Schema == nil || strings.TrimSpace(stmt.Schema.Table) == "" {
			return nil, fmt.Errorf("resolve portable backup model %T: empty table name", mdl)
		}
		keep[stmt.Schema.Table] = struct{}{}
	}
	return keep, nil
}
