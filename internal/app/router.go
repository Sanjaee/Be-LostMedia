package app

import (
	"log"
	"time"
	"yourapp/internal/config"
	"yourapp/internal/middleware"
	"yourapp/internal/model"
	"yourapp/internal/repository"
	"yourapp/internal/service"
	"yourapp/internal/util"
	"yourapp/internal/websocket"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewRouter(cfg *config.Config) *gin.Engine {
	// Set Gin mode
	if cfg.ServerPort == "5000" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// CORS middleware
	r.Use(corsMiddleware(cfg.ClientURL))

	// Rate limiting middleware (if enabled)
	if cfg.RateLimitEnabled {
		rateLimiter := middleware.NewRateLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)
		r.Use(rateLimiter.Middleware())
		log.Printf("Rate limiting enabled: %d req/sec, burst: %d", cfg.RateLimitRPS, cfg.RateLimitBurst)
	}

	// Initialize database
	db, err := initDB(cfg)
	if err != nil {
		panic("Failed to connect to database: " + err.Error())
	}

	// Auto migrate
	if err := db.AutoMigrate(&model.User{}, &model.Profile{}, &model.Friendship{}, &model.Notification{}, &model.Post{}, &model.PostTag{}, &model.PostLocation{}, &model.Group{}); err != nil {
		panic("Failed to migrate database: " + err.Error())
	}

	// Initialize Redis with retry logic
	redisClient := initRedisWithRetry(cfg)

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	profileRepo := repository.NewProfileRepository(db, redisClient)
	friendshipRepo := repository.NewFriendshipRepository(db, redisClient)
	notificationRepo := repository.NewNotificationRepository(db, redisClient)
	postRepo := repository.NewPostRepository(db, redisClient)

	// Initialize RabbitMQ with retry logic
	rabbitMQ := initRabbitMQWithRetry(cfg)

	// Initialize email service
	emailService := service.NewEmailService(cfg)

	// Initialize email worker if RabbitMQ is available
	var emailWorker *service.EmailWorker
	if rabbitMQ != nil {
		emailWorker = service.NewEmailWorker(emailService, rabbitMQ)
		if err := emailWorker.Start(); err != nil {
			log.Printf("Warning: Failed to start email worker: %v", err)
		} else {
			log.Println("Email worker started successfully")
		}
	} else {
		log.Println("Email worker not started - RabbitMQ connection failed. Will retry on first email send.")
		// Start background goroutine to retry RabbitMQ connection and start email worker
		go func() {
			for {
				time.Sleep(10 * time.Second)
				newRabbitMQ := initRabbitMQWithRetry(cfg)
				if newRabbitMQ != nil {
					log.Println("RabbitMQ reconnected! Starting email worker...")
					emailWorker = service.NewEmailWorker(emailService, newRabbitMQ)
					if err := emailWorker.Start(); err != nil {
						log.Printf("Warning: Failed to start email worker after reconnect: %v", err)
					} else {
						log.Println("Email worker started successfully after reconnect")
						// Update rabbitMQ in authService (we'll need to modify authService to support this)
						// For now, we'll rely on the reconnect logic in PublishEmail
						break
					}
				}
			}
		}()
	}

	// Initialize WebSocket hub
	wsHub := websocket.NewHub()
	go wsHub.Run()
	log.Println("WebSocket hub started")

	// Initialize services
	authService := service.NewAuthServiceWithConfig(userRepo, cfg.JWTSecret, rabbitMQ, cfg)
	profileService := service.NewProfileService(profileRepo, userRepo)
	notificationService := service.NewNotificationService(notificationRepo, rabbitMQ)
	notificationService.SetWSHub(wsHub)
	friendshipService := service.NewFriendshipService(friendshipRepo, userRepo, notificationService)
	postService := service.NewPostService(postRepo, userRepo, friendshipRepo)

	// Initialize notification worker if RabbitMQ is available
	// TODO: Re-enable RabbitMQ worker later for async processing
	/*
	if rabbitMQ != nil {
		notificationWorker := service.NewNotificationWorker(notificationService, rabbitMQ, wsHub)
		if err := notificationWorker.Start(); err != nil {
			log.Printf("Warning: Failed to start notification worker: %v", err)
		} else {
			log.Println("Notification worker started successfully")
		}
	}
	*/
	// For now, notifications are sent directly via WebSocket (no RabbitMQ)

	// Initialize handlers
	authHandler := NewAuthHandler(authService, cfg.JWTSecret)
	profileHandler := NewProfileHandler(profileService, cfg.JWTSecret)
	friendshipHandler := NewFriendshipHandler(friendshipService, cfg.JWTSecret)
	notificationHandler := NewNotificationHandler(notificationService, cfg.JWTSecret)
	postHandler := NewPostHandler(postService, cfg.JWTSecret)

	// API routes
	api := r.Group("/api/v1")
	{
		// Auth routes
		auth := api.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/verify-otp", authHandler.VerifyOTP)
			auth.POST("/resend-otp", authHandler.ResendOTP)
			auth.POST("/google-oauth", authHandler.GoogleOAuth)
			auth.POST("/refresh-token", authHandler.RefreshToken)
			auth.POST("/forgot-password", authHandler.RequestResetPassword)
			auth.POST("/verify-reset-password", authHandler.VerifyResetPassword)
			auth.POST("/reset-password", authHandler.ResetPassword)
			auth.POST("/verify-email", authHandler.VerifyEmail)

			// Protected routes
			auth.GET("/me", authHandler.AuthMiddleware(), authHandler.GetMe)
		}

		// User search routes
		users := api.Group("/users")
		{
			users.Use(authHandler.AuthMiddleware())
			{
				users.GET("/search", authHandler.SearchUsers)
			}
		}

		// Profile routes
		profiles := api.Group("/profiles")
		{
			// Public routes
			profiles.GET("/:id", profileHandler.GetProfile)
			profiles.GET("/user/:userID", profileHandler.GetProfileByUserID)

			// Protected routes
			profiles.Use(authHandler.AuthMiddleware())
			{
				profiles.POST("", profileHandler.CreateProfile)
				profiles.GET("/me", profileHandler.GetMyProfile)
				profiles.PUT("/:id", profileHandler.UpdateProfile)
				profiles.DELETE("/:id", profileHandler.DeleteProfile)
			}
		}

		// Friendship routes
		friendships := api.Group("/friendships")
		{
			// Protected routes
			friendships.Use(authHandler.AuthMiddleware())
			{
				friendships.POST("/request", friendshipHandler.SendFriendRequest)
				friendships.GET("", friendshipHandler.GetMyFriendships)
				friendships.GET("/pending", friendshipHandler.GetPendingRequests)
				friendships.GET("/friends", friendshipHandler.GetFriends)
				friendships.GET("/status/:userID", friendshipHandler.GetFriendshipStatus)
				friendships.GET("/count/:userID", friendshipHandler.GetFriendsCount)
				friendships.GET("/:id", friendshipHandler.GetFriendship)
				friendships.POST("/:id/accept", friendshipHandler.AcceptFriendRequest)
				friendships.POST("/:id/reject", friendshipHandler.RejectFriendRequest)
				friendships.DELETE("/:id", friendshipHandler.RemoveFriend)
			}
		}

		// Notification routes
		notifications := api.Group("/notifications")
		{
			// Protected routes
			notifications.Use(authHandler.AuthMiddleware())
			{
				notifications.GET("", notificationHandler.GetNotifications)
				notifications.GET("/unread", notificationHandler.GetUnreadNotifications)
				notifications.GET("/unread/count", notificationHandler.GetUnreadCount)
				notifications.PUT("/:id/read", notificationHandler.MarkAsRead)
				notifications.PUT("/read-all", notificationHandler.MarkAllAsRead)
				notifications.DELETE("/:id", notificationHandler.DeleteNotification)
			}
		}

		// Post routes
		posts := api.Group("/posts")
		{
			// Public routes (some posts can be viewed without auth)
			posts.GET("/:id", postHandler.GetPost)
			posts.GET("/user/:userID", postHandler.GetPostsByUserID)
			posts.GET("/user/:userID/count", postHandler.CountPostsByUserID)
			posts.GET("/group/:groupID", postHandler.GetPostsByGroupID)
			posts.GET("/group/:groupID/count", postHandler.CountPostsByGroupID)

			// Protected routes
			posts.Use(authHandler.AuthMiddleware())
			{
				posts.POST("", postHandler.CreatePost)
				posts.GET("/feed", postHandler.GetFeed)
				posts.PUT("/:id", postHandler.UpdatePost)
				posts.DELETE("/:id", postHandler.DeletePost)
			}
		}
	}

	// WebSocket route
	r.GET("/ws", func(c *gin.Context) {
		websocket.ServeWS(wsHub, cfg.JWTSecret).ServeHTTP(c.Writer, c.Request)
	})

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	return r
}

