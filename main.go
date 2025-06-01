package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/BerniceZTT/crm_end/config"
	"github.com/BerniceZTT/crm_end/middleware"
	"github.com/BerniceZTT/crm_end/repository"
	"github.com/BerniceZTT/crm_end/routes"
	"github.com/BerniceZTT/crm_end/utils"

	"github.com/gin-gonic/gin"
)

func main() {
	// 初始化日志
	utils.InitLogger()

	// 加载配置
	cfg := config.LoadConfig()

	// 设置Gin模式
	if cfg.Debug {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// 初始化数据库
	if err := repository.InitMongoDB(cfg.MongoURI, cfg.MongoDB); err != nil {
		utils.Logger.Fatal().Err(err).Msg("Failed to connect to MongoDB")
	}

	defer repository.CloseMongoDB()

	// 创建Gin实例
	router := gin.New()

	// 应用中间件
	router.Use(middleware.Logger())
	router.Use(middleware.Recovery())
	router.Use(middleware.CORS())
	router.Use(middleware.ErrorHandler())
	router.Use(middleware.OperationLoggerMiddleware())

	// 注册路由
	routes.RegisterRoutes(router)

	// 初始化系统数据
	utils.Logger.Info().Msg("开始系统初始化...")
	if err := repository.InitializeCollections(); err != nil {
		utils.Logger.Error().Err(err).Msg("初始化数据库集合失败")
	}
	if err := repository.InitializeAdminAccount(); err != nil {
		utils.Logger.Error().Err(err).Msg("初始化管理员账户失败")
	}
	utils.Logger.Info().Msg("系统初始化完成")

	// 设置HTTP服务器
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 启动服务器
	go func() {
		utils.Logger.Info().Msgf("服务器启动，监听端口: %d", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			utils.Logger.Fatal().Err(err).Msg("启动服务器失败")
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	utils.Logger.Info().Msg("正在关闭服务器...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		utils.Logger.Fatal().Err(err).Msg("服务器关闭异常")
	}

	utils.Logger.Info().Msg("服务器已优雅关闭")
}

// sudo nano /usr/local/bin/mongodb_backup.sh

// # 解压备份文件
// tar -xzvf /backup/mongodb/mongodb_backup_20240101_030000.tar.gz -C /tmp

// # 执行恢复
// mongorestore \
//   --host localhost \
//   --port 27017 \
//   --username admin \
//   --password your_password \
//   --authenticationDatabase admin \
//   /tmp/mongodb_backup_20240101_030000
