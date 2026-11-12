package router

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/BwCloudWeGo/bw-cli/internal/gateway/handler"
	"github.com/BwCloudWeGo/bw-cli/pkg/middleware"
)

// registerTradeRoutes 注册题目 01 的交易路由：买家、仓库、客服、运营与模拟渠道控制台。
// 接口鉴权由 JWT 中间件识别登录用户，角色校验在各服务层完成（防止直接改请求越权）。
func registerTradeRoutes(v1 *gin.RouterGroup, h *handler.Handlers, jwtCfg middleware.JWTConfig, log *zap.Logger) {
	auth := middleware.JWTAuth(jwtCfg)

	products := v1.Group("/products")
	{
		products.GET("", auth, h.Product.List)
		products.POST("", auth, h.Product.Create)
		products.PUT("/:id", auth, h.Product.Update)
	}

	orders := v1.Group("/orders")
	{
		orders.POST("", auth, h.Order.Create)
		orders.GET("/mine", auth, h.Order.ListMy)
		orders.GET("/:orderNo", auth, h.Order.Get)
		orders.POST("/:orderNo/cancel", auth, h.Order.Cancel)
		orders.POST("/:orderNo/pay", auth, h.Order.Pay)
		orders.POST("/:orderNo/refund", auth, h.Order.ApplyRefund)
	}

	warehouse := v1.Group("/warehouse")
	{
		warehouse.GET("/orders", auth, h.Staff.PendingShip)
		warehouse.POST("/orders/:orderNo/ship", auth, h.Staff.Ship)
	}

	cs := v1.Group("/cs")
	{
		cs.GET("/refunds", auth, h.Staff.RefundRequests)
		cs.POST("/refunds/:refundId/review", auth, h.Staff.ReviewRefund)
		cs.POST("/orders/:orderNo/refund-retry", auth, h.Staff.RetryRefund)
		cs.GET("/pay-exceptions", auth, h.Staff.PayExceptions)
		cs.POST("/pay-exceptions/:exceptionId/refund", auth, h.Staff.RefundPayException)
	}

	ops := v1.Group("/ops")
	{
		ops.GET("/summary", auth, h.Staff.OpsSummary)
		ops.GET("/orders/:orderNo/records", auth, h.Staff.OrderRecords)
	}

	channel := v1.Group("/channel")
	{
		channel.GET("/config", h.Channel.GetConfig)
		channel.PUT("/config", h.Channel.SetConfig)
		channel.GET("/records", h.Channel.Records)
	}
}
