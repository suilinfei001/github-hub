package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github-hub/internal/quality/models"
)

// Storage 质量引擎存储接口
type Storage interface {
	// 事件相关
	CreateEvent(event *models.GitHubEvent) error
	GetEvent(id int) (*models.GitHubEvent, error)
	GetEventByEventID(eventID string) (*models.GitHubEvent, error)
	ListEvents() ([]*models.GitHubEvent, error)
	UpdateEvent(event *models.GitHubEvent) error
	DeleteEvent(id int) error
	DeleteAllEvents() error

	// 质量检查相关
	CreateQualityCheck(check *models.PRQualityCheck) error
	GetQualityCheck(id int) (*models.PRQualityCheck, error)
	ListQualityChecksByEventID(eventID string) ([]models.PRQualityCheck, error)
	UpdateQualityCheck(check *models.PRQualityCheck) error

	// 清理
	CleanupExpired(ttl time.Duration) error
}

// FileStorage 文件存储实现
type FileStorage struct {
	root        string
	eventsDir   string
	checksDir   string
	nextEventID int
	nextCheckID int
	mu          sync.RWMutex
}

// NewFileStorage 创建新的文件存储
func NewFileStorage(root string) (*FileStorage, error) {
	eventsDir := filepath.Join(root, "events")
	checksDir := filepath.Join(root, "quality_checks")

	// 创建目录
	if err := os.MkdirAll(eventsDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create events directory: %w", err)
	}
	if err := os.MkdirAll(checksDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create checks directory: %w", err)
	}

	// 计算下一个ID
	nextEventID, err := getNextID(eventsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get next event ID: %w", err)
	}

	nextCheckID, err := getNextID(checksDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get next check ID: %w", err)
	}

	return &FileStorage{
		root:        root,
		eventsDir:   eventsDir,
		checksDir:   checksDir,
		nextEventID: nextEventID,
		nextCheckID: nextCheckID,
	}, nil
}

// getNextID 获取下一个ID
func getNextID(dir string) (int, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return 1, nil
	}

	maxID := 0
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		idStr := name[:len(name)-5] // 移除 .json
		id, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}
		if id > maxID {
			maxID = id
		}
	}

	return maxID + 1, nil
}

// CreateEvent 创建事件
func (s *FileStorage) CreateEvent(event *models.GitHubEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	event.ID = s.nextEventID
	s.nextEventID++

	// 创建事件文件
	filePath := filepath.Join(s.eventsDir, fmt.Sprintf("%d.json", event.ID))
	data, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write event file: %w", err)
	}

	// 创建质量检查项
	for i := range event.QualityChecks {
		event.QualityChecks[i].ID = s.nextCheckID
		event.QualityChecks[i].GitHubEventID = event.EventID
		s.nextCheckID++

		checkPath := filepath.Join(s.checksDir, fmt.Sprintf("%d.json", event.QualityChecks[i].ID))
		checkData, err := json.MarshalIndent(event.QualityChecks[i], "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal quality check: %w", err)
		}

		if err := os.WriteFile(checkPath, checkData, 0o644); err != nil {
			return fmt.Errorf("failed to write quality check file: %w", err)
		}
	}

	return nil
}

// GetEvent 获取事件
func (s *FileStorage) GetEvent(id int) (*models.GitHubEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filePath := filepath.Join(s.eventsDir, fmt.Sprintf("%d.json", id))
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("event not found")
		}
		return nil, fmt.Errorf("failed to read event file: %w", err)
	}

	var event models.GitHubEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, fmt.Errorf("failed to unmarshal event: %w", err)
	}

	// 加载质量检查项
	checks, err := s.ListQualityChecksByEventID(event.EventID)
	if err != nil {
		event.QualityChecks = []models.PRQualityCheck{}
	} else {
		event.QualityChecks = checks
	}

	return &event, nil
}

// GetEventByEventID 根据EventID获取事件
func (s *FileStorage) GetEventByEventID(eventID string) (*models.GitHubEvent, error) {
	events, err := s.ListEvents()
	if err != nil {
		return nil, err
	}

	for _, event := range events {
		if event.EventID == eventID {
			return event, nil
		}
	}

	return nil, fmt.Errorf("event not found")
}

// ListEvents 列出所有事件
func (s *FileStorage) ListEvents() ([]*models.GitHubEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	files, err := os.ReadDir(s.eventsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read events directory: %w", err)
	}

	var events []*models.GitHubEvent
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		idStr := name[:len(name)-5] // 移除 .json
		id, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}

		event, err := s.GetEvent(id)
		if err != nil {
			continue
		}

		events = append(events, event)
	}

	// 按ID排序
	sort.Slice(events, func(i, j int) bool {
		return events[i].ID > events[j].ID
	})

	return events, nil
}

