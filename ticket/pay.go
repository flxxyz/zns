package ticket

import (
	"errors"
	"github.com/go-kiss/sqlx"
	"modernc.org/sqlite"
	"time"
)

type PayRepo struct {
	db *sqlx.DB
}

func (r PayRepo) Init() {
	if _, err := r.db.Exec((*Ticket).Schema(nil)); err != nil {
		panic(err)
	}
}

func expires(t time.Time) time.Time {
	t = t.AddDate(0, 1, -t.Day()+1)
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

func (r PayRepo) New(token string, bytes int, trade, order string) error {
	now := time.Now()

	t := Ticket{
		Token:      token,
		Bytes:      bytes,
		TotalBytes: bytes,
		PayOrder:   order,
		BuyOrder:   trade,
		Created:    now,
		Updated:    now,
		Expires:    expires(now),
	}

	_, err := r.db.Insert(&t)

	se := &sqlite.Error{}
	// constraint failed: UNIQUE constraint failed
	if errors.As(err, &se) && se.Code() == 2067 {
		return nil
	}

	return err
}

func (r PayRepo) Cost(token string, bytes int) error {
	now := time.Now()

	sql := "update " + (*Ticket).TableName(nil) +
		" set bytes = bytes - ? where id in (select id from " + (*Ticket).TableName(nil) +
		" where token = ? and expires > ? order by id asc limit 1) and bytes >= ?"

	_r, err := r.db.Exec(sql, bytes, token, now, bytes)
	if err != nil {
		return err
	}
	n, err := _r.RowsAffected()
	if err != nil {
		return err
	}
	if n == 1 {
		return nil
	}

	return r.costSlow(token, bytes)
}

func (r PayRepo) costSlow(token string, bytes int) error {
	sql := "select * from " + (*Ticket).TableName(nil) +
		" where token = ? and bytes > 0 and expires > ?" +
		" order by id asc"
	var ts []Ticket
	if err := r.db.Select(&ts, sql, token, time.Now()); err != nil {
		return err
	}

	var i int
	var t Ticket
	for i, t = range ts {
		if t.Bytes >= bytes {
			ts[i].Bytes -= bytes
			bytes = 0
			break
		} else {
			bytes -= t.Bytes
			ts[i].Bytes = 0
		}
	}

	if bytes > 0 {
		ts[i].Bytes -= bytes
	}

	if i == 0 {
		t := ts[i]
		t.Updated = time.Now()
		_, err := r.db.Update(&t)
		return err
	}

	tx, err := r.db.Beginx()
	if err != nil {
		return err
	}

	for ; i >= 0; i-- {
		t := ts[i]
		t.Updated = time.Now()
		_, err := tx.Update(&t)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (r PayRepo) List(token string, limit int) (tickets []Ticket, err error) {
	sql := "select * from " + (*Ticket).TableName(nil) +
		" where token = ? order by id desc limit ?"
	err = r.db.Select(&tickets, sql, token, limit)
	return
}
