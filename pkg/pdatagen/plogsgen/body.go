package plogsgen

import (
	"fmt"
	"math/rand"
	"runtime"
	"strings"
	"time"
)

var (
	// Pools of possible values for randomization
	podNames       = []string{"aws-integration-agent-00-1", "aws-network-flow-monitor-agent-qdrt2", "gkwk-ncde-prod-auth-v4-84c96ffcc5-fzlx2", "civil-eagle-us-signoz-otel-collector-975668fb8-n28mc", "civil-eagle-us-clickhouse-operator-7cff467cc7-lhrqv"}
	namespaceNames = []string{"amazon-network-flow-monitor", "prod", "dev", "test"}
	containerNames = []string{"aws-network-flow-monitor-agent", "gkwk-ncde-prod-auth-v4", "aws-network-runner-0000-01", "witcher2-0000-01"}
	levels         = []string{"INFO", "WARN", "ERROR", "DEBUG"}
	messages       = []string{"Error sending abc webhooks", "Webhook sent", "Processing event", "Under log_processed", "under valorant 3"}
)

// BatchData holds the prepared data for a batch to ensure consistency across tables
type BatchData struct {
	Rows []BatchRow
}

// BatchRow represents a single row of data
type BatchRow struct {
	ID        uint64
	Timestamp uint64
	Body      any
}

// PathGenerator generates unique meaningful paths
type PathGenerator struct {
	usedPaths map[string]bool
	rand      *rand.Rand
}

