package repositories

import (
	"context"
	"database/sql"
	migrationassets "kldns/migrations"
	"path/filepath"
	"testing"
)

func TestInitialMigrationEnablesConstraints(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kldns.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := ConfigureSQLite(db, 1000, false); err != nil {
		t.Fatal(err)
	}
	if err := RunMigrations(db, "../migrations"); err != nil {
		t.Fatal(err)
	}

	var foreignKeys int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, want 1", foreignKeys)
	}

	_, err = db.Exec(`INSERT INTO records(uid, did, record_id, name, type, value, line_id, line)
		VALUES (999, 999, 'r1', 'www', 'A', '1.1.1.1', '0', '默认')`)
	if err == nil {
		t.Fatal("expected foreign key failure for orphan record")
	}

	var systemUserID int64
	if err := db.QueryRow(`SELECT id FROM users WHERE id = 0 AND username = 'system-sync'`).Scan(&systemUserID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO dns_providers(key, config_ciphertext) VALUES ('fake', '{}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO provider_configs(id, provider_key, name, config_ciphertext) VALUES (1, 'fake', 'fake-main', 'cipher')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO domains(id, provider_key, provider_config_id, remote_zone_id, domain, group_policy, record_types) VALUES (1, 'fake', 1, 'z1', 'example.com', '0', 'A')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO records(uid, did, record_id, name, type, value, line_id, line)
		VALUES (0, 1, 'remote-1', 'www', 'A', '1.1.1.1', '0', '默认')`); err != nil {
		t.Fatalf("system user should own synced records: %v", err)
	}
}

func TestEmbeddedMigrationsInitializeSchema(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kldns.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := ConfigureSQLite(db, 1000, false); err != nil {
		t.Fatal(err)
	}
	if err := RunMigrationsFS(db, migrationassets.FS, migrationassets.Dir); err != nil {
		t.Fatal(err)
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(1) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("embedded migrations did not apply")
	}
}

func TestProviderConfigMigrationMovesDomainCredentials(t *testing.T) {
	db, err := OpenSQLite(filepath.Join(t.TempDir(), "kldns.db"), 1000, false)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := RunMigrations(db.SQLDB(), "../migrations"); err != nil {
		t.Fatal(err)
	}
	var hasColumn int
	if err := db.QueryRow(`SELECT COUNT(1) FROM pragma_table_info('domains') WHERE name = 'provider_config_id'`).Scan(&hasColumn); err != nil {
		t.Fatal(err)
	}
	if hasColumn != 1 {
		t.Fatal("domains.provider_config_id should exist after migration")
	}
	var legacyColumn int
	if err := db.QueryRow(`SELECT COUNT(1) FROM pragma_table_info('domains') WHERE name = 'provider_config_ciphertext'`).Scan(&legacyColumn); err != nil {
		t.Fatal(err)
	}
	if legacyColumn != 0 {
		t.Fatal("domains.provider_config_ciphertext should be removed after migration")
	}
	if _, err := db.Exec(`INSERT INTO dns_providers(key, config_ciphertext) VALUES ('fake', '')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO provider_configs(id, provider_key, name, config_ciphertext) VALUES (10, 'fake', 'acct-a', 'cipher-a')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO provider_configs(id, provider_key, name, config_ciphertext) VALUES (11, 'fake', 'acct-b', 'cipher-b')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO domains(provider_key, provider_config_id, remote_zone_id, domain, group_policy, record_types)
		VALUES ('fake', 10, 'z1', 'a.example', '0', 'A'), ('fake', 11, 'z2', 'b.example', '0', 'A')`); err != nil {
		t.Fatal(err)
	}
	items, err := NewProviderConfigsRepository(db).List(context.Background(), ProviderConfigFilter{ProviderKey: "fake"})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("provider configs = %#v, want 2", items)
	}
}

func TestRecordSetMigrationAllowsMultipleValuesPerNameAndType(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kldns.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := ConfigureSQLite(db, 1000, false); err != nil {
		t.Fatal(err)
	}
	if err := RunMigrations(db, "../migrations"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO dns_providers(key, config_ciphertext) VALUES ('fake', '{}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO provider_configs(id, provider_key, name, config_ciphertext) VALUES (1, 'fake', 'fake-main', 'cipher')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO domains(id, provider_key, provider_config_id, remote_zone_id, domain, group_policy, record_types) VALUES (1, 'fake', 1, 'z1', 'example.com', '0', 'A,TXT')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO users(id, group_id, status, username, password_hash, sid, points) VALUES (1, 100, 2, 'alice', 'hash', 'alice', 10)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO records(uid, did, record_id, name, type, value, line_id) VALUES
		(1, 1, 'remote-1', 'www', 'A', '192.0.2.1', '0'),
		(1, 1, 'remote-2', 'www', 'A', '192.0.2.2', '0')`); err != nil {
		t.Fatalf("multiple values in one RRset should be allowed: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO records(uid, did, record_id, name, type, value, line_id)
		VALUES (1, 1, 'remote-3', 'www', 'A', '192.0.2.1', '0')`); err == nil {
		t.Fatal("an exact duplicate record should remain unique")
	}
}

func TestRejectedSubdomainHistoryDoesNotOccupyName(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "kldns.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := ConfigureSQLite(db, 1000, false); err != nil {
		t.Fatal(err)
	}
	if err := RunMigrations(db, "../migrations"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO dns_providers(key, config_ciphertext) VALUES ('fake', '{}')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO provider_configs(id, provider_key, name, config_ciphertext) VALUES (1, 'fake', 'fake-main', 'cipher')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO domains(id, provider_key, provider_config_id, remote_zone_id, domain, group_policy, record_types) VALUES (1, 'fake', 1, 'z1', 'example.com', '0', 'A')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO users(id, group_id, status, username, password_hash, sid, points) VALUES (1, 100, 2, 'alice', 'hash', 'alice', 10)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO subdomains(uid, did, name, full_domain, status, purpose, reject_reason)
		VALUES (1, 1, 'demo', 'demo.example.com', 3, '个人博客', '用途不合规')`); err != nil {
		t.Fatalf("insert rejected history: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO subdomains(uid, did, name, full_domain, status, purpose)
		VALUES (1, 1, 'demo', 'demo.example.com', 2, '重新申请')`); err != nil {
		t.Fatalf("rejected history should not block pending reapply: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO subdomains(uid, did, name, full_domain, status, purpose, reject_reason)
		VALUES (1, 1, 'demo', 'demo.example.com', 3, '再次历史', '仍不合规')`); err != nil {
		t.Fatalf("multiple rejected histories should be allowed: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO subdomains(uid, did, name, full_domain, status)
		VALUES (1, 1, 'demo', 'demo.example.com', 1)`); err == nil {
		t.Fatal("live subdomain names should remain unique")
	}
}
