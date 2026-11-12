package database

import (
	"github.com/BwCloudWeGo/bw-cli/pkg/config"
	"go.uber.org/zap"
	"os"
	"path/filepath"
	"testing"
)

func TestSQLiteAndInvalidConfig(t *testing.T) {
	p := filepath.Join(t.TempDir(), "nested", "db.sqlite")
	db, e := Open(config.DatabaseConfig{Driver: "sqlite", DSN: p}, config.MySQLConfig{}, config.PostgreSQLConfig{}, zap.NewNop())
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := db.DB()
	defer raw.Close()
	if e = db.Exec("CREATE TABLE example (id INTEGER)").Error; e != nil {
		t.Fatal(e)
	}
	if e = ensureDir("memory.db"); e != nil {
		t.Fatal(e)
	}
	f := filepath.Join(t.TempDir(), "file")
	if e = os.WriteFile(f, []byte("x"), 0600); e != nil {
		t.Fatal(e)
	}
	if e = ensureDir(filepath.Join(f, "nested", "db")); e == nil {
		t.Fatal("file parent")
	}
	for _, driver := range []string{"unknown", "mysql", "pg"} {
		if _, e = Open(config.DatabaseConfig{Driver: driver}, config.MySQLConfig{}, config.PostgreSQLConfig{}, zap.NewNop()); e == nil {
			t.Fatal(driver)
		}
	}
	if ToMySQLConfig(config.MySQLConfig{DSN: "m"}).DSN != "m" || ToPostgreSQLConfig(config.PostgreSQLConfig{DSN: "p"}).DSN != "p" {
		t.Fatal("mapping")
	}
}
