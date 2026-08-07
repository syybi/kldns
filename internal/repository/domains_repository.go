package repository

import (
	"context"
	"database/sql"
	"strings"

	"kldns/pkg/utils/validation"
)

// DomainsRepository handles domain admin data access.
type DomainsRepository struct {
	DB *Database
}

func NewDomainsRepository(db *Database) *DomainsRepository {
	return &DomainsRepository{DB: db}
}

type DomainAdminSummary struct {
	ID                 int64  `json:"id"`
	ProviderKey        string `json:"provider_key"`
	ProviderConfigID   int64  `json:"provider_config_id"`
	ProviderConfigName string `json:"provider_config_name"`
	RemoteZoneID       string `json:"remote_zone_id"`
	Domain             string `json:"domain"`
	GroupPolicy        string `json:"group_policy"`
	RecordTypes        string `json:"record_types"`
	Beian              int    `json:"beian"`
	PointsCost         int64  `json:"points_cost"`
	RequireReview      int    `json:"require_review"`
	Description        string `json:"description"`
}

type DomainAdminFilter struct {
	ProviderKey string
	Keyword     string
}

type DomainWrite struct {
	ID               int64    `json:"id"`
	ProviderKey      string   `json:"provider_key"`
	ProviderConfigID int64    `json:"provider_config_id"`
	RemoteZoneID     string   `json:"remote_zone_id"`
	Domain           string   `json:"domain"`
	GroupPolicy      string   `json:"group_policy"`
	RecordTypes      []string `json:"record_types"`
	Beian            int      `json:"beian"`
	PointsCost       int64    `json:"points_cost"`
	RequireReview    int      `json:"require_review"`
	Description      string   `json:"description"`
}

type DomainConflict struct {
	ID           int64
	ProviderKey  string
	RemoteZoneID string
	Domain       string
}

func (r *DomainsRepository) ListDomains(ctx context.Context, filter DomainAdminFilter) ([]DomainAdminSummary, error) {
	result, err := r.ListDomainsPage(ctx, filter, PageQuery{})
	return result.Items, err
}

func (r *DomainsRepository) ListDomainsPage(ctx context.Context, filter DomainAdminFilter, page PageQuery) (PageResult[DomainAdminSummary], error) {
	fromWhere := `FROM domains d
		JOIN provider_configs pc ON pc.id = d.provider_config_id
		WHERE 1 = 1`
	args := []any{}
	if filter.ProviderKey != "" {
		fromWhere += ` AND d.provider_key = ?`
		args = append(args, filter.ProviderKey)
	}
	if term := likeTerm(filter.Keyword); term != "" {
		fromWhere += ` AND lower(d.domain) LIKE ?`
		args = append(args, term)
	}
	total := int64(0)
	if page.Enabled() {
		var err error
		total, err = countRows(ctx, r.DB, fromWhere, args)
		if err != nil {
			return PageResult[DomainAdminSummary]{}, err
		}
	}
	query := `SELECT d.id, d.provider_key, d.provider_config_id, pc.name, d.remote_zone_id, d.domain, d.group_policy, d.record_types, d.beian, d.points_cost, d.require_review, COALESCE(d.description, '') ` + fromWhere
	query += ` ORDER BY d.id DESC`
	if page.Enabled() {
		page = page.Normalize()
		query, args = applyPage(query, args, page)
	}
	rows, err := r.DB.QueryContext(ctx, query, args...)
	if err != nil {
		return PageResult[DomainAdminSummary]{}, err
	}
	defer rows.Close()
	items := []DomainAdminSummary{}
	for rows.Next() {
		var item DomainAdminSummary
		if err := rows.Scan(&item.ID, &item.ProviderKey, &item.ProviderConfigID, &item.ProviderConfigName, &item.RemoteZoneID, &item.Domain, &item.GroupPolicy, &item.RecordTypes, &item.Beian, &item.PointsCost, &item.RequireReview, &item.Description); err != nil {
			return PageResult[DomainAdminSummary]{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return PageResult[DomainAdminSummary]{}, err
	}
	if !page.Enabled() {
		total = int64(len(items))
		page = PageQuery{Page: 1, PageSize: len(items)}
	}
	return PageResult[DomainAdminSummary]{Items: items, Total: total, Page: page.Page, PageSize: page.PageSize}, nil
}

func (r *DomainsRepository) FindDomainConflict(ctx context.Context, input DomainWrite) (DomainConflict, bool, error) {
	query := `SELECT id, provider_key, remote_zone_id, domain FROM domains
		WHERE id != ? AND (domain = ? OR (provider_key = ? AND remote_zone_id = ?))
		LIMIT 1`
	var conflict DomainConflict
	err := r.DB.QueryRowContext(ctx, query, input.ID, input.Domain, input.ProviderKey, input.RemoteZoneID).
		Scan(&conflict.ID, &conflict.ProviderKey, &conflict.RemoteZoneID, &conflict.Domain)
	if err == sql.ErrNoRows {
		return DomainConflict{}, false, nil
	}
	if err != nil {
		return DomainConflict{}, false, err
	}
	return conflict, true, nil
}

func (r *DomainsRepository) UpsertDomain(ctx context.Context, input DomainWrite) (int64, error) {
	recordTypes := strings.Join(validation.NormalizeRecordTypes(input.RecordTypes), ",")
	if input.GroupPolicy == "" {
		input.GroupPolicy = "0"
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
		_, err := tx.ExecContext(ctx, `UPDATE domains
			SET provider_key = ?, provider_config_id = ?, remote_zone_id = ?, domain = ?, group_policy = ?, record_types = ?, beian = ?, points_cost = ?, require_review = ?, description = ?, updated_at = strftime('%s','now')
			WHERE id = ?`,
			input.ProviderKey, input.ProviderConfigID, input.RemoteZoneID, input.Domain, input.GroupPolicy, recordTypes, boolInt(input.Beian), maxInt64(input.PointsCost, 0), boolInt(input.RequireReview), input.Description, input.ID)
		if err != nil {
			_ = tx.Rollback()
			return 0, err
		}
		return input.ID, tx.Commit()
	}
	res, err := tx.ExecContext(ctx, `INSERT INTO domains(provider_key, provider_config_id, remote_zone_id, domain, group_policy, record_types, beian, points_cost, require_review, description)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		input.ProviderKey, input.ProviderConfigID, input.RemoteZoneID, input.Domain, input.GroupPolicy, recordTypes, boolInt(input.Beian), maxInt64(input.PointsCost, 0), boolInt(input.RequireReview), input.Description)
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

func (r *DomainsRepository) DeleteDomain(ctx context.Context, id int64) (bool, error) {
	res, err := r.DB.ExecContext(ctx, `DELETE FROM domains WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}
