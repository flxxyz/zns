package ticket

type FreeRepo struct{}

func (r FreeRepo) New(token string, bytes int, trade, order string) error {
	return nil
}

func (r FreeRepo) Cost(token string, bytes int) error {
	return nil
}

func (r FreeRepo) List(token string, limit int) ([]Ticket, error) {
	return []Ticket{{Bytes: 100}}, nil
}