func initDB(cfg *config.Config) (*gorm.DB, error) {
	dsn := cfg.DatabaseURL
	if dsn == "" {
		dsn = "host=" + cfg.PostgresHost +
			" port=" + cfg.PostgresPort +
			" user=" + cfg.PostgresUser +
			" password=" + cfg.PostgresPassword +
			" dbname=" + cfg.PostgresDB +
			" sslmode=" + cfg.PostgresSSLMode
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return db, nil
}

// initRabbitMQWithRetry attempts to connect to RabbitMQ with exponential backoff retry
func initRabbitMQWithRetry(cfg *config.Config) *util.RabbitMQClient {
	maxRetries := 10
	initialDelay := 2 * time.Second
	maxDelay := 30 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		rabbitMQ, err := util.NewRabbitMQClient(cfg)
		if err == nil {
			log.Printf("RabbitMQ connected successfully on attempt %d", attempt)
			return rabbitMQ
		}

		if attempt < maxRetries {
			// Calculate delay with exponential backoff
			delay := initialDelay * time.Duration(1<<uint(attempt-1))
			if delay > maxDelay {
				delay = maxDelay
			}

			log.Printf("Failed to connect to RabbitMQ (attempt %d/%d): %v. Retrying in %v...", attempt, maxRetries, err, delay)
			time.Sleep(delay)
		} else {
			log.Printf("Warning: Failed to connect to RabbitMQ after %d attempts: %v. Email sending will be disabled.", maxRetries, err)
			log.Println("Note: RabbitMQ will be retried automatically when email is sent (if connection is restored)")
		}
	}

	return nil
}

