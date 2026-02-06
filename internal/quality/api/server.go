package api

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github-hub/internal/quality/handlers"
	"github-hub/internal/quality/models"
	"github-hub/internal/quality/storage"
)

// Server 质量引擎服务器
type Server struct {
	storage     storage.Storage
	prHandler   *handlers.PRHandler
	pushHandler *handlers.PushHandler
	qualityDir  string
}

// NewServer 创建新的质量引擎服务器
func NewServer(root string) (*Server, error) {
	qualityDir := filepath.Join(root, "quality-engine")
	if err := os.MkdirAll(qualityDir, 0o755); err != nil {
		return nil, err
	}

	// 创建存储
	store, err := storage.NewFileStorage(qualityDir)
	if err != nil {
		return nil, err
	}

	return NewServerWithStorage(store)
}

// NewServerWithStorage 使用提供的存储创建新的质量引擎服务器
func NewServerWithStorage(store storage.Storage) (*Server, error) {
	// 创建处理器
	prHandler := handlers.NewPRHandler(store)
	pushHandler := handlers.NewPushHandler(store)

	return &Server{
		storage:     store,
		prHandler:   prHandler,
		pushHandler: pushHandler,
		qualityDir:  "/usr/local/share/quality-data",
	}, nil
}

// RegisterRoutes 注册路由
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	// Webhook 端点
	mux.HandleFunc("/webhook", s.handleWebhook)

	// API 端点
	mux.HandleFunc("/api/events", s.handleEvents)
	mux.HandleFunc("/api/repositories", s.handleRepositories)
	mux.HandleFunc("/api/mock/events", s.handleMockEvents)
	mux.HandleFunc("/api/mock/simulate/", s.handleMockSimulate)
	mux.HandleFunc("/api/login", s.handleLogin)
	mux.HandleFunc("/api/logout", s.handleLogout)
	mux.HandleFunc("/api/check-login", s.handleCheckLogin)
	mux.HandleFunc("/api/status", s.handleStatus)

	// 动态路由处理
	mux.HandleFunc("/api/", s.handleDynamicRoutes)

	// 静态文件 (仅在文件存储模式下)
	if s.qualityDir != "" {
		mux.Handle("/", http.FileServer(http.Dir(filepath.Join(s.qualityDir, "static"))))
	}
}

// handleWebhook 处理Webhook事件
func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 获取事件类型
	eventType := r.Header.Get("X-GitHub-Event")
	if eventType == "" {
		http.Error(w, "missing X-GitHub-Event header", http.StatusBadRequest)
		return
	}

	// 解析请求体
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	log.Printf("Received event: %s", eventType)

	// 事件过滤逻辑
	shouldProcess := false

	if eventType == "push" {
		// Push事件过滤：只处理main分支
		shouldProcess = models.ShouldProcessPushEvent(payload)
		if shouldProcess {
			log.Println("Processing push event")
		} else {
			log.Println("Skipping push event")
		}

	} else if eventType == "pull_request" {
		// PR事件过滤：只处理非main分支合入main分支的事件
		shouldProcess = models.ShouldProcessPREvent(payload)
		if shouldProcess {
			log.Println("Processing PR event")
		} else {
			log.Println("Skipping PR event")
		}
	}

	if !shouldProcess {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "skipped",
			"event":  eventType,
		})
		return
	}

	// 异步处理事件
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic in event processing: %v", r)
			}
		}()

		// 根据事件类型处理
		if eventType == "push" {
			s.pushHandler.Handle(payload)
		} else if eventType == "pull_request" {
			s.prHandler.Handle(payload)
		} else {
			log.Printf("Unknown event type: %s", eventType)
		}
	}()

	// 返回响应
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "received",
		"event":  eventType,
	})
}

