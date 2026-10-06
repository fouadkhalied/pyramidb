package db

func (db *DB) FlushMemTable() error {
	return nil
}

func (db *DB) DeleteWal() error {
	// wal should delete this
	return nil
}
