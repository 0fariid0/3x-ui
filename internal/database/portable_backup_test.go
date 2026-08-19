package database

import (
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestCompactPortableSQLiteBackupDropsForkOnlyTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "portable-test.db")
	gdb, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}

	for _, mdl := range portableBackupModels() {
		if err := gdb.AutoMigrate(mdl); err != nil {
			t.Fatalf("AutoMigrate(%T): %v", mdl, err)
		}
	}
	if err := gdb.Exec(`CREATE TABLE fork_only_history (id INTEGER PRIMARY KEY, payload TEXT NOT NULL)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := gdb.Exec(`INSERT INTO fork_only_history(payload) VALUES ('x')`).Error; err != nil {
		t.Fatal(err)
	}
	if sqlDB, err := gdb.DB(); err == nil {
		_ = sqlDB.Close()
	}

	if err := compactPortableSQLiteBackup(path); err != nil {
		t.Fatalf("compactPortableSQLiteBackup: %v", err)
	}
	if err := ValidateSQLiteDB(path); err != nil {
		t.Fatalf("ValidateSQLiteDB: %v", err)
	}

	check, err := gorm.Open(sqlite.Open(path), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	var customCount int64
	if err := check.Raw(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='fork_only_history'`).Scan(&customCount).Error; err != nil {
		t.Fatal(err)
	}
	if customCount != 0 {
		t.Fatalf("fork_only_history still exists")
	}
	keep, err := portableBackupTableSet(check)
	if err != nil {
		t.Fatal(err)
	}
	for table := range keep {
		var count int64
		if err := check.Raw(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("required table %q missing after compact", table)
		}
	}
}
