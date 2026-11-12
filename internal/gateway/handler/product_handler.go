package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	productv1 "github.com/BwCloudWeGo/bw-cli/api/gen/product/v1"
	"github.com/BwCloudWeGo/bw-cli/internal/gateway/request"
	apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"
	"github.com/BwCloudWeGo/bw-cli/pkg/httpx"
)

// ProductHandler 将商品 HTTP 接口适配到 product gRPC client。
type ProductHandler struct {
	client productv1.ProductServiceClient
	log    *zap.Logger
}

func NewProductHandler(client productv1.ProductServiceClient, log *zap.Logger) *ProductHandler {
	return &ProductHandler{client: client, log: log}
}

// List 商品列表（include_off_sale 仅运营使用）。
func (h *ProductHandler) List(c *gin.Context) {
	includeOffSale := c.Query("include_off_sale") == "true"
	resp, err := h.client.ListProducts(outgoingContext(c), &productv1.ListProductsRequest{IncludeOffSale: includeOffSale})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}

// Create 运营创建商品。
func (h *ProductHandler) Create(c *gin.Context) {
	var req request.CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_request", err.Error()))
		return
	}
	resp, err := h.client.CreateProduct(outgoingContext(c), &productv1.CreateProductRequest{
		Name:       req.Name,
		PriceCents: req.PriceCents,
		StockTotal: req.StockTotal,
	})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.Created(c, resp)
}

// Update 运营调整商品。
func (h *ProductHandler) Update(c *gin.Context) {
	var req request.UpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_request", err.Error()))
		return
	}
	resp, err := h.client.UpdateProduct(outgoingContext(c), &productv1.UpdateProductRequest{
		Id:         c.Param("id"),
		SetPrice:   req.SetPrice,
		PriceCents: req.PriceCents,
		SetOnSale:  req.SetOnSale,
		OnSale:     req.OnSale,
		StockDelta: req.StockDelta,
	})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}
