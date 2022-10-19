package usage

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
	"go.opencensus.io/metric/metricdata"
	"go.opencensus.io/metric/metricexport"
)

// Options provides options for LogExporter
type Options struct {
	// ReportingInterval is a time interval between two successive metrics
	// export.
	ReportingInterval time.Duration
}

type UsageDB struct {
	ID        string    `ch:"id"`
	TimeStamp time.Time `ch:"timestamp"`
	Tenant    string    `ch:"tenant"`
	Data      string    `ch:"data"`
}

type Usage struct {
	Tenant string
	Count  int64
	Size   int64
}

type UsageCollector struct {
	id             string
	reader         *metricexport.Reader
	ir             *metricexport.IntervalReader
	initReaderOnce sync.Once
	o              Options
	db             clickhouse.Conn
	dbName         string
	tableName      string
	usageParser    func(metrics []*metricdata.Metric) (Usage, error)
	prevCount      int64
	prevSize       int64
}

func NewUsageCollector(db clickhouse.Conn, options Options, dbName string, tableName string, usageParser func(metrics []*metricdata.Metric) (Usage, error)) *UsageCollector {
	return &UsageCollector{
		id:          uuid.NewString(),
		reader:      metricexport.NewReader(),
		o:           options,
		db:          db,
		dbName:      dbName,
		tableName:   tableName,
		usageParser: usageParser,
		prevCount:   0,
		prevSize:    0,
	}
}
func (e *UsageCollector) CreateTable(db clickhouse.Conn, databaseName string) error {
	query := fmt.Sprintf(
		`
		CREATE TABLE IF NOT EXISTS %s.usage (
			id String,
			timestamp DateTime,
			tenant String,
			data String
		) ENGINE MergeTree()
		ORDER BY (id);
		`,
		databaseName,
	)

	err := db.Exec(context.Background(), query)
	return err
}

func (e *UsageCollector) Start() error {
	// create table if not exists
	err := e.CreateTable(e.db, e.dbName)
	if err != nil {
		return err
	}

	// start collector routine which
	e.initReaderOnce.Do(func() {
		e.ir, _ = metricexport.NewIntervalReader(&metricexport.Reader{}, e)
	})
	e.ir.ReportingInterval = e.o.ReportingInterval
	return e.ir.Start()
}

func (c *UsageCollector) Stop() error {
	c.ir.Stop()
	return nil
}

func (e *UsageCollector) ExportMetrics(ctx context.Context, metrics []*metricdata.Metric) error {
	usage, err := e.usageParser(metrics)
	if err != nil {
		return err
	}

	// dont update if value hasn't changed
	if usage.Count == e.prevCount && usage.Size == e.prevSize {
		return nil
	}

	usageBytes, err := json.Marshal(usage)
	if err != nil {
		return err
	}

	encryptedData, err := Encrypt([]byte(e.id)[:32], usageBytes)
	if err != nil {
		return err
	}

	prevUsages := []UsageDB{}
	err = e.db.Select(ctx, &prevUsages, fmt.Sprintf("SELECT * FROM %s.usage where id='%s'", e.dbName, e.id))
	if err != nil {
		return err
	}

	if len(prevUsages) > 0 {
		// already exist then update
		err := e.db.Exec(ctx, fmt.Sprintf("alter table %s.usage update timestamp=$1, data=$2 where id=$3", e.dbName), time.Now(), string(encryptedData), e.id)
		if err != nil {
			return err
		}
	} else {
		err := e.db.Exec(ctx, fmt.Sprintf("insert into %s.usage values ($1, $2, $3, $4)", e.dbName), e.id, time.Now(), "", string(encryptedData))
		if err != nil {
			return err
		}
	}

	e.prevCount = usage.Count
	e.prevSize = usage.Size

	return nil
}

func Encrypt(key, text []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	b := base64.StdEncoding.EncodeToString(text)
	ciphertext := make([]byte, aes.BlockSize+len(b))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}
	cfb := cipher.NewCFBEncrypter(block, iv)
	cfb.XORKeyStream(ciphertext[aes.BlockSize:], []byte(b))
	return ciphertext, nil
}
