package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github-hub/internal/quality/api"
	"github-hub/internal/quality/storage"
)

func main() {
	// 解析命令行参数
	var (
		addr   = flag.String("addr", ":5001", "服务器监听地址")
		root   = flag.String("root", ".", "数据存储根目录")
		dbDSN  = flag.String("db", "", "MySQL数据库连接字符串 (例如: root:password@tcp(localhost:3306)/github_hub?parseTime=true)")
	)
	flag.Parse()

	var store storage.Storage
	var err error

	// 如果提供了数据库连接字符串，使用MySQL存储
	if *dbDSN != "" {
		store, err = storage.NewMySQLStorage(*dbDSN)
		if err != nil {
			log.Fatalf("Failed to create MySQL storage: %v", err)
		}
		log.Printf("Using MySQL storage")
	} else {
		// 确保根目录存在
		if err := os.MkdirAll(*root, 0o755); err != nil {
			log.Fatalf("Failed to create root directory: %v", err)
		}

		// 创建质量引擎服务器
		qualityDir := filepath.Join(*root, "quality-engine")
		store, err = storage.NewFileStorage(qualityDir)
		if err != nil {
			log.Fatalf("Failed to create file storage: %v", err)
		}
		log.Printf("Using file storage: %s", qualityDir)
	}

	// 创建质量引擎服务器
	server, err := api.NewServerWithStorage(store)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// 创建HTTP多路复用器
	mux := http.NewServeMux()

	// 注册路由
	server.RegisterRoutes(mux)

	// 启动服务器
	log.Printf("Starting quality-engine server on %s", *addr)
	if *dbDSN != "" {
		log.Printf("Data storage: MySQL database")
	} else {
		log.Printf("Data storage: %s", filepath.Join(*root, "quality-engine"))
	}
	log.Printf("Webhook endpoint: http://localhost%s/webhook", *addr)
	log.Printf("API endpoint: http://localhost%s/api", *addr)

	if err := http.ListenAndServe(*addr, mux); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
