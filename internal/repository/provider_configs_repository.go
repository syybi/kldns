package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// ErrProviderConfigInUse is returned when a config still has domains attached.
var ErrProviderConfigInUse = errors.New("provider config in use")

// ProviderConfigsRepository manages reusable DNS platform credential profiles.
type ProviderConfigsRepository struct {
	DB *Database
}

func NewProviderConfigsRepository(db *Database) *ProviderConfigsRepository {
	return &ProviderConfigsRepository{DB: db}
}

type ProviderConfigSummary struct {
	ID           int64  `json:"id"`
	ProviderKey  string `json:"provider_key"`
	Name         string `json:"name"`
	ConfigStored bool   `json:"config_stored"`
	DomainCount  int64  `json:"domain_count"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

type ProviderConfigFilter struct {
	ProviderKey string
	Keyword     string
}

type ProviderConfigWrite struct {
	ID                     int64             `json:"id"`
	ProviderKey            string            `json:"provider_key"`
	Name                   string            `json:"name"`
	Config                 map[string]string `json:"config"`
	ConfigCiphertext       string            `json:"-"`
	KeepExistingCiphertext bool              `json:"-"`
}

type ProviderConfigRecord struct {
	ID               int64
	ProviderKey      string
	Name             string
	ConfigCiphertext string
}

func (r *ProviderConfigsRepository) applyFilter(base string, filter ProviderConfigFilter) (string, []any) {
	args := []any{}
	if filter.ProviderKey != "" {
		base += ` AND pc.provider_key = ?`
		args = append(args, filter.ProviderKey)
	}
	if term := likeTerm(filter.Keyword); term != "" {
		base += ` AND (lower(pc.name) LIKE ? OR lower(pc.provider_key) LIKE ?)`
		args = append(args, term, term)
	}
	return base, args
}

func (r *ProviderConfigsRepository) List(ctx context.Context, filter ProviderConfigFilter) ([]ProviderConfigSummary, error) {
	result, err := r.ListPage(ctx, filter, PageQuery{})
	return result.Items, err
}

func (r *ProviderConfigsRepository) ListPage(ctx context.Context, filter ProviderConfigFilter, page PageQuery) (PageResult[ProviderConfigSummary], error) {
	countFrom, countArgs := r.applyFilter(`FROM provider_configs pc WHERE 1 = 1`, filter)
	total := int64(0)
	if page.Enabled() {
		var err error
		total, err = countRows(ctx, r.DB, countFrom, countArgs)
		if err != nil {
			return PageResult[ProviderConfigSummary]{}, err
		}
	}

	fromWhere, args := r.applyFilter(`FROM provider_configs pc
		LEFT JOIN domains d ON d.provider_config_id = pc.id
		WHERE 1 = 1`, filter)
	query := `SELECT pc.id, pc.provider_key, pc.name, COALESCE(pc.config_ciphertext, '') != '', COUNT(d.id), pc.created_at, pc.updated_at ` +
		fromWhere + ` GROUP BY pc.id ORDER BY pc.id DESC`
	if page.Enabled() {
		page = page.Normalize()
		query, args = applyPage(query, args, page)
	}
	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return PageResult[ProviderConfigSummary]{}, err
	}
	defer rows.Close()
	items := []ProviderConfigSummary{}
	for rows.Next() {
		var item ProviderConfigSummary
		if err := rows.Scan(&item.ID, &item.ProviderKey, &item.Name, &item.ConfigStored, &item.DomainCount, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return PageResult[ProviderConfigSummary]{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return PageResult[ProviderConfigSummary]{}, err
	}
	if !page.Enabled() {
		total = int64(len(items))
		page = PageQuery{Page: 1, PageSize: len(items)}
	}
	return PageResult[ProviderConfigSummary]{Items: items, Total: total, Page: page.Page, PageSize: page.PageSize}, nil
}

func (r *ProviderConfigsRepository) Get(ctx context.Context, id int64) (ProviderConfigRecord, error) {
	var item ProviderConfigRecord
	err := r.DB.QueryRowContext(ctx, `SELECT id, provider_key, name, COALESCE(config_ciphertext, '') FROM provider_configs WHERE id = ?`, id).
		Scan(&item.ID, &item.ProviderKey, &item.Name, &item.ConfigCiphertext)
	return item, err
}

func (r *ProviderConfigsRepository) FindByName(ctx context.Context, providerKey string, name string, ignoreID int64) (ProviderConfigRecord, bool, error) {
	var item ProviderConfigRecord
	err := r.DB.QueryRowContext(ctx, `SELECT id, provider_key, name, COALESCE(config_ciphertext, '') FROM provider_configs
		WHERE provider_key = ? AND name = ? AND id != ? LIMIT 1`, providerKey, name, ignoreID).
		Scan(&item.ID, &item.ProviderKey, &item.Name, &item.ConfigCiphertext)
	if err == sql.ErrNoRows {
		return ProviderConfigRecord{}, false, nil
	}
	if err != nil {
		return ProviderConfigRecord{}, false, err
	}
	return item, true, nil
}

func (r *ProviderConfigsRepository) Upsert(ctx context.Context, input ProviderConfigWrite) (int64, error) {
	input.ProviderKey = strings.TrimSpace(input.ProviderKey)
	input.Name = strings.TrimSpace(input.Name)
	ciphertext := input.ConfigCiphertext
	if input.KeepExistingCiphertext && input.ID > 0 {
		existing, err := r.Get(ctx, input.ID)
		if err != nil {
			return 0, err
		}
		ciphertext = existing.ConfigCiphertext
	}
	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO dns_providers(key, config_ciphertext, created_at, updated_at)
		VALUES (?, '', strftime('%s','now'), strftime('%s','now'))
		ON CONFLICT(key) DO NOTHING`, input.ProviderKey); err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	if input.ID > 0 {
		_, err := tx.ExecContext(ctx, `UPDATE provider_configs
			SET provider_key = ?, name = ?, config_ciphertext = ?, updated_at = strftime('%s','now')
			WHERE id = ?`, input.ProviderKey, input.Name, ciphertext, input.ID)
		if err != nil {
			_ = tx.Rollback()
			return 0, err
		}
		return input.ID, tx.Commit()
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO provider_configs(provider_key, name, config_ciphertext)
		VALUES (?, ?, ?)`, input.ProviderKey, input.Name, ciphertext)
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	return id, tx.Commit()
}

func (r *ProviderConfigsRepository) Delete(ctx context.Context, id int64) (bool, error) {
	count, err := r.DomainCount(ctx, id)
	if err != nil {
		return false, err
	}
	if count > 0 {
		return false, ErrProviderConfigInUse
	}
	res, err := r.DB.ExecContext(ctx, `DELETE FROM provider_configs WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}

func (r *ProviderConfigsRepository) DomainCount(ctx context.Context, id int64) (int64, error) {
	var count int64
	err := r.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM domains WHERE provider_config_id = ?`, id).Scan(&count)
	return count, err
}

func (r *ProviderConfigsRepository) StoredProviders(ctx context.Context) (map[string]bool, error) {
	rows, err := r.DB.QueryContext(ctx, `SELECT provider_key FROM provider_configs WHERE COALESCE(config_ciphertext, '') != '' GROUP BY provider_key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := map[string]bool{}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		items[key] = true
	}
	return items, rows.Err()
}
