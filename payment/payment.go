package payment

import (
	"github.com/smartwalle/alipay/v3"
	"net/http"
)

type Pay interface {
	NewQR(order Order, notifyURL string) (string, error)
	OnPay(req *http.Request) (Order, error)
}

func NewPay(appID, privateKey, publicKey string) Pay {
	client, err := alipay.New(appID, privateKey, true)
	if err != nil {
		panic(err)
	}

	if err = client.LoadAliPayPublicKey(publicKey); err != nil {
		panic(err)
	}

	return aliPay{ali: client}
}
