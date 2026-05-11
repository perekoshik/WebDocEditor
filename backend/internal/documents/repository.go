package documents

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("document not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, title, filename, storagePath string, isTemplate bool) (Document, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Document{}, err
	}
	defer tx.Rollback(ctx)

	var d Document
	err = tx.QueryRow(ctx, `
        insert into documents (title, filename, storage_path, is_template)
        values ($1, $2, $3, $4)
        returning id, title, filename, storage_path, version, is_template, created_at, updated_at
    `, title, filename, storagePath, isTemplate).
		Scan(&d.ID, &d.Title, &d.Filename, &d.StoragePath, &d.Version, &d.IsTemplate, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return Document{}, fmt.Errorf("insert document: %w", err)
	}

	if _, err := tx.Exec(ctx, `
        insert into document_versions (document_id, version, storage_path)
        values ($1, $2, $3)
    `, d.ID, d.Version, storagePath); err != nil {
		return Document{}, fmt.Errorf("insert version: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return Document{}, err
	}
	return d, nil
}

func (r *Repository) List(ctx context.Context, q string, isTemplate bool) ([]Document, error) {
	rows, err := r.pool.Query(ctx, `
        select id, title, filename, storage_path, version, is_template, created_at, updated_at
        from documents
        where is_template = $2 and ($1 = '' or title ilike '%' || $1 || '%')
        order by updated_at desc
    `, q, isTemplate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Document, 0)
	for rows.Next() {
		var d Document
		if err := rows.Scan(&d.ID, &d.Title, &d.Filename, &d.StoragePath, &d.Version, &d.IsTemplate, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (Document, error) {
	var d Document
	err := r.pool.QueryRow(ctx, `
        select id, title, filename, storage_path, version, is_template, created_at, updated_at
        from documents where id = $1
    `, id).Scan(&d.ID, &d.Title, &d.Filename, &d.StoragePath, &d.Version, &d.IsTemplate, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Document{}, ErrNotFound
	}
	if err != nil {
		return Document{}, err
	}
	return d, nil
}

func (r *Repository) AddVersion(ctx context.Context, id uuid.UUID, storagePath string) (Document, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Document{}, err
	}
	defer tx.Rollback(ctx)

	var d Document
	err = tx.QueryRow(ctx, `
        update documents
        set version = version + 1,
            storage_path = $2,
            updated_at = now()
        where id = $1
        returning id, title, filename, storage_path, version, is_template, created_at, updated_at
    `, id, storagePath).
		Scan(&d.ID, &d.Title, &d.Filename, &d.StoragePath, &d.Version, &d.IsTemplate, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Document{}, ErrNotFound
	}
	if err != nil {
		return Document{}, err
	}

	if _, err := tx.Exec(ctx, `
        insert into document_versions (document_id, version, storage_path)
        values ($1, $2, $3)
    `, d.ID, d.Version, storagePath); err != nil {
		return Document{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Document{}, err
	}
	return d, nil
}

func (r *Repository) GetVersionPath(ctx context.Context, id uuid.UUID, version int) (string, error) {
	var path string
	err := r.pool.QueryRow(ctx, `
        select storage_path from document_versions
        where document_id = $1 and version = $2
    `, id, version).Scan(&path)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return path, nil
}

func (r *Repository) Versions(ctx context.Context, id uuid.UUID) ([]Version, error) {
	rows, err := r.pool.Query(ctx, `
        select version, created_at
        from document_versions
        where document_id = $1
        order by version asc
    `, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Version, 0)
	for rows.Next() {
		var v Version
		if err := rows.Scan(&v.Version, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *Repository) UpdateTitle(ctx context.Context, id uuid.UUID, title string) (Document, error) {
	var d Document
	err := r.pool.QueryRow(ctx, `
        update documents set title = $2, updated_at = now()
        where id = $1
        returning id, title, filename, storage_path, version, is_template, created_at, updated_at
    `, id, title).Scan(&d.ID, &d.Title, &d.Filename, &d.StoragePath, &d.Version, &d.IsTemplate, &d.CreatedAt, &d.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Document{}, ErrNotFound
	}
	if err != nil {
		return Document{}, err
	}
	return d, nil
}

func (r *Repository) Delete(ctx context.Context, id uuid.UUID) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
        select storage_path from document_versions where document_id = $1
    `, id)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0)
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			rows.Close()
			return nil, err
		}
		paths = append(paths, p)
	}
	rows.Close()

	tag, err := r.pool.Exec(ctx, `delete from documents where id = $1`, id)
	if err != nil {
		return nil, err
	}
	if tag.RowsAffected() == 0 {
		return nil, ErrNotFound
	}
	return paths, nil
}
