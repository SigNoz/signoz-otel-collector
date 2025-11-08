package clickhouse

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/AfterShip/clickhouse-sql-parser/parser"
	"github.com/ClickHouse/clickhouse-go/v2"
	"go.uber.org/zap"
)

type TTLResponse struct {
	EngineFull string `ch:"engine_full"`
}

type TTLParser struct {
	logger *zap.Logger
}

func NewTTLParser(logger *zap.Logger) *TTLParser {
	return &TTLParser{
		logger: logger,
	}
}

func (p *TTLParser) GetTableTTLDays(ctx context.Context, conn clickhouse.Conn, database, tableName string) uint64 {
	if conn == nil {
		p.logger.Debug("no database connection available, cannot determine TTL")
		return 0
	}

	var dbResp []TTLResponse
	query := fmt.Sprintf("SELECT engine_full FROM system.tables WHERE database = '%s' AND name = '%s'", database, tableName)

	err := conn.Select(ctx, &dbResp, query)
	if err != nil {
		p.logger.Error("error while fetching ttl from database",
			zap.Error(err), zap.String("database", database), zap.String("table", tableName))
		return 0
	}

	if len(dbResp) == 0 {
		p.logger.Warn("table not found in system.tables", zap.String("database", database), zap.String("table", tableName))
		return 0
	}

	ttlDays := p.ParseTTLFromEngineFull(dbResp[0].EngineFull)
	if ttlDays > 0 {
		p.logger.Info("TTL configured for table",
			zap.String("database", database), zap.String("table", tableName), zap.Uint64("ttl_days", ttlDays))
		return ttlDays
	}

	p.logger.Info("TTL not configured for table", zap.String("database", database), zap.String("table", tableName))
	return 0
}

func (p *TTLParser) ParseTTLFromEngineFull(engineFull string) uint64 {
	// engine_full for parsing
	sqlStatement := "CREATE TABLE dummy_and_dumb (id Int32) ENGINE = " + engineFull

	clickhouseParser := parser.NewParser(sqlStatement)
	stmts, err := clickhouseParser.ParseStmts()
	if err != nil {
		p.logger.Debug("failed to parse engine_full string",
			zap.Error(err),
			zap.String("engine_full", engineFull))
		return 0
	}

	if len(stmts) == 0 {
		p.logger.Debug("no statements found in engine_full string",
			zap.String("engine_full", engineFull))
		return 0
	}

	for _, stmt := range stmts {
		if createTable, ok := stmt.(*parser.CreateTable); ok {
			if createTable.Engine != nil && createTable.Engine.TTL != nil {
				p.logger.Debug("TTL clause found in engine_full string",
					zap.String("engine_full", engineFull))
				return p.extractTTLFromClause(createTable.Engine.TTL)
			}
		}
	}

	return 0
}

func (p *TTLParser) extractTTLFromClause(ttlClause *parser.TTLClause) uint64 {
	if ttlClause == nil || len(ttlClause.Items) == 0 {
		return 0
	}

	ttlExpr := ttlClause.Items[0]
	if ttlExpr == nil || ttlExpr.Expr == nil {
		return 0
	}

	exprStr := ttlExpr.Expr.String()
	return p.extractIntervalFromString(exprStr)
}

func (p *TTLParser) extractIntervalFromString(exprStr string) uint64 {
	// toIntervalXXX functions in the expression
	intervalFunctions := []struct {
		funcName     string
		intervalType string
	}{
		{"toIntervalSecond", "Second"},
		{"toIntervalMinute", "Minute"},
		{"toIntervalHour", "Hour"},
		{"toIntervalDay", "Day"},
		{"toIntervalWeek", "Week"},
		{"toIntervalMonth", "Month"},
		{"toIntervalYear", "Year"},
	}

	for _, intervalFunc := range intervalFunctions {
		if strings.Contains(exprStr, intervalFunc.funcName) {
			if value := p.extractNumericValueFromFunction(exprStr, intervalFunc.funcName); value > 0 {
				return p.convertToDays(intervalFunc.intervalType, value)
			}
		}
	}

	return 0
}

func (p *TTLParser) extractNumericValueFromFunction(exprStr, funcName string) uint64 {
	// dumb string manipulation
	startIdx := strings.Index(exprStr, funcName+"(")
	if startIdx == -1 {
		return 0
	}

	startIdx += len(funcName) + 1
	endIdx := strings.Index(exprStr[startIdx:], ")")
	if endIdx == -1 {
		return 0
	}

	paramStr := strings.TrimSpace(exprStr[startIdx : startIdx+endIdx])
	if value, err := strconv.ParseUint(paramStr, 10, 64); err == nil {
		return value
	}

	return 0
}

func (p *TTLParser) convertToDays(intervalType string, value uint64) uint64 {
	if value == 0 {
		return 0
	}

	switch intervalType {
	case "Second":
		return value / (24 * 60 * 60)
	case "Minute":
		return value / (24 * 60)
	case "Hour":
		return value / 24
	case "Day":
		return value
	case "Week":
		return value * 7
	case "Month":
		return value * 30
	case "Year":
		return value * 365
	default:
		return 0
	}
}
