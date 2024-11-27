package ticket

import (
	"github.com/go-kiss/sqlx"
	"time"
)

type Ticket struct {
	ID         int    `db:"id" json:"id"`
	Token      string `db:"token" json:"-"`
	Bytes      int    `db:"bytes" json:"bytes"`
	TotalBytes int    `db:"total_bytes" json:"total_bytes"`
	PayOrder   string `db:"pay_order" json:"pay_order"`
	BuyOrder   string `db:"buy_order" json:"buy_order"`

	Created time.Time `db:"created" json:"created"`
	Updated time.Time `db:"updated" json:"updated"`
	Expires time.Time `db:"expires" json:"expires"`
}

func (_ *Ticket) KeyName() string   { return "id" }
func (_ *Ticket) TableName() string { return "tickets" }
func (t *Ticket) Schema() string {
	return "CREATE TABLE IF NOT EXISTS " + t.TableName() + `(
	` + t.KeyName() + ` INTEGER PRIMARY KEY AUTOINCREMENT,
	token TEXT,
	bytes INTEGER,
	total_bytes INTEGER,
	pay_order TEXT,
	buy_order TEXT,
	created DATETIME,
	updated DATETIME,
	expires DATETIME
);
	CREATE INDEX IF NOT EXISTS t_token_expires ON ` + t.TableName() + `(token, expires);
	CREATE UNIQUE INDEX IF NOT EXISTS t_pay_order ON ` + t.TableName() + `(pay_order);`
}

type TaxCollector interface {
	// New create and save one Ticket
	New(token string, bytes int, trade string, order string) error
	// Cost decreases  bytes of one Ticket
	Cost(token string, bytes int) error
	// List fetches all current Tickets with bytes available.
	List(token string, limit int) ([]Ticket, error)
}

func NewTaxCollector(driver, path string) TaxCollector {
	db, err := sqlx.Connect(driver, path+"?cache=shared&mode=rwc&_journal_mode=WAL")
	if err != nil {
		panic(err)
	}
	r := PayRepo{db: db}
	r.Init()
	return r
}
