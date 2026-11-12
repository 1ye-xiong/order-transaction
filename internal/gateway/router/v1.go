package router

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/BwCloudWeGo/bw-cli/internal/gateway/client"
	"github.com/BwCloudWeGo/bw-cli/internal/gateway/handler"
	"github.com/BwCloudWeGo/bw-cli/pkg/config"
)

// registerAPIRoutes 创建 /api/v1 路由命名空间，再按业务模块分发。
func registerAPIRoutes(r *gin.Engine, clients *client.Clients, log *zap.Logger, middlewareCfg config.MiddlewareConfig) {
	api := r.Group("/api")
	v1 := api.Group("/v1")

	registerUserRoutes(v1, handler.NewUserHandler(clients.User, middlewareCfg.JWT, log), middlewareCfg.JWT)
	registerNoteRoutes(v1, handler.NewNoteHandler(clients.Note, log))
	staff := handler.NewStaffHandler(clients.Order, clients.Warehouse, log)
	h := handler.NewHandlers(clients.Product, clients.Order, clients.Payment, staff, log)
	registerTradeRoutes(v1, h, middlewareCfg.JWT, log)

	registerStaticPages(r)
}

// registerStaticPages 提供教学演示前端页面（可点击操作的原型）。
func registerStaticPages(r *gin.Engine) {
	// Keep the original teaching page as an explicit fallback when frontend is not built.
	index := "./web/index.html"
	if _, err := os.Stat("./web/dist/index.html"); err == nil {
		index = "./web/dist/index.html"
		r.Static("/assets", "./web/dist/assets")
	}
	r.StaticFile("/legacy", "./web/index.html")
	r.GET("/", func(c *gin.Context) {
		c.File(index)
	})
	r.NoRoute(func(c *gin.Context) {
		if c.Request.Method == http.MethodGet && !strings.HasPrefix(c.Request.URL.Path, "/api") {
			c.File(index)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})
}
