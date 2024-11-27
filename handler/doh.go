package handler

import (
	"bytes"
	"encoding/base64"
	"errors"
	"github.com/flxxyz/doh/ticket"
	"io"
	"net/http"
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type Answer struct {
	Result  []byte
	Latency int64
}

// Query sends a DNS query to the upstreams and returns the fastest answer.
func Query(upstreams []string, question []byte) (*Answer, error) {
	if len(upstreams) == 0 {
		return nil, errors.New("no upstream")
	}

	mu := sync.Mutex{}
	wg := sync.WaitGroup{}
	wg.Add(len(upstreams))
	answers := make([]*Answer, 0, len(upstreams))
	for _, upstream := range upstreams {
		go func() {
			defer wg.Done()
			stime := time.Now()
			resp, err := http.Post(upstream, "application/dns-message", bytes.NewReader(question))
			if err != nil {
				return
			}
			defer resp.Body.Close()

			answer, err := io.ReadAll(resp.Body)
			if err != nil {
				return
			}

			defer func() {
				latency := time.Now().Sub(stime).Milliseconds()
				mu.Lock()
				defer mu.Unlock()
				answers = append(answers, &Answer{Result: answer, Latency: latency})
			}()
		}()
	}
	wg.Wait()

	if len(answers) == 0 {
		return nil, errors.New("no answer")
	}

	answer := slices.MinFunc(answers, func(i, j *Answer) int {
		if i.Latency < j.Latency {
			return -1
		} else if i.Latency > j.Latency {
			return 1
		}
		return 0
	})

	return answer, nil
}

type DoHHandler struct {
	Upstreams    []string
	TaxCollector ticket.TaxCollector
	AltSvc       string
}

func (h *DoHHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.AltSvc != "" {
		w.Header().Set("Alt-Svc", h.AltSvc)
	}

	token := r.PathValue("token")
	if token == "" {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	ts, err := h.TaxCollector.List(token, 1)
	if err != nil {
		http.Error(w, "invalid token", http.StatusInternalServerError)
		return
	}
	if len(ts) == 0 || ts[0].Bytes <= 0 {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	var question []byte
	if r.Method == http.MethodGet {
		q := r.URL.Query().Get("dns")
		question, err = base64.RawURLEncoding.DecodeString(q)
	} else {
		question, err = io.ReadAll(r.Body)
		r.Body.Close()
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var m dns.Msg
	if err := m.Unpack(question); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var hasSubnet bool
	if e := m.IsEdns0(); e != nil {
		for _, o := range e.Option {
			if o.Option() == dns.EDNS0SUBNET {
				a := o.(*dns.EDNS0_SUBNET).Address[:2]
				// skip empty subnet like 0.0.0.0/0
				if !bytes.HasPrefix(a, []byte{0, 0}) {
					hasSubnet = true
				}
				break
			}
		}
	}

	if !hasSubnet {
		ip, err := netip.ParseAddrPort(r.RemoteAddr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		addr := ip.Addr()
		opt := &dns.OPT{Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT}}
		ecs := &dns.EDNS0_SUBNET{Code: dns.EDNS0SUBNET}
		var bits int
		if addr.Is4() {
			bits = 24
			ecs.Family = 1
		} else {
			bits = 48
			ecs.Family = 2
		}
		ecs.SourceNetmask = uint8(bits)
		p := netip.PrefixFrom(addr, bits)
		ecs.Address = p.Masked().Addr().AsSlice()
		opt.Option = append(opt.Option, ecs)
		m.Extra = []dns.RR{opt}
	}

	if question, err = m.Pack(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	answer, err := Query(h.Upstreams, question)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = h.TaxCollector.Cost(token, len(question)+len(answer.Result)); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	w.Header().Add("content-type", "application/dns-message")
	w.Write(answer.Result)
}
