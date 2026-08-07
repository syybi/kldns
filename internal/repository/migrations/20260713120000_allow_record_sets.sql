CREATE TABLE records_new (
  id INTEGER PRIMARY KEY,
  uid INTEGER NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
  did INTEGER NOT NULL REFERENCES domains(id) ON UPDATE CASCADE ON DELETE CASCADE,
  subdomain_id INTEGER REFERENCES subdomains(id) ON UPDATE CASCADE ON DELETE SET NULL,
  record_id TEXT NOT NULL,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  value TEXT NOT NULL,
  line_id TEXT NOT NULL DEFAULT '0',
  line TEXT,
  created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
  updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
  UNIQUE(did, name, type, value, line_id)
);

INSERT INTO records_new(id, uid, did, subdomain_id, record_id, name, type, value, line_id, line, created_at, updated_at)
SELECT id, uid, did, subdomain_id, record_id, name, type, value, line_id, line, created_at, updated_at
FROM records;

DROP TABLE records;
ALTER TABLE records_new RENAME TO records;

CREATE INDEX idx_records_uid ON records(uid);
CREATE INDEX idx_records_record_id ON records(record_id);
CREATE INDEX idx_records_did_type ON records(did, type);
CREATE INDEX idx_records_subdomain_id ON records(subdomain_id);
CREATE INDEX idx_records_did_name_type ON records(did, name, type);
CREATE INDEX idx_records_did_record_id ON records(did, record_id);

-- +kldns Down

CREATE TABLE records_old (
  id INTEGER PRIMARY KEY,
  uid INTEGER NOT NULL REFERENCES users(id) ON UPDATE CASCADE ON DELETE CASCADE,
  did INTEGER NOT NULL REFERENCES domains(id) ON UPDATE CASCADE ON DELETE CASCADE,
  subdomain_id INTEGER REFERENCES subdomains(id) ON UPDATE CASCADE ON DELETE SET NULL,
  record_id TEXT NOT NULL,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  value TEXT NOT NULL,
  line_id TEXT NOT NULL DEFAULT '0',
  line TEXT,
  created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
  updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
  UNIQUE(did, name, type)
);

INSERT INTO records_old(id, uid, did, subdomain_id, record_id, name, type, value, line_id, line, created_at, updated_at)
SELECT id, uid, did, subdomain_id, record_id, name, type, value, line_id, line, created_at, updated_at
FROM records;

DROP TABLE records;
ALTER TABLE records_old RENAME TO records;
