package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
)

// fakeRepo 是订单域仓储的内存实现，专用于用例层单元测试。
type fakeRepo struct {
	mu         sync.Mutex
	orders     map[string]*entity.Order
	orderSeq   []string
	records    []entity.OrderRecord
	refunds    map[uint]*entity.RefundRequest
	refundSeq  uint
	exceptions map[uint]*entity.PayException
	exByTrade  map[string]uint
	exSeq      uint
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		orders:     map[string]*entity.Order{},
		refunds:    map[uint]*entity.RefundRequest{},
		exceptions: map[uint]*entity.PayException{},
		exByTrade:  map[string]uint{},
	}
}

func (f *fakeRepo) Create(_ context.Context, order *entity.Order) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *order
	f.orders[order.OrderNo] = &cp
	f.orderSeq = append(f.orderSeq, order.OrderNo)
	return nil
}

func (f *fakeRepo) FindByOrderNo(_ context.Context, orderNo string) (*entity.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.orders[orderNo]
	if !ok {
		return nil, entity.ErrOrderNotFound
	}
	cp := *o
	return &cp, nil
}

func (f *fakeRepo) CancelPending(_ context.Context, orderNo string, reason string, now time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.orders[orderNo]
	if !ok {
		return false, entity.ErrOrderNotFound
	}
	if o.Status != entity.StatusPendingPayment {
		return false, nil
	}
	o.Status = entity.StatusCancelled
	o.CancelReason = reason
	t := now
	o.ClosedAt = &t
	return true, nil
}

func (f *fakeRepo) CloseIfExpired(_ context.Context, orderNo string, now time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.orders[orderNo]
	if !ok {
		return false, entity.ErrOrderNotFound
	}
	if o.Status != entity.StatusPendingPayment || !now.After(o.PayExpireAt) {
		return false, nil
	}
	o.Status = entity.StatusClosedTimeout
	t := now
	o.ClosedAt = &t
	return true, nil
}

func (f *fakeRepo) MarkPaid(_ context.Context, orderNo string, tradeNo string, now time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, other := range f.orders {
		if other.ChannelTradeNo == tradeNo && other.ChannelTradeNo != "" {
			return false, errors.New("unique constraint: channel_trade_no")
		}
	}
	o, ok := f.orders[orderNo]
	if !ok {
		return false, entity.ErrOrderNotFound
	}
	if o.Status != entity.StatusPendingPayment || now.After(o.PayExpireAt) {
		return false, nil
	}
	o.Status = entity.StatusPaid
	o.ChannelTradeNo = tradeNo
	t := now
	o.PaidAt = &t
	return true, nil
}

func (f *fakeRepo) Transition(_ context.Context, orderNo string, from string, to string, apply func(*entity.Order), now time.Time) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.orders[orderNo]
	if !ok {
		return false, entity.ErrOrderNotFound
	}
	if o.Status != from {
		return false, nil
	}
	if apply != nil {
		apply(o)
	}
	o.Status = to
	o.UpdatedAt = now
	return true, nil
}

func (f *fakeRepo) ListByBuyer(_ context.Context, buyerID string, offset int, limit int) ([]*entity.Order, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var items []*entity.Order
	var total int64
	for _, no := range f.orderSeq {
		o := f.orders[no]
		if o.BuyerID == buyerID {
			total++
			cp := *o
			items = append(items, &cp)
		}
	}
	return items, total, nil
}

func (f *fakeRepo) ListByStatus(_ context.Context, status string) ([]*entity.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var items []*entity.Order
	for _, no := range f.orderSeq {
		o := f.orders[no]
		if o.Status == status {
			cp := *o
			items = append(items, &cp)
		}
	}
	return items, nil
}

func (f *fakeRepo) ListExpiredPending(_ context.Context, now time.Time) ([]*entity.Order, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var items []*entity.Order
	for _, no := range f.orderSeq {
		o := f.orders[no]
		if o.Status == entity.StatusPendingPayment && now.After(o.PayExpireAt) {
			cp := *o
			items = append(items, &cp)
		}
	}
	return items, nil
}

func (f *fakeRepo) CreateRecord(_ context.Context, record *entity.OrderRecord) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	record.ID = uint(len(f.records) + 1)
	f.records = append(f.records, *record)
	return nil
}

func (f *fakeRepo) ListRecords(_ context.Context, orderNo string) ([]*entity.OrderRecord, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var items []*entity.OrderRecord
	for i := range f.records {
		if f.records[i].OrderNo == orderNo {
			cp := f.records[i]
			items = append(items, &cp)
		}
	}
	return items, nil
}

func (f *fakeRepo) CreateRefundRequest(_ context.Context, req *entity.RefundRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refundSeq++
	req.ID = f.refundSeq
	cp := *req
	f.refunds[req.ID] = &cp
	return nil
}

func (f *fakeRepo) FindRefundRequest(_ context.Context, id uint) (*entity.RefundRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, ok := f.refunds[id]
	if !ok {
		return nil, errors.New("refund request not found")
	}
	cp := *r
	return &cp, nil
}

func (f *fakeRepo) FindPendingRefundByOrder(_ context.Context, orderNo string) (*entity.RefundRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.refunds {
		if r.OrderNo == orderNo && r.Status == entity.RefundPending {
			cp := *r
			return &cp, nil
		}
	}
	return nil, nil
}

