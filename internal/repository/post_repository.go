package repository

import (
	"context"
	"database/sql"
	"livoir-blog/internal/domain"
	"livoir-blog/pkg/common"
	"livoir-blog/pkg/logger"
	"livoir-blog/pkg/ulid"
	"net/http"

	"go.uber.org/zap"
)

type postRepository struct {
	db *sql.DB
}

func NewPostRepository(db *sql.DB) (domain.PostRepository, error) {
	if db == nil {
		return nil, common.NewCustomError(http.StatusInternalServerError, "db is nil")
	}
	return &postRepository{db}, nil
}

func (r *postRepository) GetByID(ctx context.Context, id string) (*domain.PostDetail, error) {
	query := `SELECT p.id, p.current_version_id, p.created_at, p.updated_at, pv.title, pv.content, pv.version_number, c.id, c.name FROM posts p JOIN post_versions pv ON p.id = pv.post_id LEFT JOIN post_version_categories pvc ON pv.id = pvc.post_version_id LEFT JOIN categories c ON pvc.category_id = c.id WHERE p.id = $1 ORDER BY pv.version_number DESC LIMIT 1`
	rows, err := r.db.QueryContext(ctx, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, common.ErrPostNotFound
		}
		logger.Log.Error("Failed to get post by id", zap.Error(err))
		return nil, common.ErrInternalServerError
	}
	var categories []domain.Category
	var post domain.PostDetail
	defer rows.Close()
	for rows.Next() {
		var category domain.Category
		var categoryID sql.NullString
		var categoryName sql.NullString
		err := rows.Scan(&post.ID, &post.CurrentVersionID, &post.CreatedAt, &post.UpdatedAt, &post.Title, &post.Content, &post.VersionNumber, &categoryID, &categoryName)
		if err != nil {
			logger.Log.Error("Failed to scan post", zap.Error(err))
			return nil, common.ErrInternalServerError
		}
		if categoryID.Valid {
			category.ID = categoryID.String
		}
		if categoryName.Valid {
			category.Name = categoryName.String
		}
		if category.ID != "" {
			categories = append(categories, category)
		}
	}
	if post.ID == "" {
		return nil, common.ErrPostNotFound
	}
	post.Categories = categories
	return &post, nil
}

func (r *postRepository) Create(ctx context.Context, tx domain.Transaction, post *domain.Post) error {
	post.ID = ulid.New()
	sqlTx := tx.GetTx()
	query := `INSERT INTO posts (id, created_at, updated_at) VALUES ($1, $2, $3)`
	result, err := sqlTx.ExecContext(ctx, query, post.ID, post.CreatedAt, post.UpdatedAt)
	if err != nil {
		logger.Log.Error("Failed to create post", zap.Error(err))
		return common.ErrInternalServerError
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logger.Log.Error("Failed to get rows affected", zap.Error(err))
		return common.ErrInternalServerError
	}
	if rowsAffected == 0 {
		return common.NewCustomError(http.StatusInternalServerError, "failed to create post")
	}
	return nil
}

func (r *postRepository) Update(ctx context.Context, tx domain.Transaction, post *domain.Post) error {
	sqlTx := tx.GetTx()
	query := `UPDATE posts SET current_version_id = $1, updated_at = $2 WHERE id = $3`
	result, err := sqlTx.ExecContext(ctx, query, post.CurrentVersionID, post.UpdatedAt, post.ID)
	if err != nil {
		logger.Log.Error("Failed to update post", zap.Error(err))
		return common.ErrInternalServerError
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		logger.Log.Error("Failed to get rows affected", zap.Error(err))
		return common.ErrInternalServerError
	}
	if rowsAffected == 0 {
		return common.NewCustomError(http.StatusInternalServerError, "failed to update post")
	}
	return nil
}

func (r *postRepository) GetByIDForUpdate(ctx context.Context, tx domain.Transaction, id string) (*domain.Post, error) {
	sqlTx := tx.GetTx()
	post := &domain.Post{}
	err := sqlTx.QueryRowContext(ctx, "SELECT id, current_version_id, created_at, updated_at FROM posts WHERE id = $1 FOR UPDATE", id).
		Scan(&post.ID, &post.CurrentVersionID, &post.CreatedAt, &post.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			logger.Log.Error("No post versions found for post id", zap.String("id", id))
			return nil, common.ErrPostNotFound
		}
		logger.Log.Error("Failed to get latest post by id for update", zap.Error(err))
		return nil, common.ErrInternalServerError
	}
	return post, nil
}
