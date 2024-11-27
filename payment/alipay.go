package payment

import (
	"fmt"
	"github.com/smartwalle/alipay/v3"
	"net/http"
)

type aliPay struct {
	ali *alipay.Client
}

func (p aliPay) OnPay(req *http.Request) (o Order, err error) {
	n, err := p.ali.GetTradeNotification(req)
	if err != nil {
		return
	}

	o.OrderNo = n.OutTradeNo
	o.TradeNo = n.TradeNo
	o.Amount = n.ReceiptAmount

	return
}

func (p aliPay) NewQR(order Order, notifyURL string) (string, error) {
	r, err := p.ali.TradePreCreate(alipay.TradePreCreate{
		Trade: alipay.Trade{
			NotifyURL:      notifyURL,
			Subject:        "DoH Ticket",
			OutTradeNo:     order.OrderNo,
			TotalAmount:    order.Amount,
			TimeoutExpress: "15m",
		},
	})
	if err != nil {
		return "", err
	}

	if r.Code != alipay.CodeSuccess {
		return "", fmt.Errorf("TradePreCreate error: %w", err)
	}

	return r.QRCode, nil
}
