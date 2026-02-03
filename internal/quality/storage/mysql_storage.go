package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github-hub/internal/quality/models"
	_ "github.com/go-sql-driver/mysql"
)

// MySQLStorage MySQL存储实现
type MySQLStorage struct {
	db *sql.DB
}

// NewMySQLStorage 创建新的MySQL存储
func NewMySQLStorage(dsn string) (*MySQLStorage, error) {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &MySQLStorage{db: db}, nil
}

// Close 关闭数据库连接
func (s *MySQLStorage) Close() error {
	return s.db.Close()
}

// CreateEvent 创建事件
func (s *MySQLStorage) CreateEvent(event *models.GitHubEvent) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.Exec(`
		INSERT INTO github_events (event_id, event_type, event_status, repository, branch, target_branch, commit_sha, pr_number, action, pusher, author, payload, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, event.EventID, event.EventType, event.EventStatus, event.Repository, event.Branch, event.TargetBranch, event.CommitSHA, event.PRNumber, event.Action, event.Pusher, event.Author, event.Payload, event.CreatedAt, event.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert event: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	event.ID = int(id)

	for i := range event.QualityChecks {
		event.QualityChecks[i].GitHubEventID = event.EventID
		if err := s.createQualityCheckInTx(tx, &event.QualityChecks[i]); err != nil {
			return fmt.Errorf("failed to create quality check: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetEvent 获取事件
func (s *MySQLStorage) GetEvent(id int) (*models.GitHubEvent, error) {
	var event models.GitHubEvent
	var targetBranch, commitSHA, action, pusher, author sql.NullString
	var prNumber sql.NullInt64
	var processedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, event_id, event_type, event_status, repository, branch, target_branch, commit_sha, pr_number, action, pusher, author, payload, created_at, updated_at, processed_at
		FROM github_events
		WHERE id = ?
	`, id).Scan(
		&event.ID, &event.EventID, &event.EventType, &event.EventStatus, &event.Repository, &event.Branch, &targetBranch, &commitSHA, &prNumber, &action, &pusher, &author, &event.Payload, &event.CreatedAt, &event.UpdatedAt, &processedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("event not found")
		}
		return nil, fmt.Errorf("failed to query event: %w", err)
	}

	if targetBranch.Valid {
		event.TargetBranch = &targetBranch.String
	}
	if commitSHA.Valid {
		event.CommitSHA = &commitSHA.String
	}
	if action.Valid {
		event.Action = &action.String
	}
	if pusher.Valid {
		event.Pusher = &pusher.String
	}
	if author.Valid {
		event.Author = &author.String
	}
	if prNumber.Valid {
		n := int(prNumber.Int64)
		event.PRNumber = &n
	}
	if processedAt.Valid {
		event.ProcessedAt = &processedAt.Time
	}

	checks, err := s.ListQualityChecksByEventID(event.EventID)
	if err != nil {
		event.QualityChecks = []models.PRQualityCheck{}
	} else {
		event.QualityChecks = checks
	}

	return &event, nil
}