// UpdateEvent 更新事件
func (s *FileStorage) UpdateEvent(event *models.GitHubEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := filepath.Join(s.eventsDir, fmt.Sprintf("%d.json", event.ID))
	data, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write event file: %w", err)
	}

	return nil
}

// DeleteEvent 删除事件
func (s *FileStorage) DeleteEvent(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 删除事件文件
	filePath := filepath.Join(s.eventsDir, fmt.Sprintf("%d.json", id))
	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to delete event file: %w", err)
	}

	// 删除相关的质量检查项
	event, err := s.GetEvent(id)
	if err == nil {
		checks, err := s.ListQualityChecksByEventID(event.EventID)
		if err == nil {
			for _, check := range checks {
				checkPath := filepath.Join(s.checksDir, fmt.Sprintf("%d.json", check.ID))
				os.Remove(checkPath)
			}
		}
	}

	return nil
}

// DeleteAllEvents 删除所有事件
func (s *FileStorage) DeleteAllEvents() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 删除所有事件文件
	files, err := os.ReadDir(s.eventsDir)
	if err != nil {
		return fmt.Errorf("failed to read events directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		filePath := filepath.Join(s.eventsDir, file.Name())
		os.Remove(filePath)
	}

	// 删除所有质量检查文件
	checkFiles, err := os.ReadDir(s.checksDir)
	if err != nil {
		return fmt.Errorf("failed to read checks directory: %w", err)
	}

	for _, file := range checkFiles {
		if file.IsDir() {
			continue
		}
		filePath := filepath.Join(s.checksDir, file.Name())
		os.Remove(filePath)
	}

	// 重置ID计数器
	s.nextEventID = 1
	s.nextCheckID = 1

	return nil
}

// CreateQualityCheck 创建质量检查
func (s *FileStorage) CreateQualityCheck(check *models.PRQualityCheck) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	check.ID = s.nextCheckID
	s.nextCheckID++

	filePath := filepath.Join(s.checksDir, fmt.Sprintf("%d.json", check.ID))
	data, err := json.MarshalIndent(check, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal quality check: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write quality check file: %w", err)
	}

	return nil
}

// GetQualityCheck 获取质量检查
func (s *FileStorage) GetQualityCheck(id int) (*models.PRQualityCheck, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	filePath := filepath.Join(s.checksDir, fmt.Sprintf("%d.json", id))
	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("quality check not found")
		}
		return nil, fmt.Errorf("failed to read quality check file: %w", err)
	}

	var check models.PRQualityCheck
	if err := json.Unmarshal(data, &check); err != nil {
		return nil, fmt.Errorf("failed to unmarshal quality check: %w", err)
	}

	return &check, nil
}

// ListQualityChecksByEventID 列出事件的质量检查项
func (s *FileStorage) ListQualityChecksByEventID(eventID string) ([]models.PRQualityCheck, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var checks []models.PRQualityCheck

	files, err := os.ReadDir(s.checksDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read checks directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		if filepath.Ext(name) != ".json" {
			continue
		}
		idStr := name[:len(name)-5] // 移除 .json
		_, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}

		filePath := filepath.Join(s.checksDir, name)
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var check models.PRQualityCheck
		if err := json.Unmarshal(data, &check); err != nil {
			continue
		}

		if check.GitHubEventID == eventID {
			checks = append(checks, check)
		}
	}

	// 按阶段和顺序排序
	sort.Slice(checks, func(i, j int) bool {
		if checks[i].StageOrder != checks[j].StageOrder {
			return checks[i].StageOrder < checks[j].StageOrder
		}
		return checks[i].CheckOrder < checks[j].CheckOrder
	})

	return checks, nil
}

// UpdateQualityCheck 更新质量检查
func (s *FileStorage) UpdateQualityCheck(check *models.PRQualityCheck) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filePath := filepath.Join(s.checksDir, fmt.Sprintf("%d.json", check.ID))
	data, err := json.MarshalIndent(check, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal quality check: %w", err)
	}

	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write quality check file: %w", err)
	}

	return nil
}

// CleanupExpired 清理过期数据
func (s *FileStorage) CleanupExpired(ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	events, err := s.ListEvents()
	if err != nil {
		return err
	}

	now := time.Now()
	for _, event := range events {
		if now.Sub(event.CreatedAt) > ttl {
			s.DeleteEvent(event.ID)
		}
	}

	return nil
}