// handleEvents 处理事件列表请求
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.handleGetEvents(w, r)
	case http.MethodDelete:
		s.handleDeleteAllEvents(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetEvents 处理获取事件列表
func (s *Server) handleGetEvents(w http.ResponseWriter, r *http.Request) {
	// 获取查询参数
	eventType := r.URL.Query().Get("event_type")
	status := r.URL.Query().Get("status")
	branch := r.URL.Query().Get("branch")
	repository := r.URL.Query().Get("repository")

	events, err := s.storage.ListEvents()
	if err != nil {
		http.Error(w, "failed to list events", http.StatusInternalServerError)
		return
	}

	// 过滤事件
	filteredEvents := []*models.GitHubEvent{}
	for _, event := range events {
		// 按事件类型过滤
		if eventType != "" && string(event.EventType) != eventType {
			continue
		}
		// 按状态过滤
		if status != "" && string(event.EventStatus) != status {
			continue
		}
		// 按分支过滤
		if branch != "" && event.Branch != branch {
			continue
		}
		// 按仓库过滤
		if repository != "" && event.Repository != repository {
			continue
		}
		filteredEvents = append(filteredEvents, event)
	}

	// 格式化响应
	response := map[string]interface{}{
		"success": true,
		"data":    filteredEvents,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleDynamicRoutes 处理动态路由
func (s *Server) handleDynamicRoutes(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	if r.Method == http.MethodGet && len(path) > len("/api/events/") {
		idStr := path[len("/api/events/"):]
		if id, err := strconv.Atoi(idStr); err == nil {
			s.handleEventDetail(w, r, id)
			return
		}
	}

	if r.Method == http.MethodGet && len(path) > len("/api/events/") && path[len(path)-len("/quality-checks"):] == "/quality-checks" {
		eventIDStr := path[len("/api/events/") : len(path)-len("/quality-checks")]
		if eventIDStr != "" {
			s.handleQualityChecks(w, r, eventIDStr)
			return
		}
	}

	if r.Method == http.MethodPut && len(path) > len("/api/quality-checks/") {
		idStr := path[len("/api/quality-checks/"):]
		if id, err := strconv.Atoi(idStr); err == nil {
			s.handleQualityCheckUpdate(w, r, id)
			return
		}
	}

	http.NotFound(w, r)
}

// handleDeleteAllEvents 处理删除所有事件
func (s *Server) handleDeleteAllEvents(w http.ResponseWriter, r *http.Request) {
	if err := s.storage.DeleteAllEvents(); err != nil {
		http.Error(w, "failed to delete all events", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"message": "数据库清空成功",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleEventDetail 处理事件详情请求
func (s *Server) handleEventDetail(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	event, err := s.storage.GetEvent(id)
	if err != nil {
		http.Error(w, "event not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    event,
	})
}

// handleRepositories 处理仓库列表请求
func (s *Server) handleRepositories(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 简化实现：返回空列表
	response := map[string]interface{}{
		"success": true,
		"data":    []interface{}{},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleQualityChecks 处理质量检查列表请求
func (s *Server) handleQualityChecks(w http.ResponseWriter, r *http.Request, eventID string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	checks, err := s.storage.ListQualityChecksByEventID(eventID)
	if err != nil {
		checks = []models.PRQualityCheck{}
	}

	response := map[string]interface{}{
		"success": true,
		"data":    checks,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleQualityCheckUpdate 处理质量检查更新请求
func (s *Server) handleQualityCheckUpdate(w http.ResponseWriter, r *http.Request, id int) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	check, err := s.storage.GetQualityCheck(id)
	if err != nil {
		http.Error(w, "quality check not found", http.StatusNotFound)
		return
	}

	// 解析请求体
	var updateData map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		http.Error(w, "invalid JSON payload", http.StatusBadRequest)
		return
	}

	// 更新质量检查状态
	if statusStr, ok := updateData["status"].(string); ok {
		if status, err := models.ParseQualityCheckStatus(statusStr); err == nil {
			check.CheckStatus = status
		}
	}

	// 更新错误信息
	if errorMsg, ok := updateData["error_message"].(string); ok {
		check.ErrorMessage = &errorMsg
	}

	// 更新输出
	if output, ok := updateData["output"].(string); ok {
		check.Output = &output
	}

	// 更新完成时间
	now := time.Now()
	check.CompletedAt = &now

	// 保存更新
	if err := s.storage.UpdateQualityCheck(check); err != nil {
		http.Error(w, "failed to update quality check", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    check,
	})
}

// handleMockEvents 处理Mock事件列表请求
func (s *Server) handleMockEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 从JSON文件读取预定义的Mock事件模板
	mockDataPath := filepath.Join(s.qualityDir, "github_webhook_payload_mock.json")
	mockData, err := os.ReadFile(mockDataPath)
	if err != nil {
		log.Printf("Failed to read mock data file: %v", err)
		// 如果文件不存在，返回空数组
		response := map[string]interface{}{
			"success": true,
			"data":    []map[string]interface{}{},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	var mockEvents []map[string]interface{}
	if err := json.Unmarshal(mockData, &mockEvents); err != nil {
		log.Printf("Failed to parse mock data: %v", err)
		http.Error(w, "failed to parse mock data", http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"success": true,
		"data":    mockEvents,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleMockSimulate 处理模拟事件请求
func (s *Server) handleMockSimulate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 提取事件类型
	path := r.URL.Path
	eventTypeStr := path[len("/api/mock/simulate/"):]
	if eventTypeStr == "" {
		http.Error(w, "missing event type", http.StatusBadRequest)
		return
	}

	// 从JSON文件读取mock数据
	mockDataPath := filepath.Join(s.qualityDir, "github_webhook_payload_mock.json")
	mockData, err := os.ReadFile(mockDataPath)
	if err != nil {
		log.Printf("Failed to read mock data file: %v", err)
		http.Error(w, "failed to read mock data", http.StatusInternalServerError)
		return
	}

	var mockEvents []map[string]interface{}
	if err := json.Unmarshal(mockData, &mockEvents); err != nil {
		log.Printf("Failed to parse mock data: %v", err)
		http.Error(w, "failed to parse mock data", http.StatusInternalServerError)
		return
	}

	// 查找匹配的mock数据
	var selectedMockData map[string]interface{}
	simpleEventType := eventTypeStr
	var action string
	if strings.HasPrefix(eventTypeStr, "pull_request.") {
		simpleEventType = "pull_request"
		action = strings.TrimPrefix(eventTypeStr, "pull_request.")
	}

	for _, mockEvent := range mockEvents {
		if mockEventType, ok := mockEvent["event_type"].(string); ok {
			if mockEventType == simpleEventType {
				// 对于PR事件，检查action是否匹配
				if simpleEventType == "pull_request" && action != "" {
					if mockAction, ok := mockEvent["pr_action"].(string); ok {
						if mockAction == action {
							selectedMockData = mockEvent
							break
						}
					}
				} else {
					selectedMockData = mockEvent
					break
				}
			}
		}
	}

	// 如果没有找到匹配的mock数据，使用空数据
	if selectedMockData == nil {
		selectedMockData = make(map[string]interface{})
		selectedMockData["event_type"] = eventTypeStr
	}

	// 异步处理事件
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic in mock event processing: %v", r)
			}
		}()

		// 根据事件类型处理
		if simpleEventType == "pull_request" {
			s.prHandler.Handle(selectedMockData)
		} else if simpleEventType == "push" {
			s.pushHandler.Handle(selectedMockData)
		} else {
			log.Printf("Unknown mock event type: %s", eventTypeStr)
		}
	}()

	// 返回响应
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"message":    "Mock event received and being processed",
		"event_type": eventTypeStr,
	})
}

// handleLogin 处理登录请求
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 简化实现：固定返回登录成功
	response := map[string]interface{}{
		"success":  true,
		"message":  "登录成功",
		"username": "admin",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleLogout 处理登出请求
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 简化实现：固定返回登出成功
	response := map[string]interface{}{
		"success": true,
		"message": "登出成功",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleCheckLogin 处理登录状态检查请求
func (s *Server) handleCheckLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 简化实现：固定返回已登录
	response := map[string]interface{}{
		"is_logged_in": true,
		"username":     "admin",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleStatus 处理系统状态请求
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 获取事件统计
	events, err := s.storage.ListEvents()
	totalEvents := 0
	pendingEvents := 0
	if err == nil {
		totalEvents = len(events)
		for _, event := range events {
			if event.EventStatus == models.EventStatusPending {
				pendingEvents++
			}
		}
	}

	response := map[string]interface{}{
		"success": true,
		"data": map[string]interface{}{
			"service_status":  "healthy",
			"database_status": "connected",
			"total_events":    totalEvents,
			"pending_events":  pendingEvents,
			"db_type":         "File Storage",
			"db_host":         "localhost",
			"db_name":         "quality-engine",
			"version":         "1.0.0",
			"uptime":          "Unknown",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
