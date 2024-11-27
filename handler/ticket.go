package handler

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"github.com/flxxyz/doh/payment"
	"github.com/flxxyz/doh/ticket"
	"net/http"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type TicketHandler struct {
	MBpCNY       int
	Pay          payment.Pay
	TaxCollector ticket.TaxCollector
	AltSvc       string
}

func (h *TicketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.AltSvc != "" {
		w.Header().Set("Alt-Svc", h.AltSvc)
	}

	if r.Method == http.MethodGet {
		token := r.PathValue("token")
		ts, err := h.TaxCollector.List(token, 10)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Add("content-type", "application/json")
		json.NewEncoder(w).Encode(ts)
		return
	}

	if r.URL.Query().Get("buy") != "" {
		req := struct {
			Token string `json:"token"`
			Cents int    `json:"cents"`
		}{}
		defer r.Body.Close()
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if req.Cents < 10 {
			http.Error(w, "cents must > 10", http.StatusBadRequest)
			return
		}

		if req.Token == "" {
			b := make([]byte, 16)
			_, err := rand.Read(b)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			req.Token = base64.RawURLEncoding.EncodeToString(b)
		}

		now := time.Now().Format(time.RFC3339)
		yuan := strconv.FormatFloat(float64(req.Cents)/100, 'f', 2, 64)
		o := payment.Order{
			OrderNo: req.Token + "@" + now,
			Amount:  yuan,
		}

		notify := "https://" + r.Host + r.URL.Path
		qr, err := h.Pay.NewQR(o, notify)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Add("content-type", "application/json")
		json.NewEncoder(w).Encode(struct {
			QR    string `json:"qr"`
			Token string `json:"token"`
			Order string `json:"order"`
		}{QR: qr, Token: req.Token, Order: o.OrderNo})
	} else {
		o, err := h.Pay.OnPay(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		i := strings.Index(o.OrderNo, "@")
		token := o.OrderNo[:i]

		yuan, err := strconv.ParseFloat(o.Amount, 64)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		bytes := int(yuan * float64(h.MBpCNY) * 1024 * 1024)

		err = h.TaxCollector.New(token, bytes, o.OrderNo, o.TradeNo)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Write([]byte("success"))
	}
}
