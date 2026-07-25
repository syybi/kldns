-- Extract DNS platform credentials into reusable provider_configs.
-- Existing domain-level configs are migrated and domains point at a config id.

CREATE TABLE provider_configs (
  id INTEGER PRIMARY KEY,
  provider_key TEXT NOT NULL REFERENCES dns_providers(key) ON UPDATE CASCADE ON DELETE RESTRICT,
  name TEXT NOT NULL,
  config_ciphertext TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
  updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
  UNIQUE(provider_key, name)
);
CREATE INDEX idx_provider_configs_provider_key ON provider_configs(provider_key);

-- Ensure every domain provider key exists in the registry before inserts.
INSERT INTO dns_providers(key, config_ciphertext, created_at, updated_at)
SELECT DISTINCT provider_key, '', strftime('%s','now'), strftime('%s','now')
FROM domains
WHERE COALESCE(provider_key, '') != ''
  AND provider_key NOT IN (SELECT key FROM dns_providers);

-- One shared config per identical (provider_key, ciphertext) pair.
INSERT INTO provider_configs(provider_key, name, config_ciphertext, created_at, updated_at)
SELECT
  d.provider_key,
  CASE
    WHEN COUNT(*) = 1 THEN MIN(d.domain)
    ELSE MIN(d.domain) || ' 等' || COUNT(*) || '个主域'
  END,
  COALESCE(d.provider_config_ciphertext, ''),
  strftime('%s','now'),
  strftime('%s','now')
FROM domains d
GROUP BY d.provider_key, COALESCE(d.provider_config_ciphertext, '');

CREATE TABLE domains_new (
  id INTEGER PRIMARY KEY,
  provider_key TEXT NOT NULL REFERENCES dns_providers(key) ON UPDATE CASCADE ON DELETE RESTRICT,
  provider_config_id INTEGER NOT NULL REFERENCES provider_configs(id) ON UPDATE CASCADE ON DELETE RESTRICT,
  remote_zone_id TEXT NOT NULL,
  domain TEXT NOT NULL,
  group_policy TEXT NOT NULL DEFAULT '0',
  record_types TEXT NOT NULL DEFAULT 'A,CNAME',
  beian INTEGER NOT NULL DEFAULT 0 CHECK (beian IN (0, 1)),
  points_cost INTEGER NOT NULL DEFAULT 0 CHECK (points_cost >= 0),
  require_review INTEGER NOT NULL DEFAULT 0 CHECK (require_review IN (0, 1)),
  description TEXT,
  created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
  updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
  UNIQUE(provider_key, remote_zone_id),
  UNIQUE(domain)
);

INSERT INTO domains_new(
  id, provider_key, provider_config_id, remote_zone_id, domain, group_policy,
  record_types, beian, points_cost, require_review, description, created_at, updated_at
)
SELECT
  d.id,
  d.provider_key,
  pc.id,
  d.remote_zone_id,
  d.domain,
  d.group_policy,
  d.record_types,
  d.beian,
  d.points_cost,
  COALESCE(d.require_review, 0),
  d.description,
  d.created_at,
  d.updated_at
FROM domains d
JOIN provider_configs pc
  ON pc.provider_key = d.provider_key
 AND pc.config_ciphertext = COALESCE(d.provider_config_ciphertext, '');

DROP TABLE domains;
ALTER TABLE domains_new RENAME TO domains;

CREATE INDEX idx_domains_provider_key ON domains(provider_key);
CREATE INDEX idx_domains_provider_config_id ON domains(provider_config_id);

-- +kldns Down

CREATE TABLE domains_old (
  id INTEGER PRIMARY KEY,
  provider_key TEXT NOT NULL REFERENCES dns_providers(key) ON UPDATE CASCADE ON DELETE RESTRICT,
  provider_config_ciphertext TEXT,
  remote_zone_id TEXT NOT NULL,
  domain TEXT NOT NULL,
  group_policy TEXT NOT NULL DEFAULT '0',
  record_types TEXT NOT NULL DEFAULT 'A,CNAME',
  beian INTEGER NOT NULL DEFAULT 0 CHECK (beian IN (0, 1)),
  points_cost INTEGER NOT NULL DEFAULT 0 CHECK (points_cost >= 0),
  require_review INTEGER NOT NULL DEFAULT 0 CHECK (require_review IN (0, 1)),
  description TEXT,
  created_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
  updated_at INTEGER NOT NULL DEFAULT (strftime('%s','now')),
  UNIQUE(provider_key, remote_zone_id),
  UNIQUE(domain)
);

INSERT INTO domains_old(
  id, provider_key, provider_config_ciphertext, remote_zone_id, domain, group_policy,
  record_types, beian, points_cost, require_review, description, created_at, updated_at
)
SELECT
  d.id,
  d.provider_key,
  pc.config_ciphertext,
  d.remote_zone_id,
  d.domain,
  d.group_policy,
  d.record_types,
  d.beian,
  d.points_cost,
  d.require_review,
  d.description,
  d.created_at,
  d.updated_at
FROM domains d
JOIN provider_configs pc ON pc.id = d.provider_config_id;

DROP TABLE domains;
ALTER TABLE domains_old RENAME TO domains;
CREATE INDEX idx_domains_provider_key ON domains(provider_key);
DROP TABLE provider_configs;