// GetEventByEventID 根据EventID获取事件
func (s *MySQLStorage) GetEventByEventID(eventID string) (*models.GitHubEvent, error) {
	var event models.GitHubEvent
	var targetBranch, commitSHA, action, pusher, author sql.NullString
	var prNumber sql.NullInt64
	var processedAt sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, event_id, event_type, event_status, repository, branch, target_branch, commit_sha, pr_number, action, pusher, author, payload, created_at, updated_at, processed_at
		FROM github_events
		WHERE event_id = ?
	`, eventID).Scan(
		&event.ID, &event.EventID, &event.EventType, &event.EventStatus, &event.Repository, &event.Branch, &targetBranch, &commitSHA, &prNumber, &action, &pusher, &author, &event.Payload, &event.CreatedAt, &event.UpdatedAt, &processedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("event not found")
		}
		return nil, fmt.Errorf("failed to query event: %w", err)
	}

	if targetBranch.Valid {
		event.TargetBranch = &targetBranch.String
	}
	if commitSHA.Valid {
		event.CommitSHA = &commitSHA.String
	}
	if action.Valid {
		event.Action = &action.String
	}
	if pusher.Valid {
		event.Pusher = &pusher.String
	}
	if author.Valid {
		event.Author = &author.String
	}
	if prNumber.Valid {
		n := int(prNumber.Int64)
		event.PRNumber = &n
	}
	if processedAt.Valid {
		event.ProcessedAt = &processedAt.Time
	}

	checks, err := s.ListQualityChecksByEventID(event.EventID)
	if err != nil {
		event.QualityChecks = []models.PRQualityCheck{}
	} else {
		event.QualityChecks = checks
	}

	return &event, nil
}

// ListEvents 列出所有事件
func (s *MySQLStorage) ListEvents() ([]*models.GitHubEvent, error) {
	rows, err := s.db.Query(`
		SELECT id, event_id, event_type, event_status, repository, branch, target_branch, commit_sha, pr_number, action, pusher, author, payload, created_at, updated_at, processed_at
		FROM github_events
		ORDER BY id DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query events: %w", err)
	}
	defer rows.Close()

	var events []*models.GitHubEvent
	for rows.Next() {
		var event models.GitHubEvent
		var targetBranch, commitSHA, action, pusher, author sql.NullString
		var prNumber sql.NullInt64
		var processedAt sql.NullTime

		if err := rows.Scan(
			&event.ID, &event.EventID, &event.EventType, &event.EventStatus, &event.Repository, &event.Branch, &targetBranch, &commitSHA, &prNumber, &action, &pusher, &author, &event.Payload, &event.CreatedAt, &event.UpdatedAt, &processedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan event: %w", err)
		}

		if targetBranch.Valid {
			event.TargetBranch = &targetBranch.String
		}
		if commitSHA.Valid {
			event.CommitSHA = &commitSHA.String
		}
		if action.Valid {
			event.Action = &action.String
		}
		if pusher.Valid {
			event.Pusher = &pusher.String
		}
		if author.Valid {
			event.Author = &author.String
		}
		if prNumber.Valid {
			n := int(prNumber.Int64)
			event.PRNumber = &n
		}
		if processedAt.Valid {
			event.ProcessedAt = &processedAt.Time
		}

		checks, err := s.ListQualityChecksByEventID(event.EventID)
		if err != nil {
			event.QualityChecks = []models.PRQualityCheck{}
		} else {
			event.QualityChecks = checks
		}

		events = append(events, &event)
	}

	return events, nil
}

// UpdateEvent 更新事件
func (s *MySQLStorage) UpdateEvent(event *models.GitHubEvent) error {
	_, err := s.db.Exec(`
		UPDATE github_events
		SET event_status = ?, processed_at = ?, updated_at = ?
		WHERE id = ?
	`, event.EventStatus, event.ProcessedAt, event.UpdatedAt, event.ID)
	if err != nil {
		return fmt.Errorf("failed to update event: %w", err)
	}
	return nil
}

