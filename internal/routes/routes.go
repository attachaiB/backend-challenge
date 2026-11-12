package routes

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"

	"github.com/attachaiB/backend-challenge/internal/controller"
	"github.com/attachaiB/backend-challenge/internal/middleware"
)

type Handlers struct {
	Auth           *controller.AuthController
	User           *controller.UserController
	AuthMiddleware gin.HandlerFunc
}

func SetupRouter(h Handlers) *gin.Engine {
	useJSONFieldNames()

	// gin.New instead of gin.Default, because we use our own logging middleware.
	router := gin.New()
	router.Use(gin.Recovery(), middleware.Logger())
	_ = router.SetTrustedProxies(nil)

	router.GET("/health", func(ctx *gin.Context) { ctx.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	api := router.Group("/api")
	{
		api.POST("/auth/register", h.Auth.Register)
		api.POST("/auth/login", h.Auth.Login)

		// Every /api/users endpoint requires a valid JWT.
		users := api.Group("/users", h.AuthMiddleware)
		users.POST("", h.User.Create)
		users.GET("", h.User.List)
		users.GET("/:id", h.User.GetByID)
		users.PATCH("/:id", h.User.Update)
		users.DELETE("/:id", h.User.Delete)
	}

	return router
}

// useJSONFieldNames makes validation errors report "email" instead of "Email".
func useJSONFieldNames() {
	validate, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		return
	}

	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		name, _, _ := strings.Cut(field.Tag.Get("json"), ",")
		if name == "" || name == "-" {
			return field.Name
		}

		return name
	})
}
