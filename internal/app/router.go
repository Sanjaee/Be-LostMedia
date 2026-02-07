package app

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
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
	if err := db.AutoMigrate(&model.User{}, &model.Profile{}, &model.Friendship{}, &model.Notification{}, &model.Post{}, &model.PostTag{}, &model.PostLocation{}, &model.Group{}, &model.Comment{}, &model.Like{}, &model.PostView{}, &model.ChatMessage{}); err != nil {
		panic("Failed to migrate database: " + err.Error())
	}

	// Fix incorrect foreign key constraints for polymorphic likes table
	// GORM may create incorrect foreign key constraints for polymorphic relationships
	// We need to drop any foreign key constraints on target_id since it can reference multiple tables
	fixLikesTableConstraints(db)

	// Initialize Redis with retry logic
	redisClient := initRedisWithRetry(cfg)

	// Initialize repositories
	userRepo := repository.NewUserRepository(db)
	profileRepo := repository.NewProfileRepository(db, redisClient)
	friendshipRepo := repository.NewFriendshipRepository(db, redisClient)
	notificationRepo := repository.NewNotificationRepository(db, redisClient)
	postRepo := repository.NewPostRepository(db, redisClient)
	commentRepo := repository.NewCommentRepository(db, redisClient)
	likeRepo := repository.NewLikeRepository(db, redisClient)
	chatRepo := repository.NewChatRepository(db)

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

	// Initialize tmp directory
	if err := initTmpDir(); err != nil {
		log.Printf("Warning: Failed to create tmp directory: %v", err)
	} else {
		log.Println("Tmp directory initialized successfully")
	}

	// Initialize Cloudinary client
	var cloudinaryClient *util.CloudinaryClient
	if cfg.CloudinaryCloudName != "" && cfg.CloudinaryAPIKey != "" && cfg.CloudinaryAPISecret != "" {
		var err error
		cloudinaryClient, err = util.NewCloudinaryClient(cfg)
		if err != nil {
			log.Printf("Warning: Failed to initialize Cloudinary: %v. Image uploads will be disabled.", err)
		} else {
			log.Println("Cloudinary initialized successfully")
		}
	} else {
		log.Println("Cloudinary credentials not configured. Image uploads will be disabled.")
	}

	// Initialize services
	authService := service.NewAuthServiceWithConfig(userRepo, cfg.JWTSecret, rabbitMQ, cfg)
	profileService := service.NewProfileService(profileRepo, userRepo)
	notificationService := service.NewNotificationService(notificationRepo, rabbitMQ)
	notificationService.SetWSHub(wsHub)
	friendshipService := service.NewFriendshipService(friendshipRepo, userRepo, notificationService)
	postService := service.NewPostService(postRepo, userRepo, friendshipRepo)
	postViewRepo := repository.NewPostViewRepository(db, redisClient)
	postViewService := service.NewPostViewService(postViewRepo, postRepo, userRepo)
	commentService := service.NewCommentService(commentRepo, userRepo, postRepo, notificationService)
	likeService := service.NewLikeService(likeRepo, userRepo, postRepo, commentRepo)
	chatService := service.NewChatService(chatRepo, userRepo, friendshipRepo)

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
	userHandler := NewUserHandler(userRepo, cfg.JWTSecret, wsHub, notificationService)
	profileHandler := NewProfileHandler(profileService, cfg.JWTSecret)
	friendshipHandler := NewFriendshipHandler(friendshipService, cfg.JWTSecret)
	notificationHandler := NewNotificationHandler(notificationService, cfg.JWTSecret)

	// Initialize post handler with Cloudinary if available
	var postHandler *PostHandler
	if cloudinaryClient != nil {
		postHandler = NewPostHandlerWithCloudinary(postService, postViewService, notificationService, cloudinaryClient, wsHub, likeService, commentService, cfg.JWTSecret)
	} else {
		// Create a simple post handler without Cloudinary but with view service and engagement enrichment
		postHandler = &PostHandler{
			postService:     postService,
			postViewService: postViewService,
			likeService:     likeService,
			commentService: commentService,
			jwtSecret:       cfg.JWTSecret,
		}
	}

	commentHandler := NewCommentHandler(commentService, cfg.JWTSecret)
	likeHandler := NewLikeHandlerWithNotification(likeService, notificationService, postService, userRepo, cfg.JWTSecret)
	chatHandler := NewChatHandler(chatService, wsHub)

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
			auth.DELETE("/account", authHandler.AuthMiddleware(), authHandler.DeleteAccount)
		}

		// User search routes
		users := api.Group("/users")
		{
			users.Use(authHandler.AuthMiddleware())
			{
				users.GET("/search", authHandler.SearchUsers)
			}
		}

		// Admin routes (owner only)
		admin := api.Group("/admin")
		{
			admin.Use(authHandler.AuthMiddleware())
			admin.Use(authHandler.AdminMiddleware())
			{
				admin.GET("/users", userHandler.GetAllUsers)
				admin.GET("/stats", userHandler.GetUserStats)
				admin.POST("/users/:id/ban", userHandler.BanUser)
				admin.POST("/users/:id/unban", userHandler.UnbanUser)
				admin.PUT("/users/:id/role", userHandler.UpdateUserRole)
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
			// IMPORTANT: More specific routes must be registered before wildcard routes
			posts.GET("/user/:userID", postHandler.GetPostsByUserID)
			posts.GET("/user/:userID/count", postHandler.CountPostsByUserID)
			posts.GET("/group/:groupID", postHandler.GetPostsByGroupID)
			posts.GET("/group/:groupID/count", postHandler.CountPostsByGroupID)

			// Post comments routes (must be before /:id route to avoid conflict)
			// Route with more segments must be registered first
			posts.GET("/:id/comments", commentHandler.GetCommentsByPost)
			posts.GET("/:id/comments/count", commentHandler.GetCommentCount)

			// Post views routes (must be before /:id route to avoid conflict)
			posts.GET("/:id/views/count", postHandler.GetViewCount)

			// Post detail route (wildcard route - must be last)
			posts.GET("/:id", postHandler.GetPost)

			// Protected routes
			posts.Use(authHandler.AuthMiddleware())
			{
				posts.POST("", postHandler.CreatePost)
				posts.POST("/upload", postHandler.CreatePostWithImages) // Async image upload
				posts.GET("/feed", postHandler.GetFeed)
				posts.PUT("/:id", postHandler.UpdatePost)
				posts.DELETE("/:id", postHandler.DeletePost)
				posts.POST("/:id/view", postHandler.TrackView) // Track post view

				// Post likes
				posts.POST("/:id/like", likeHandler.LikePost)
				posts.DELETE("/:id/like", likeHandler.UnlikePost)
			}
		}

		// Comment routes
		comments := api.Group("/comments")
		{
			// Public routes
			comments.GET("/:id", commentHandler.GetComment)
			comments.GET("/:id/replies", commentHandler.GetReplies)

			// Protected routes
			comments.Use(authHandler.AuthMiddleware())
			{
				comments.POST("", commentHandler.CreateComment)
				comments.PUT("/:id", commentHandler.UpdateComment)
				comments.DELETE("/:id", commentHandler.DeleteComment)

				// Comment likes
				comments.POST("/:id/like", likeHandler.LikeComment)
				comments.DELETE("/:id/like", likeHandler.UnlikeComment)
			}
		}

		// Like routes
		likes := api.Group("/likes")
		{
			// Public routes
			likes.GET("", likeHandler.GetLikes)
			likes.GET("/count", likeHandler.GetLikeCount)
		}

		// Chat routes
		chat := api.Group("/chat")
		chat.Use(authHandler.AuthMiddleware())
		{
			chat.POST("/messages", chatHandler.SendMessage)
			chat.GET("/messages", chatHandler.GetConversation)
			chat.PUT("/read/:senderID", chatHandler.MarkAsRead)
			chat.GET("/unread/by-senders", chatHandler.GetUnreadCountBySenders)
			chat.GET("/unread/count", chatHandler.GetUnreadCount)
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

// initTmpDir initializes the tmp directory for file uploads
func initTmpDir() error {
	wd, err := os.Getwd()
	if err != nil {
		// Fallback to temp directory if can't get working directory
		tmpDir := filepath.Join(os.TempDir(), "tmp")
		return os.MkdirAll(tmpDir, 0755)
	}

	tmpDir := filepath.Join(wd, "tmp")
	return os.MkdirAll(tmpDir, 0755)
}

// fixLikesTableConstraints removes incorrect foreign key constraints from the likes table
// Since likes.target_id is polymorphic (can reference posts or comments), we cannot have
// a foreign key constraint on it. GORM may create incorrect constraints during AutoMigrate.
func fixLikesTableConstraints(db *gorm.DB) {
	// Query to find all foreign key constraints on the likes table
	query := `
		SELECT constraint_name 
		FROM information_schema.table_constraints 
		WHERE table_name = 'likes' 
		AND constraint_type = 'FOREIGN KEY'
		AND constraint_name IN (
			SELECT constraint_name
			FROM information_schema.key_column_usage
			WHERE table_name = 'likes' 
			AND column_name = 'target_id'
		)
	`

	var constraints []struct {
		ConstraintName string `gorm:"column:constraint_name"`
	}

	if err := db.Raw(query).Scan(&constraints).Error; err != nil {
		log.Printf("Warning: Failed to query foreign key constraints on likes table: %v", err)
		return
	}

	// Drop all found constraints
	for _, constraint := range constraints {
		dropQuery := fmt.Sprintf("ALTER TABLE likes DROP CONSTRAINT IF EXISTS %s", constraint.ConstraintName)
		if err := db.Exec(dropQuery).Error; err != nil {
			log.Printf("Warning: Failed to drop constraint %s: %v", constraint.ConstraintName, err)
		} else {
			log.Printf("Dropped incorrect foreign key constraint: %s", constraint.ConstraintName)
		}
	}

	// Also try to drop known constraint names that might exist
	knownConstraints := []string{
		"fk_comments_likes",
		"likes_target_id_fkey",
		"fk_likes_comments",
		"fk_likes_posts",
	}

	for _, constraintName := range knownConstraints {
		dropQuery := fmt.Sprintf("ALTER TABLE likes DROP CONSTRAINT IF EXISTS %s", constraintName)
		if err := db.Exec(dropQuery).Error; err != nil {
			// Ignore errors for constraints that don't exist
			log.Printf("Note: Constraint %s does not exist or already dropped", constraintName)
		}
	}
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