// DeleteEvent 删除事件
func (s *MySQLStorage) DeleteEvent(id int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec("DELETE FROM pr_quality_checks WHERE github_event_id = (SELECT event_id FROM github_events WHERE id = ?)", id)
	if err != nil {
		return fmt.Errorf("failed to delete quality checks: %w", err)
	}

	_, err = tx.Exec("DELETE FROM github_events WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// DeleteAllEvents 删除所有事件
func (s *MySQLStorage) DeleteAllEvents() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec("DELETE FROM pr_quality_checks")
	if err != nil {
		return fmt.Errorf("failed to delete quality checks: %w", err)
	}

	_, err = tx.Exec("DELETE FROM github_events")
	if err != nil {
		return fmt.Errorf("failed to delete events: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// CreateQualityCheck 创建质量检查
func (s *MySQLStorage) CreateQualityCheck(check *models.PRQualityCheck) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := s.createQualityCheckInTx(tx, check); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (s *MySQLStorage) createQualityCheckInTx(tx *sql.Tx, check *models.PRQualityCheck) error {
	result, err := tx.Exec(`
		INSERT INTO pr_quality_checks (github_event_id, check_type, check_status, stage, stage_order, check_order, started_at, completed_at, duration_seconds, error_message, output, retry_count, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, check.GitHubEventID, check.CheckType, check.CheckStatus, check.Stage, check.StageOrder, check.CheckOrder, check.StartedAt, check.CompletedAt, check.DurationSeconds, check.ErrorMessage, check.Output, check.RetryCount, check.CreatedAt, check.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert quality check: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	check.ID = int(id)

	return nil
}

// GetQualityCheck 获取质量检查
func (s *MySQLStorage) GetQualityCheck(id int) (*models.PRQualityCheck, error) {
	var check models.PRQualityCheck
	var errorMessage, output sql.NullString
	var durationSeconds sql.NullFloat64
	var startedAtTime, completedAtTime sql.NullTime

	err := s.db.QueryRow(`
		SELECT id, github_event_id, check_type, check_status, stage, stage_order, check_order, started_at, completed_at, duration_seconds, error_message, output, retry_count, created_at, updated_at
		FROM pr_quality_checks
		WHERE id = ?
	`, id).Scan(
		&check.ID, &check.GitHubEventID, &check.CheckType, &check.CheckStatus, &check.Stage, &check.StageOrder, &check.CheckOrder, &startedAtTime, &completedAtTime, &durationSeconds, &errorMessage, &output, &check.RetryCount, &check.CreatedAt, &check.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("quality check not found")
		}
		return nil, fmt.Errorf("failed to query quality check: %w", err)
	}

	if startedAtTime.Valid {
		check.StartedAt = &startedAtTime.Time
	}
	if completedAtTime.Valid {
		check.CompletedAt = &completedAtTime.Time
	}
	if durationSeconds.Valid {
		check.DurationSeconds = &durationSeconds.Float64
	}
	if errorMessage.Valid {
		check.ErrorMessage = &errorMessage.String
	}
	if output.Valid {
		check.Output = &output.String
	}

	return &check, nil
}

// ListQualityChecksByEventID 列出事件的质量检查项
func (s *MySQLStorage) ListQualityChecksByEventID(eventID string) ([]models.PRQualityCheck, error) {
	rows, err := s.db.Query(`
		SELECT id, github_event_id, check_type, check_status, stage, stage_order, check_order, started_at, completed_at, duration_seconds, error_message, output, retry_count, created_at, updated_at
		FROM pr_quality_checks
		WHERE github_event_id = ?
		ORDER BY stage_order, check_order
	`, eventID)
	if err != nil {
		return nil, fmt.Errorf("failed to query quality checks: %w", err)
	}
	defer rows.Close()

	var checks []models.PRQualityCheck
	for rows.Next() {
		var check models.PRQualityCheck
		var errorMessage, output sql.NullString
		var durationSeconds sql.NullFloat64
		var startedAtTime, completedAtTime sql.NullTime

		if err := rows.Scan(
			&check.ID, &check.GitHubEventID, &check.CheckType, &check.CheckStatus, &check.Stage, &check.StageOrder, &check.CheckOrder, &startedAtTime, &completedAtTime, &durationSeconds, &errorMessage, &output, &check.RetryCount, &check.CreatedAt, &check.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan quality check: %w", err)
		}

		if startedAtTime.Valid {
			check.StartedAt = &startedAtTime.Time
		}
		if completedAtTime.Valid {
			check.CompletedAt = &completedAtTime.Time
		}
		if durationSeconds.Valid {
			check.DurationSeconds = &durationSeconds.Float64
		}
		if errorMessage.Valid {
			check.ErrorMessage = &errorMessage.String
		}
		if output.Valid {
			check.Output = &output.String
		}

		checks = append(checks, check)
	}

	return checks, nil
}

// UpdateQualityCheck 更新质量检查
func (s *MySQLStorage) UpdateQualityCheck(check *models.PRQualityCheck) error {
	_, err := s.db.Exec(`
		UPDATE pr_quality_checks
		SET check_status = ?, started_at = ?, completed_at = ?, duration_seconds = ?, error_message = ?, output = ?, updated_at = ?
		WHERE id = ?
	`, check.CheckStatus, check.StartedAt, check.CompletedAt, check.DurationSeconds, check.ErrorMessage, check.Output, check.UpdatedAt, check.ID)
	if err != nil {
		return fmt.Errorf("failed to update quality check: %w", err)
	}
	return nil
}

// CleanupExpired 清理过期数据
func (s *MySQLStorage) CleanupExpired(ttl time.Duration) error {
	cutoff := time.Now().Add(-ttl)
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec("DELETE FROM pr_quality_checks WHERE github_event_id IN (SELECT event_id FROM github_events WHERE created_at < ?)", cutoff)
	if err != nil {
		return fmt.Errorf("failed to delete expired quality checks: %w", err)
	}

	_, err = tx.Exec("DELETE FROM github_events WHERE created_at < ?", cutoff)
	if err != nil {
		return fmt.Errorf("failed to delete expired events: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}