func (f *fakeRepo) FindLatestRefundByOrder(_ context.Context, orderNo string) (*entity.RefundRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var latest *entity.RefundRequest
	for _, r := range f.refunds {
		if r.OrderNo == orderNo && (latest == nil || r.ID > latest.ID) {
			cp := *r
			latest = &cp
		}
	}
	return latest, nil
}

func (f *fakeRepo) UpdateRefundRequest(_ context.Context, req *entity.RefundRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *req
	f.refunds[req.ID] = &cp
	return nil
}

func (f *fakeRepo) ListRefundRequests(_ context.Context, status string) ([]*entity.RefundRequest, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var items []*entity.RefundRequest
	for _, r := range f.refunds {
		if status == "" || r.Status == status {
			cp := *r
			items = append(items, &cp)
		}
	}
	return items, nil
}

func (f *fakeRepo) CreatePayException(_ context.Context, ex *entity.PayException) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, dup := f.exByTrade[ex.ChannelTradeNo]; dup {
		return errors.New("unique constraint: channel_trade_no")
	}
	f.exSeq++
	ex.ID = f.exSeq
	cp := *ex
	f.exceptions[ex.ID] = &cp
	f.exByTrade[ex.ChannelTradeNo] = ex.ID
	return nil
}

func (f *fakeRepo) FindPayException(_ context.Context, id uint) (*entity.PayException, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	ex, ok := f.exceptions[id]
	if !ok {
		return nil, errors.New("pay exception not found")
	}
	cp := *ex
	return &cp, nil
}

func (f *fakeRepo) FindPayExceptionByTradeNo(_ context.Context, tradeNo string) (*entity.PayException, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id, ok := f.exByTrade[tradeNo]
	if !ok {
		return nil, nil
	}
	cp := *f.exceptions[id]
	return &cp, nil
}

func (f *fakeRepo) UpdatePayException(_ context.Context, ex *entity.PayException) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *ex
	f.exceptions[ex.ID] = &cp
	return nil
}

func (f *fakeRepo) ListPayExceptions(_ context.Context, status string) ([]*entity.PayException, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var items []*entity.PayException
	for _, ex := range f.exceptions {
		if status == "" || ex.Status == status {
			cp := *ex
			items = append(items, &cp)
		}
	}
	return items, nil
}

func (f *fakeRepo) Summary(_ context.Context) (int64, int64, map[string]int64, int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var paid, refunded int64
	counts := map[string]int64{}
	for _, o := range f.orders {
		counts[o.Status]++
		if o.Status == entity.StatusRefunded {
			refunded += o.AmountCents()
		}
		if o.ChannelTradeNo != "" && o.Status != entity.StatusCancelled && o.Status != entity.StatusClosedTimeout {
			paid += o.AmountCents()
		}
	}
	var exceptionCount int64
	for _, ex := range f.exceptions {
		if ex.Status == entity.ExceptionRefunded {
			refunded += ex.AmountCents
		}
		exceptionCount++
	}
	return paid, refunded, counts, exceptionCount, nil
}

// fakeChannel 是模拟支付渠道的测试替身；succeedFrom 控制第 N 次退款起成功。
type fakeChannel struct {
	payErr      error
	payTrade    string
	succeedFrom int
	attempts    map[string]int
	refunds     []string
}

func (c *fakeChannel) attemptsFor(refundNo string) int {
	if c.attempts == nil {
		c.attempts = map[string]int{}
	}
	c.attempts[refundNo]++
	return c.attempts[refundNo]
}

func (c *fakeChannel) Pay(_ context.Context, orderNo string, _ string, _ int64) (string, error) {
	if c.payErr != nil {
		return "", c.payErr
	}
	if c.payTrade != "" {
		return c.payTrade, nil
	}
	return "CH-" + orderNo, nil
}

func (c *fakeChannel) Refund(_ context.Context, refundNo string, _ string, _ int64, _ string) (bool, string, error) {
	c.refunds = append(c.refunds, refundNo)
	from := c.succeedFrom
	if from <= 0 {
		from = 1
	}
	if c.attemptsFor(refundNo) >= from {
		return true, "success", nil
	}
	return false, "channel busy", nil
}

// fakeStock 是商品服务的测试替身，记录调用次数。
type fakeStock struct {
	reserves   int
	releases   int
	confirms   int
	restores   int
	reserveErr error
}

func (s *fakeStock) Reserve(_ context.Context, _ string, _ string, _ int64) error {
	s.reserves++
	return s.reserveErr
}

func (s *fakeStock) Release(_ context.Context, _ string) error { s.releases++; return nil }

func (s *fakeStock) Confirm(_ context.Context, _ string) error { s.confirms++; return nil }

func (s *fakeStock) Restore(_ context.Context, _ string) error { s.restores++; return nil }

func (s *fakeStock) GetProduct(_ context.Context, _ string) (string, int64, error) {
	return "限量商品", 10000, nil
}

func newTestService(repo *fakeRepo, channel ChannelClient, stock StockClient) *Service {
	return NewService(repo, channel, stock, 10*time.Minute, "test-secret", nil)
}

func ctx() context.Context { return context.Background() }