// initRedisWithRetry attempts to connect to Redis with exponential backoff retry
func initRedisWithRetry(cfg *config.Config) *util.RedisClient {
	maxRetries := 10
	initialDelay := 2 * time.Second
	maxDelay := 30 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		redisClient, err := util.NewRedisClient(cfg)
		if err == nil {
			log.Printf("Redis connected successfully on attempt %d", attempt)
			return redisClient
		}

		if attempt < maxRetries {
			// Calculate delay with exponential backoff
			delay := initialDelay * time.Duration(1<<uint(attempt-1))
			if delay > maxDelay {
				delay = maxDelay
			}

			log.Printf("Failed to connect to Redis (attempt %d/%d): %v. Retrying in %v...", attempt, maxRetries, err, delay)
			time.Sleep(delay)
		} else {
			log.Printf("Warning: Failed to connect to Redis after %d attempts: %v. Caching will be disabled.", maxRetries, err)
			log.Println("Note: Application will continue without Redis caching")
		}
	}

	return nil
}

func corsMiddleware(clientURL string) gin.HandlerFunc {
	// Allowed origins (whitelist)
	allowedOrigins := []string{
		clientURL, // Default from config
		"http://localhost:3000",
		"https://lost-media-dev.vercel.app",
		"https://lostmedia.zacloth.com",
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is in whitelist
		allowed := false
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				allowed = true
				break
			}
		}

		// If origin is allowed, set it; otherwise, use default
		if allowed {
			c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
		} else {
			c.Writer.Header().Set("Access-Control-Allow-Origin", clientURL)
		}

		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