func NewPathGenerator() *PathGenerator {
	return &PathGenerator{
		usedPaths: make(map[string]bool),
		rand:      rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (pg *PathGenerator) randomFromPool(pool []string) string {
	return pool[pg.rand.Intn(len(pool))]
}

func (pg *PathGenerator) randomInt(min, max int) int {
	return pg.rand.Intn(max-min+1) + min
}

func (pg *PathGenerator) randomBool() bool {
	return pg.rand.Intn(2) == 0
}

// generateUniquePath creates a unique path that hasn't been used before
func (pg *PathGenerator) generateUniquePath() string {
	attempts := 0
	for attempts < 1000 {
		path := pg.generatePath()
		if !pg.usedPaths[path] {
			pg.usedPaths[path] = true
			return path
		}
		attempts++
	}
	// Fallback with timestamp to ensure uniqueness
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("fallback.path.%d", timestamp)
}

// generatePath creates a meaningful path structure with random depth
func (pg *PathGenerator) generatePath() string {
	// Random depth between 2 and 6 levels
	depth := pg.randomInt(2, 6)

	pathComponents := []string{}

	// Root level - always present, ensure variety
	roots := []string{
		"application", "system", "user", "service", "api", "database", "network",
		"security", "monitoring", "logging", "metrics", "events", "transactions",
		"authentication", "authorization", "payment", "shipping", "inventory",
		"customer", "order", "product", "notification", "cache", "queue",
	}
	pathComponents = append(pathComponents, pg.randomFromPool(roots))

	// Second level - context specific
	switch pathComponents[0] {
	case "application":
		seconds := []string{"config", "state", "session", "cache", "memory", "threads", "processes", "modules", "components", "services", "routes", "middleware", "handlers", "controllers", "models", "views", "templates", "assets", "static", "public", "private", "admin", "user", "api", "web", "mobile", "desktop"}
		pathComponents = append(pathComponents, pg.randomFromPool(seconds))
	case "system":
		seconds := []string{"os", "kernel", "processes", "memory", "cpu", "disk", "network", "filesystem", "users", "groups", "permissions", "services", "daemons", "drivers", "devices", "ports", "sockets", "pipes", "signals", "interrupts", "scheduler", "virtualization", "containerization"}
		pathComponents = append(pathComponents, pg.randomFromPool(seconds))
	case "user":
		seconds := []string{"profile", "preferences", "settings", "sessions", "authentication", "authorization", "permissions", "roles", "groups", "activity", "history", "data", "files", "documents", "media", "contacts", "messages", "notifications", "subscriptions", "billing", "payment", "shipping", "orders"}
		pathComponents = append(pathComponents, pg.randomFromPool(seconds))
	case "service":
		seconds := []string{"health", "status", "metrics", "logs", "errors", "performance", "availability", "latency", "throughput", "requests", "responses", "timeouts", "failures", "retries", "circuit_breaker", "load_balancer", "proxy", "gateway", "discovery", "registry", "configuration", "deployment", "scaling"}
		pathComponents = append(pathComponents, pg.randomFromPool(seconds))
	case "api":
		seconds := []string{"endpoints", "routes", "methods", "parameters", "headers", "body", "response", "status", "rate_limiting", "authentication", "authorization", "validation", "serialization", "deserialization", "caching", "versioning", "documentation", "testing", "monitoring", "analytics", "usage", "performance"}
		pathComponents = append(pathComponents, pg.randomFromPool(seconds))
	case "database":
		seconds := []string{"connection", "query", "transaction", "index", "table", "schema", "migration", "backup", "replication", "sharding", "partitioning", "optimization", "performance", "monitoring", "logs", "errors", "slow_queries", "deadlocks", "locks", "cache", "pool", "driver", "orm"}
		pathComponents = append(pathComponents, pg.randomFromPool(seconds))
	case "network":
		seconds := []string{"connection", "socket", "protocol", "packet", "bandwidth", "latency", "throughput", "routing", "dns", "firewall", "proxy", "load_balancer", "vpn", "ssl", "tls", "certificate", "authentication", "authorization", "monitoring", "traffic", "errors", "timeouts", "retries"}
		pathComponents = append(pathComponents, pg.randomFromPool(seconds))
	case "security":
		seconds := []string{"authentication", "authorization", "encryption", "decryption", "hashing", "signing", "certificate", "key", "token", "session", "permission", "role", "policy", "audit", "logging", "monitoring", "threat", "vulnerability", "scan", "firewall", "intrusion", "detection", "prevention"}
		pathComponents = append(pathComponents, pg.randomFromPool(seconds))
	case "monitoring":
		seconds := []string{"metrics", "logs", "alerts", "dashboards", "health", "status", "performance", "availability", "latency", "throughput", "errors", "warnings", "events", "traces", "profiling", "sampling", "aggregation", "storage", "retention", "analysis", "reporting", "notification"}
		pathComponents = append(pathComponents, pg.randomFromPool(seconds))
	case "logging":
		seconds := []string{"level", "format", "output", "rotation", "retention", "compression", "encryption", "filtering", "parsing", "aggregation", "correlation", "sampling", "buffering", "async", "sync", "structured", "unstructured", "json", "text", "binary", "syslog", "journald"}
		pathComponents = append(pathComponents, pg.randomFromPool(seconds))
	case "metrics":
		seconds := []string{"counter", "gauge", "histogram", "summary", "rate", "duration", "throughput", "latency", "error_rate", "success_rate", "availability", "utilization", "capacity", "saturation", "errors", "warnings", "custom", "business", "technical", "infrastructure", "application"}
		pathComponents = append(pathComponents, pg.randomFromPool(seconds))
	case "events":
		seconds := []string{"publish", "subscribe", "queue", "stream", "batch", "real_time", "near_real_time", "delayed", "scheduled", "periodic", "triggered", "manual", "automatic", "system", "user", "business", "technical", "audit", "security", "performance", "error", "warning", "info"}
		pathComponents = append(pathComponents, pg.randomFromPool(seconds))
	case "transactions":
		seconds := []string{"begin", "commit", "rollback", "savepoint", "isolation", "consistency", "durability", "atomicity", "locking", "deadlock", "timeout", "retry", "compensation", "saga", "distributed", "local", "nested", "flat", "long_running", "short_running", "critical", "non_critical"}
		pathComponents = append(pathComponents, pg.randomFromPool(seconds))
	case "authentication":
		seconds := []string{"login", "logout", "register", "password", "token", "session", "oauth", "saml", "ldap", "kerberos", "certificate", "biometric", "two_factor", "multi_factor", "single_sign_on", "federation", "identity", "provider", "client", "server", "user", "service"}
		pathComponents = append(pathComponents, pg.randomFromPool(seconds))
	case "authorization":
		seconds := []string{"permission", "role", "policy", "access", "control", "resource", "action", "subject", "object", "context", "condition", "rule", "decision", "enforcement", "audit", "logging", "monitoring", "review", "approval", "delegation", "inheritance", "hierarchy"}
		pathComponents = append(pathComponents, pg.randomFromPool(seconds))
	case "payment":
		seconds := []string{"gateway", "processor", "method", "card", "bank", "wallet", "crypto", "invoice", "receipt", "refund", "chargeback", "dispute", "settlement", "reconciliation", "fraud", "risk", "compliance", "audit", "reporting", "analytics", "integration", "webhook"}
		pathComponents = append(pathComponents, pg.randomFromPool(seconds))
	case "shipping":
		seconds := []string{"carrier", "method", "tracking", "label", "package", "address", "zone", "rate", "cost", "time", "delivery", "pickup", "return", "exchange", "damage", "loss", "insurance", "signature", "notification", "status", "history", "analytics"}
		pathComponents = append(pathComponents, pg.randomFromPool(seconds))
	case "inventory":
		seconds := []string{"stock", "item", "sku", "category", "warehouse", "location", "movement", "adjustment", "reservation", "allocation", "fulfillment", "backorder", "oversell", "cycle_count", "audit", "valuation", "cost", "price", "supplier", "purchase", "order", "receipt"}
		pathComponents = append(pathComponents, pg.randomFromPool(seconds))
	case "customer":
		seconds := []string{"profile", "account", "preferences", "history", "orders", "wishlist", "reviews", "support", "tickets", "feedback", "segmentation", "loyalty", "points", "rewards", "communication", "marketing", "consent", "privacy", "data", "gdpr", "compliance"}
		pathComponents = append(pathComponents, pg.randomFromPool(seconds))
	case "order":
		seconds := []string{"creation", "processing", "confirmation", "payment", "shipping", "tracking", "delivery", "cancellation", "refund", "return", "exchange", "modification", "status", "history", "items", "totals", "taxes", "discounts", "coupons", "notes", "attachments"}
		pathComponents = append(pathComponents, pg.randomFromPool(seconds))
	case "product":
		seconds := []string{"catalog", "details", "images", "variants", "options", "pricing", "availability", "reviews", "ratings", "recommendations", "related", "cross_sell", "upsell", "bundle", "kit", "digital", "physical", "subscription", "rental", "auction", "bidding"}
		pathComponents = append(pathComponents, pg.randomFromPool(seconds))
	case "notification":
		seconds := []string{"email", "sms", "push", "in_app", "webhook", "template", "content", "delivery", "status", "read", "unread", "archive", "delete", "preferences", "frequency", "channel", "priority", "urgent", "normal", "low", "scheduled", "immediate"}
		pathComponents = append(pathComponents, pg.randomFromPool(seconds))
	case "cache":
		seconds := []string{"memory", "redis", "memcached", "local", "distributed", "cluster", "replication", "persistence", "eviction", "expiration", "invalidation", "warming", "preloading", "hit_rate", "miss_rate", "size", "capacity", "performance", "monitoring", "metrics"}
		pathComponents = append(pathComponents, pg.randomFromPool(seconds))
	case "queue":
		seconds := []string{"message", "job", "task", "event", "priority", "dead_letter", "retry", "delay", "scheduled", "batch", "stream", "consumer", "producer", "broker", "topic", "partition", "offset", "commit", "ack", "nack", "reject", "requeue"}
		pathComponents = append(pathComponents, pg.randomFromPool(seconds))
	default:
		seconds := []string{"data", "config", "state", "info", "details", "metadata", "properties", "attributes", "settings", "options", "parameters", "values", "items", "elements", "objects", "entities", "records", "documents", "files", "resources"}
		pathComponents = append(pathComponents, pg.randomFromPool(seconds))
	}

	// Add additional levels based on random depth
	for level := 2; level < depth; level++ {
		if pg.randomBool() {
			additionalLevels := []string{
				"create", "read", "update", "delete", "list", "search", "filter", "sort", "paginate",
				"validate", "transform", "process", "execute", "run", "start", "stop", "pause", "resume",
				"enable", "disable", "activate", "deactivate", "approve", "reject", "accept", "decline",
				"send", "receive", "forward", "redirect", "proxy", "route", "dispatch", "deliver",
				"store", "retrieve", "backup", "restore", "sync", "replicate", "migrate", "upgrade",
				"monitor", "alert", "notify", "log", "audit", "track", "trace", "profile", "analyze",
				"calculate", "compute", "estimate", "predict", "forecast", "simulate", "test", "verify",
				"authenticate", "authorize", "encrypt", "decrypt", "hash", "sign", "verify", "certify",
				"connect", "disconnect", "bind", "unbind", "register", "unregister", "subscribe", "unsubscribe",
			}
			pathComponents = append(pathComponents, pg.randomFromPool(additionalLevels))
		}
	}

	// Join all components with dots
	result := ""
	for i, component := range pathComponents {
		if i > 0 {
			result += "."
		}
		result += component
	}

	return result
}

// generateUniquePaths creates unique paths with the current path structure
func (pg *PathGenerator) generateUniquePaths(count int) []string {
	pg.usedPaths = make(map[string]bool)
	paths := make([]string, count)
	for i := 0; i < count; i++ {
		paths[i] = pg.generateUniquePath()
	}
	return paths
}

var (
	// Word pools for generating seven-word messages
	// Using different categories to ensure variety but allow some overlap
	adjectives = []string{
		"quick", "slow", "fast", "bright", "dark", "large", "small", "hot", "cold", "warm",
		"cool", "soft", "hard", "smooth", "rough", "sharp", "dull", "clean", "dirty", "fresh",
		"old", "new", "young", "ancient", "modern", "simple", "complex", "easy", "difficult",
		"safe", "dangerous", "quiet", "loud", "calm", "wild", "gentle", "strong", "weak",
		"heavy", "light", "thick", "thin", "wide", "narrow", "deep", "shallow", "high", "low",
	}

	nouns = []string{
		"system", "process", "service", "application", "database", "server", "client", "user",
		"admin", "manager", "operator", "engineer", "developer", "analyst", "designer", "tester",
		"data", "information", "message", "request", "response", "event", "log", "error", "warning",
		"notification", "alert", "report", "document", "file", "record", "entry", "transaction",
		"session", "connection", "network", "protocol", "interface", "component", "module", "function",
		"method", "algorithm", "pattern", "structure", "format", "schema", "template", "model",
	}

	verbs = []string{
		"processes", "handles", "manages", "controls", "monitors", "tracks", "analyzes", "generates",
		"creates", "updates", "deletes", "modifies", "transforms", "converts", "validates", "verifies",
		"authenticates", "authorizes", "encrypts", "decrypts", "compresses", "decompresses", "synchronizes",
		"replicates", "backups", "restores", "migrates", "deploys", "configures", "initializes", "terminates",
		"starts", "stops", "pauses", "resumes", "schedules", "executes", "runs", "performs", "implements",
		"supports", "maintains", "repairs", "fixes", "optimizes", "improves", "enhances", "extends",
	}

	objects = []string{
		"requests", "responses", "events", "logs", "errors", "warnings", "notifications", "alerts",
		"reports", "documents", "files", "records", "entries", "transactions", "sessions", "connections",
		"data", "information", "messages", "packets", "frames", "buffers", "caches", "queues",
		"tables", "databases", "schemas", "templates", "models", "patterns", "algorithms", "functions",
		"services", "applications", "systems", "processes", "threads", "tasks", "jobs", "operations",
	}

	locations = []string{
		"locally", "remotely", "globally", "regionally", "centrally", "distributed", "clustered",
		"replicated", "mirrored", "backed", "cached", "stored", "archived", "indexed", "sorted",
		"filtered", "aggregated", "processed", "transformed", "validated", "verified", "authenticated",
		"authorized", "encrypted", "compressed", "synchronized", "migrated", "deployed", "configured",
		"initialized", "terminated", "scheduled", "executed", "performed", "implemented", "supported",
	}

	conditions = []string{
		"successfully", "efficiently", "securely", "reliably", "accurately", "precisely", "correctly",
		"properly", "appropriately", "adequately", "sufficiently", "effectively", "optimally", "perfectly",
		"completely", "thoroughly", "carefully", "cautiously", "safely", "smoothly", "seamlessly",
		"automatically", "manually", "dynamically", "statically", "temporarily", "permanently", "immediately",
		"gradually", "incrementally", "periodically", "continuously", "constantly", "frequently", "occasionally",
	}

	contexts = []string{
		"in production", "during testing", "while debugging", "for monitoring", "with logging", "via API",
		"through interface", "across network", "within system", "between services", "among components",
		"over protocol", "under load", "at scale", "on demand", "by schedule", "as needed", "when required",
		"if necessary", "unless specified", "except errors", "including warnings", "excluding failures",
		"plus metadata", "minus overhead", "times performance", "divided by capacity", "modulo constraints",
	}
)

// MessageGenerator generates seven-word messages with controlled randomness
type MessageGenerator struct {
	rand *rand.Rand
}

// NewMessageGenerator creates a new message generator
func NewMessageGenerator(seed int64) *MessageGenerator {
	return &MessageGenerator{
		rand: rand.New(rand.NewSource(seed)),
	}
}

// generateSevenWordMessage creates a seven-word message with low but non-zero probability of matching
func (mg *MessageGenerator) generateSevenWordMessage(rowID uint64) string {
	// Use rowID to influence randomness but allow some overlap
	// This creates deterministic patterns that occasionally repeat

	// Create a deterministic but varied seed based on rowID
	seed := int64(rowID) + int64(rowID%1000) // Add some variation

	// Generate seven words using different pools
	words := make([]string, 7)

	// Word 1: Adjective (deterministic based on rowID)
	words[0] = adjectives[int(rowID)%len(adjectives)]

	// Word 2: Noun (slightly more random)
	words[1] = nouns[int(seed)%len(nouns)]

	// Word 3: Verb (based on rowID with some variation)
	words[2] = verbs[int(rowID*3+uint64(seed))%len(verbs)]

	// Word 4: Object (more random)
	words[3] = objects[int(seed*7+int64(rowID))%len(objects)]

	// Word 5: Location (deterministic pattern)
	words[4] = locations[int(rowID/10)%len(locations)]

	// Word 6: Condition (random with some rowID influence)
	words[5] = conditions[int(seed*11+int64(rowID*2))%len(conditions)]

	// Word 7: Context (most random)
	words[6] = contexts[int(seed*13+int64(rowID*5))%len(contexts)]

	return strings.Join(words, " ")
}

// DataGenerator generates random data
type DataGenerator struct {
	rand          *rand.Rand
	batchSize     int
	maxPathPerLog int
	pathGenerator *PathGenerator
	msgGen        *MessageGenerator
}

func NewDataGenerator(batchSize, maxPathPerLog int) *DataGenerator {
	return &DataGenerator{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
		pathGenerator: &PathGenerator{
			usedPaths: make(map[string]bool),
			rand:      rand.New(rand.NewSource(time.Now().UnixNano())),
		},
		batchSize:     batchSize,
		maxPathPerLog: maxPathPerLog,
		msgGen:        NewMessageGenerator(time.Now().UnixNano()),
	}
}

// randomType generates a random ClickHouse type for Dynamic columns
func (dg *DataGenerator) randomType() string {
	types := []string{
		"String",
		"Int64",
		"Float64",
		"Bool",
		"DateTime64(3)",
		"Array(Nullable(String))",
		"Array(Nullable(Int64))",
		"Array(Nullable(Float64))",
		"Array(Nullable(Bool))",
		"Array(JSON)",
		"Tuple(Nullable(String), Nullable(Int64))",
		"Tuple(Nullable(Float64), Nullable(Bool))",
		"Tuple(Nullable(String), Nullable(Int64), Nullable(Float64), Nullable(Bool))",
		"Tuple(JSON(), Nullable(String), Nullable(Int64), JSON())",
		"JSON",
	}
	return types[dg.rand.Intn(len(types))]
}

func (dg *DataGenerator) randomFromPool(pool []string) string {
	return pool[dg.rand.Intn(len(pool))]
}

func (dg *DataGenerator) randomMap() map[string]interface{} {
	// Rich, nested structure inspired by older generator logic (without path inputs)
	body := map[string]interface{}{
		"stream": "stdout",
		"_p":     "F",
		"log":    fmt.Sprintf(`{"level":"%s","target":"amzn_nfm::events::event_provider_ebpf"}`, dg.randomFromPool(levels)),
		"log_processed": map[string]interface{}{
			"level":     dg.randomFromPool(levels),
			"message":   dg.randomFromPool(messages),
			"target":    "amzn_nfm::events::event_provider_ebpf",
			"timestamp": time.Now().UnixMilli(),
		},
		"kubernetes": map[string]interface{}{
			"pod_name":        dg.randomFromPool(podNames),
			"namespace_name":  dg.randomFromPool(namespaceNames),
			"pod_id":          fmt.Sprintf("%x", rand.Int63()),
			"host":            fmt.Sprintf("ip-%d-%d-%d-%d.ap-south-1.compute.internal", dg.randomInt(10, 99), dg.randomInt(10, 99), dg.randomInt(10, 99), dg.randomInt(10, 99)),
			"container_name":  dg.randomFromPool(containerNames),
			"docker_id":       fmt.Sprintf("%x", rand.Int63()),
			"container_image": "some-image",
		},
		"docker":    []string{"container_1", "container_8"},
		"details":   dg.randomDetails(),
		"uninstall": dg.randomBool(),
		"message":   dg.randomFromPool(messages),
	}

	// Add richer arrays like the older version
	body["array_primitives_same_type"] = []int{dg.randomInt(1, 100), dg.randomInt(1, 100), dg.randomInt(1, 100), dg.randomInt(1, 100)}
	body["array_primitives_mixed"] = []interface{}{
		dg.randomInt(1, 100),
		dg.randomFromPool(messages),
		dg.randomBool(),
		dg.rand.Float64(),
	}
	body["array_objects"] = []map[string]interface{}{
		{"a": dg.randomFromPool(messages), "b": dg.randomInt(1, 100), "c": dg.randomBool(), "d": dg.rand.Float64()},
		{"a": dg.randomFromPool(messages), "b": dg.randomInt(1, 100), "c": dg.randomBool(), "d": dg.rand.Float64()},
		{"a": dg.randomFromPool(messages), "b": dg.randomInt(1, 100), "c": dg.randomBool(), "d": dg.rand.Float64()},
		{"a": dg.randomFromPool(messages), "b": dg.randomInt(1, 100), "c": dg.randomBool(), "d": dg.rand.Float64()},
	}
	body["array_objects_and_primitives"] = []interface{}{
		map[string]interface{}{"x": dg.randomFromPool(messages), "y": dg.randomInt(1, 100), "nested": map[string]any{
			"detail": []any{
				map[string]interface{}{"message": dg.randomFromPool(messages), "number": dg.randomInt(1, 100)},
				dg.randomFromPool(messages),
				dg.randomInt(1, 100),
				map[string]interface{}{"amount": dg.rand.Float64(), "is_paid": dg.randomBool()},
			},
		}},
		dg.randomFromPool(messages),
		dg.randomInt(1, 100),
		map[string]interface{}{"z": dg.rand.Float64(), "w": dg.randomBool()},
	}

	// Randomly remove or add keys for variability
	if dg.randomBool() {
		delete(body, "uninstall")
	}
	if dg.randomBool() {
		body["created_by"] = dg.randomFromPool([]string{"piyushsingariya", "srikanth", "nitya", "ekansh", "signoz"})
	}
	if dg.randomBool() {
		body["project_name"] = dg.randomFromPool([]string{"signoz.io", "signoz", "signoz-otel-collector", "clickhouse", "opentelemetry"})
	}

	return body
}

// randomDetails generates deeply nested details without depending on external paths
func (dg *DataGenerator) randomDetails() map[string]interface{} {
	details := map[string]interface{}{}

	// Product block
	if dg.randomBoolN(2) {
		product := map[string]interface{}{
			"contextId":         fmt.Sprintf("%x", rand.Int63()),
			"fileName":          dg.randomFromPool([]string{"abc_webhook_trigger.js", "main.go", "service.py"}),
			"level":             dg.randomFromPool([]string{"info", "warn", "error"}),
			"merchantId":        fmt.Sprintf("%x", rand.Int63()),
			"merchantShortName": dg.randomFromPool([]string{"Conscious Chemist", "BrandX", "BrandY"}),
			"message":           dg.randomFromPool(messages),
			"methodName":        dg.randomFromPool([]string{"abcTrigger", "defTrigger", "ghiTrigger"}),
			"span_id":           fmt.Sprintf("%x", rand.Int63()),
			"timeSpent":         dg.randomFromPool([]string{"NA", "10ms", "100ms"}),
			"trace_flags":       dg.randomInt(0, 1),
			"trace_id":          fmt.Sprintf("%x", rand.Int63()),
		}

		if dg.randomBool() {
			additionalInfo := map[string]interface{}{
				"error":     dg.randomBool(),
				"errorData": nil,
				"message":   dg.randomFromPool(messages),
			}

			if dg.randomBool() {
				shipping := map[string]interface{}{
					"cod_charges":             0,
					"discounted_price":        0,
					"external_shipping_match": "",
					"id":                      dg.randomInt(1, 10),
					"max":                     dg.randomInt(100, 500),
					"min":                     0,
					"name":                    "Standard Shipping",
					"payment_options":         "all",
					"pincodes_defined":        dg.randomBool(),
					"postpaid_price":          dg.randomInt(100, 500),
					"prepaid_price":           dg.randomInt(100, 500),
					"price":                   dg.randomInt(0, 50),
					"shipping_tag":            "",
					"shipping_uuid":           fmt.Sprintf("#%d", dg.randomInt(100, 999)),
					"title":                   "Standard Shipping",
					"total_amount":            dg.randomInt(1900, 2000),
					"user_type":               "",
					"visibility":              dg.randomBool(),
				}

				data := map[string]interface{}{
					"cart_id":  dg.randomInt(100000000, 999999999),
					"shipping": shipping,
					"status":   dg.randomFromPool([]string{"pending", "confirmed", "shipped"}),
				}
				additionalInfo["data"] = data
			}
			product["additionalInfo"] = additionalInfo
		}

		if dg.randomBool() {
			delete(product, "span_id")
		}
		if dg.randomBool() {
			delete(product, "merchantShortName")
		}
		if dg.randomBool() {
			delete(product, "trace_flags")
		}
		details["product"] = product
	}

	// Game block
	if dg.randomBool() {
		game := map[string]interface{}{
			"is_game": dg.randomFromPool([]string{"true", "false"}),
			"metadata": map[string]interface{}{
				"version":           dg.randomFromPool([]string{"v0.0.1", "v0.0.2", "v0.0.3", "v0.0.4", "v0.0.5"}),
				"installation_path": dg.randomFromPool([]string{"C://games/installed/valorant", "/opt/games/valorant", "C://games/installed/witcher2", "/opt/games/witcher2", "C://games/installed/witcher3", "/opt/games/witcher3", "C://games/installed/witcher4", "/opt/games/witcher4", "C://games/installed/readdeadredemption2", "/opt/games/readdeadredemption2"}),
				"vanguard": map[string]interface{}{
					"running":            dg.randomBool(),
					"malformed_hardware": dg.randomBool(),
					"version":            dg.randomFromPool([]string{"patch_v1.100.0", "patch_v1.101.0"}),
					"hash_check_status":  dg.randomFromPool([]string{"success", "fail"}),
				},
			},
		}

		if dg.randomBool() {
			delete(game["metadata"].(map[string]interface{}), "installation_path")
		}
		details["game"] = game
	}

	// Root level extras
	if dg.randomBool() {
		details["uninstall"] = dg.randomBool()
	}
	if dg.randomBool() {
		details["message"] = dg.randomFromPool(messages)
	}
	if dg.randomBool() {
		details["flag"] = dg.randomBool()
	}
	if dg.randomBool() {
		details["count"] = dg.randomInt(1, 1000)
	}
	if dg.randomBool() {
		details["ratio"] = dg.rand.Float64()
	}

	return details
}

func (dg *DataGenerator) randomBool() bool {
	return dg.rand.Intn(2) == 0
}

func (dg *DataGenerator) randomBoolN(power int) bool {
	prob := dg.rand.Intn(2) == 0
	if power <= 1 {
		return prob
	}

	return prob && dg.randomBoolN(power-1)
}

func (dg *DataGenerator) randomInt(min, max int) int {
	return dg.rand.Intn(max-min+1) + min
}

// generateValueForType generates a value directly for the specified type
func (dg *DataGenerator) generateValueForType(dataType string, rowID uint64) interface{} {
	// Sometimes return null for realistic data (10% chance)
	// But for JSON types, we want to avoid null at the column level
	if dg.rand.Intn(10) == 0 && !strings.Contains(dataType, "JSON") {
		return nil
	}

	switch dataType {
	case "String":
		return dg.msgGen.generateSevenWordMessage(rowID)
	case "Int64":
		// Use rowID as base and add some randomness
		return int64(rowID) + dg.rand.Int63n(1000)
	case "Float64":
		// Use rowID as base for deterministic but varied floats
		return float64(rowID) + dg.rand.Float64()*1000
	case "Bool":
		// Use rowID to determine boolean (more deterministic)
		return (rowID % 2) == 0
	case "DateTime64(3)":
		// Use rowID to create deterministic timestamps
		return time.Now().Add(time.Duration(rowID) * time.Hour)
	case "UUID":
		// Generate UUID based on rowID for consistency
		return fmt.Sprintf("%016x-%04x-%04x-%04x-%012x",
			rowID, dg.rand.Int63n(65536), dg.rand.Int63n(65536),
			dg.rand.Int63n(65536), dg.rand.Int63n(281474976710656))
	case "Array(Nullable(String))":
		size := 1 + int(rowID%5) // Deterministic size based on rowID
		arr := make([]interface{}, size)
		for i := 0; i < size; i++ {
			arr[i] = fmt.Sprintf("arr_%d_%d", rowID, i)
		}
		return arr
	case "Array(Nullable(Int64))":
		size := 1 + int(rowID%5)
		arr := make([]interface{}, size)
		for i := 0; i < size; i++ {
			arr[i] = int64(rowID) + int64(i)
		}
		return arr
	case "Array(Nullable(Float64))":
		size := 1 + int(rowID%5)
		arr := make([]interface{}, size)
		for i := 0; i < size; i++ {
			arr[i] = float64(rowID) + float64(i) + dg.rand.Float64()
		}
		return arr
	case "Array(Nullable(Bool))":
		size := 1 + int(rowID%5)
		arr := make([]interface{}, size)
		for i := 0; i < size; i++ {
			arr[i] = ((rowID + uint64(i)) % 2) == 0
		}
		return arr
	case "Tuple(Nullable(String), Nullable(Int64))":
		return []interface{}{
			fmt.Sprintf("tuple_str_%d", rowID),
			int64(rowID) + dg.rand.Int63n(1000),
		}
	case "Tuple(Nullable(Float64), Nullable(Bool))":
		return []interface{}{
			float64(rowID) + dg.rand.Float64()*1000,
			(rowID % 2) == 0,
		}
	case "Tuple(Nullable(String), Nullable(Int64), Nullable(Float64), Nullable(Bool))":
		return []interface{}{
			fmt.Sprintf("tuple_str_%d", rowID),
			int64(rowID) + dg.rand.Int63n(1000),
			float64(rowID) + dg.rand.Float64()*1000,
			(rowID % 2) == 0,
		}
	case "Tuple(JSON(), Nullable(String), Nullable(Int64), JSON())":
		return []interface{}{
			dg.randomMap(),
			fmt.Sprintf("tuple_str_%d", rowID),
			int64(rowID) + dg.rand.Int63n(1000),
			dg.randomMap(),
		}
	case "Array(JSON)":
		size := 1 + int(rowID%5)
		arr := make([]interface{}, size)
		for i := 0; i < size; i++ {
			// Simple JSON object based on rowID
			arr[i] = dg.randomMap()
		}
		return arr
	case "JSON":
		// Simple JSON object based on rowID
		return dg.randomMap()
	default:
		// For any other type, convert to string with rowID context
		return fmt.Sprintf("data_%d_%s", rowID, dataType)
	}
}

// prepareBatchData prepares all data for a batch to ensure consistency
func (dg *DataGenerator) GenerateBatch() (*BatchData, error) {
	defer runtime.GC()

	batchData := &BatchData{
		Rows: make([]BatchRow, 0, dg.batchSize),
	}

	// Generate initial paths for this batch
	paths := dg.pathGenerator.generateUniquePaths(dg.maxPathPerLog)
	// Generate column types for THIS record (random types every record)
	columnTypes := make([]string, len(paths))
	setTypes := func() {
		columnTypes = make([]string, len(paths))
		for i := range paths {
			columnTypes[i] = dg.randomType()
			// special case for message column to be String type
			if paths[i] == "message" && dg.randomBool() {
				columnTypes[i] = "String"
			}
		}
	}
	setTypes()

	for id := uint64(0); id < uint64(dg.batchSize); id++ {
		// Generate new path structure every 1000 records
		if id%1000 == 0 {
			paths = dg.pathGenerator.generateUniquePaths(rand.Intn(dg.maxPathPerLog-1) + 1)
			setTypes()
		} else if dg.randomBool() {
			setTypes()
		}
		row := BatchRow{
			ID:        id,
			Timestamp: uint64(time.Now().UnixNano()),
		}

		if dg.randomBoolN(10) {
			row.Body = dg.msgGen.generateSevenWordMessage(id)
		} else {
			// Generate payload - this is the source of truth
			payload := make(map[string]any)

			// Generate column values directly based on the types for THIS record
			columnValues := make([]interface{}, len(paths))
			for i, dataType := range columnTypes {
				// Generate value directly for the type, no extraction needed
				value := dg.generateValueForType(dataType, id)
				columnValues[i] = value
				payload[paths[i]] = value
			}
			row.Body = payload
		}

		batchData.Rows = append(batchData.Rows, row)
	}

	return batchData, nil
}
