package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"github.com/flxxyz/doh/handler"
	"github.com/flxxyz/doh/ticket"
	"golang.org/x/crypto/acme/autocert"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/flxxyz/doh/payment"
	"github.com/quic-go/quic-go/http3"
)

type Params struct {
	TlsCert          string
	TlsKey           string
	TlsHosts         string
	H12              string
	H3               string
	DnsUpstreams     string
	DbType           string
	DbUri            string
	PaymentPrice     int
	PaymentFree      bool
	WebRoot          string
	AlipayAppId      string
	AlipayPrivateKey string
	AlipayPublicKey  string
}

var (
	params = &Params{}
)

func listen() (lnH12 net.Listener, lnH3 net.PacketConn, err error) {
	if os.Getenv("LISTEN_PID") == strconv.Itoa(os.Getpid()) {
		if os.Getenv("LISTEN_FDS") != "2" {
			panic("LISTEN_FDS should be 2")
		}
		names := strings.Split(os.Getenv("LISTEN_FDNAMES"), ":")
		for i, name := range names {
			switch name {
			case "h12":
				f := os.NewFile(uintptr(i+3), "https port")
				lnH12, err = net.FileListener(f)
			case "h3":
				f := os.NewFile(uintptr(i+3), "quic port")
				lnH3, err = net.FilePacketConn(f)
			}
		}
	} else {
		if params.H12 != "" {
			lnH12, err = net.Listen("tcp", params.H12)
			if err != nil {
				return
			}
		}
		if params.H3 != "" {
			lnH3, err = net.ListenPacket("udp", params.H3)
		}
	}
	return
}

func init() {
	flag.StringVar(&params.TlsCert, "tls-cert", "", "File path of TLS certificate")
	flag.StringVar(&params.TlsKey, "tls-key", "", "File path of TLS key")
	flag.StringVar(&params.TlsHosts, "tls-hosts", "", "Host name for ACME")
	flag.StringVar(&params.H12, "h12", ":443", "Listen address for http1 and h2")
	flag.StringVar(&params.H3, "h3", ":443", "Listen address for h3")
	flag.StringVar(&params.DnsUpstreams, "upstreams", "https://doh.pub/dns-query,https://dns.alidns.com/dns-query,https://doh.360.cn/dns-query", "DoH upstream URL")
	flag.StringVar(&params.DbType, "dbtype", "sqlite", "Database Type")
	flag.StringVar(&params.DbUri, "db", "", "File path of Sqlite; DSN of Mysql or Postgres")
	flag.StringVar(&params.WebRoot, "root", ".", "Root path of static files")
	flag.IntVar(&params.PaymentPrice, "price", 1024, "Traffic price MB/Yuan")
	flag.BoolVar(&params.PaymentFree, "free", false, `Whether allow free access.
If not free, you should set the following environment variables:
	- ALIPAY_APP_ID
	- ALIPAY_PRIVATE_KEY
	- ALIPAY_PUBLIC_KEY
`)

	flag.Parse()

	if v := os.Getenv("TLS_CERT"); v != "" {
		params.TlsCert = v
	}
	if v := os.Getenv("TLS_KEY"); v != "" {
		params.TlsKey = v
	}
	if v := os.Getenv("TLS_HOSTS"); v != "" {
		params.TlsHosts = v
	}
	if v := os.Getenv("LISTEN_HTTP"); v != "" {
		params.H12 = v
	}
	if v := os.Getenv("LISTEN_H3"); v != "" {
		params.H3 = v
	}
	if v := os.Getenv("DNS_UPSTREAMS"); v != "" {
		params.DnsUpstreams = v
	}
	if v := os.Getenv("DB_TYPE"); v != "" {
		params.DbType = v
	}
	if v := os.Getenv("DB_URI"); v != "" {
		params.DbUri = v
	}
	if v := os.Getenv("WEB_ROOT"); v != "" {
		params.WebRoot = v
	}
	if v := os.Getenv("PAYMENT_PRICE"); v != "" {
		p, err := strconv.Atoi(v)
		if err != nil {
			panic(err)
		}
		params.PaymentPrice = p
	}
	if v := os.Getenv("PAYMENT_FREE"); v != "" {
		f, err := strconv.ParseBool(v)
		if err != nil {
			panic(err)
		}
		params.PaymentFree = f
	}
	if v := os.Getenv("ALIPAY_APP_ID"); v != "" {
		params.AlipayAppId = v
	}
	if v := os.Getenv("ALIPAY_PRIVATE_KEY"); v != "" {
		params.AlipayPrivateKey = v
	}
	if v := os.Getenv("ALIPAY_PUBLIC_KEY"); v != "" {
		params.AlipayPublicKey = v
	}
}

func main() {
	var tlsCfg *tls.Config
	// Load TLS certificate
	if params.TlsCert != "" && params.TlsKey != "" {
		tlsCfg = &tls.Config{}
		certs, err := tls.LoadX509KeyPair(params.TlsCert, params.TlsKey)
		if err != nil {
			panic(err)
		}
		tlsCfg.Certificates = []tls.Certificate{certs}
	} else {
		if params.TlsHosts != "" {
			acm := autocert.Manager{
				Prompt:     autocert.AcceptTOS,
				Cache:      autocert.DirCache(os.Getenv("HOME") + "/.autocert"),
				HostPolicy: autocert.HostWhitelist(strings.Split(params.TlsHosts, ",")...),
			}

			tlsCfg = acm.TLSConfig()
		}
	}

	lnH12, lnH3, err := listen()
	if err != nil {
		panic(err)
	}

	var pay payment.Pay
	var taxCollector ticket.TaxCollector
	if params.PaymentFree {
		taxCollector = ticket.FreeRepo{}
	} else {
		taxCollector = ticket.NewTaxCollector(params.DbType, params.DbUri)
		pay = payment.NewPay(
			params.AlipayAppId,
			params.AlipayPrivateKey,
			params.AlipayPublicKey,
		)
	}

	h := &handler.DoHHandler{
		Upstreams:    strings.Split(params.DnsUpstreams, ","),
		TaxCollector: taxCollector,
	}
	th := &handler.TicketHandler{
		MBpCNY:       params.PaymentPrice,
		Pay:          pay,
		TaxCollector: taxCollector,
	}

	mux := http.NewServeMux()
	mux.Handle("/dns-query/{token}", h)
	mux.Handle("/ticket/", th)
	mux.Handle("/ticket/{token}", th)
	mux.Handle("/", http.FileServer(http.Dir(params.WebRoot)))

	if tlsCfg != nil {
		if lnH3 != nil {
			go func() {
				defer func() {
					if err := recover(); err != nil {
						log.Printf("quic exception, err=(%s)\n", err)
						os.Exit(1)
					}
				}()

				p := lnH3.LocalAddr().(*net.UDPAddr).Port
				h.AltSvc = fmt.Sprintf(`h3=":%d"`, p)
				th.AltSvc = h.AltSvc

				h3 := http3.Server{Handler: mux, TLSConfig: tlsCfg}
				log.Printf("quic listen on %s\n", lnH3.LocalAddr())
				if err := h3.Serve(lnH3); err != nil {
					log.Fatal(err)
				}
			}()
		}

		lnTLS := tls.NewListener(lnH12, tlsCfg)
		log.Printf("https listen on %s\n", lnTLS.Addr())
		if err = http.Serve(lnTLS, mux); err != nil {
			log.Fatal(err)
		}

		return
	}

	log.Printf("http1/2 listen on %s\n", lnH12.Addr())
	if err = http.Serve(lnH12, mux); err != nil {
		log.Fatal(err)
	}
}
