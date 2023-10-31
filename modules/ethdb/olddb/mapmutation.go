package olddb

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/amazechain/amc/modules"
	"github.com/ledgerwatch/erigon-lib/etl"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/log/v3"
	"sync"
	"time"
	"unsafe"
)

type Mapmutation struct {
	puts   map[string]map[string][]byte // table -> key -> value ie. blocks -> hash -> blockBod
	db     kv.Tx
	quit   <-chan struct{}
	clean  func()
	mu     sync.RWMutex
	size   int
	count  uint64
	tmpdir string
}

func (m *Mapmutation) BucketSize(table string) (uint64, error) {
	panic("not used")
}

// NewBatch - starts in-mem batch
//
// Common pattern:
//
// batch := db.NewBatch()
// defer batch.Close()
// ... some calculations on `batch`
// batch.Commit()
func NewHashBatch(tx kv.Tx, quit <-chan struct{}, tmpdir string) *Mapmutation {
	clean := func() {}
	if quit == nil {
		ch := make(chan struct{})
		clean = func() { close(ch) }
		quit = ch
	}

	return &Mapmutation{
		db:     tx,
		puts:   make(map[string]map[string][]byte),
		quit:   quit,
		clean:  clean,
		tmpdir: tmpdir,
	}
}

func (m *Mapmutation) getMem(table string, key []byte) ([]byte, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if _, ok := m.puts[table]; !ok {
		return nil, false
	}
	if value, ok := m.puts[table][*(*string)(unsafe.Pointer(&key))]; ok {
		return value, ok
	}

	return nil, false
}

func (m *Mapmutation) IncrementSequence(bucket string, amount uint64) (res uint64, err error) {
	v, ok := m.getMem(modules.Sequence, []byte(bucket))
	if !ok && m.db != nil {
		v, err = m.db.GetOne(modules.Sequence, []byte(bucket))
		if err != nil {
			return 0, err
		}
	}

	var currentV uint64 = 0
	if len(v) > 0 {
		currentV = binary.BigEndian.Uint64(v)
	}

	newVBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(newVBytes, currentV+amount)
	if err = m.Put(modules.Sequence, []byte(bucket), newVBytes); err != nil {
		return 0, err
	}

	return currentV, nil
}
func (m *Mapmutation) ReadSequence(bucket string) (res uint64, err error) {
	v, ok := m.getMem(modules.Sequence, []byte(bucket))
	if !ok && m.db != nil {
		v, err = m.db.GetOne(modules.Sequence, []byte(bucket))
		if err != nil {
			return 0, err
		}
	}
	var currentV uint64 = 0
	if len(v) > 0 {
		currentV = binary.BigEndian.Uint64(v)
	}

	return currentV, nil
}

// Can only be called from the worker thread
func (m *Mapmutation) GetOne(table string, key []byte) ([]byte, error) {
	if value, ok := m.getMem(table, key); ok {
		return value, nil
	}
	if m.db != nil {
		// TODO: simplify when tx can no longer be parent of mutation
		value, err := m.db.GetOne(table, key)
		if err != nil {
			return nil, err
		}
		return value, nil
	}
	return nil, nil
}

func (m *Mapmutation) Last(table string) ([]byte, []byte, error) {
	c, err := m.db.Cursor(table)
	if err != nil {
		return nil, nil, err
	}
	defer c.Close()
	return c.Last()
}

func (m *Mapmutation) Has(table string, key []byte) (bool, error) {
	if _, ok := m.getMem(table, key); ok {
		return ok, nil
	}
	if m.db != nil {
		return m.db.Has(table, key)
	}
	return false, nil
}

// puts a table key with a value and if the table is not found then it appends a table
func (m *Mapmutation) Put(table string, k, v []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.puts[table]; !ok {
		m.puts[table] = make(map[string][]byte)
	}

	stringKey := string(k)

	var ok bool
	if _, ok = m.puts[table][stringKey]; !ok {
		m.size += len(v) - len(m.puts[table][stringKey])
		m.puts[table][stringKey] = v
		return nil
	}
	m.puts[table][stringKey] = v
	m.size += len(k) + len(v)
	m.count++

	return nil
}

func (m *Mapmutation) Append(table string, key []byte, value []byte) error {
	return m.Put(table, key, value)
}

func (m *Mapmutation) AppendDup(table string, key []byte, value []byte) error {
	return m.Put(table, key, value)
}

func (m *Mapmutation) BatchSize() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.size
}

func (m *Mapmutation) ForEach(bucket string, fromPrefix []byte, walker func(k, v []byte) error) error {
	m.panicOnEmptyDB()
	return m.db.ForEach(bucket, fromPrefix, walker)
}

func (m *Mapmutation) ForPrefix(bucket string, prefix []byte, walker func(k, v []byte) error) error {
	m.panicOnEmptyDB()
	return m.db.ForPrefix(bucket, prefix, walker)
}

func (m *Mapmutation) ForAmount(bucket string, prefix []byte, amount uint32, walker func(k, v []byte) error) error {
	m.panicOnEmptyDB()
	return m.db.ForAmount(bucket, prefix, amount, walker)
}

func (m *Mapmutation) Delete(table string, k []byte) error {
	return m.Put(table, k, nil)
}

func (m *Mapmutation) doCommit(tx kv.RwTx) error {
	logEvery := time.NewTicker(30 * time.Second)
	defer logEvery.Stop()
	count := 0
	total := float64(m.count)
	for table, bucket := range m.puts {
		l := log.New()
		collector := etl.NewCollector("", m.tmpdir, etl.NewSortableBuffer(etl.BufferOptimalSize), l)
		defer collector.Close()
		for key, value := range bucket {
			collector.Collect([]byte(key), value)
			count++
			select {
			default:
			case <-logEvery.C:
				progress := fmt.Sprintf("%.1fM/%.1fM", float64(count)/1_000_000, total/1_000_000)
				l.Info("Write to db", "progress", progress, "current table", table)
				tx.CollectMetrics()
			}
		}
		if err := collector.Load(tx, table, etl.IdentityLoadFunc, etl.TransformArgs{Quit: m.quit}); err != nil {
			return err
		}
	}

	tx.CollectMetrics()
	return nil
}

func (m *Mapmutation) Flush(ctx context.Context, tx kv.RwTx) error {
	if tx == nil {
		return errors.New("rwTx needed")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.doCommit(tx); err != nil {
		return err
	}

	m.puts = map[string]map[string][]byte{}
	m.size = 0
	m.count = 0
	return nil
}

func (m *Mapmutation) Close() {
	if m.clean == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.puts = map[string]map[string][]byte{}
	m.size = 0
	m.count = 0
	m.size = 0
	m.clean()
	m.clean = nil
}
func (m *Mapmutation) Commit() error { panic("not db txn, use .Flush method") }
func (m *Mapmutation) Rollback()     { panic("not db txn, use .Close method") }

func (m *Mapmutation) panicOnEmptyDB() {
	if m.db == nil {
		panic("Not implemented")
	}
}

func (m *Mapmutation) ResetTx(tx kv.Tx) {
	if m.db != nil {
		m.db.Rollback()
	}

	m.db = tx
}
