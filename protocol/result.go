package protocol

type ResultType string

const (
	ResultComplete      ResultType = "complete"
	ResultInputRequired ResultType = "input_required"
)

// ResultBase is embedded in every result type.
type ResultBase struct {
	Meta       Object     `json:"_meta,omitempty"`
	ResultType ResultType `json:"resultType"`
}

func (b *ResultBase) Base() *ResultBase { return b }

type Result interface{ Base() *ResultBase }

type CacheScope string

const (
	CachePublic  CacheScope = "public"
	CachePrivate CacheScope = "private"
)

// CacheFields is embedded in list and read results. ttlMs is always sent, 0 included.
type CacheFields struct {
	TTLMs      int64      `json:"ttlMs"`
	CacheScope CacheScope `json:"cacheScope"`
}

func (c *CacheFields) Cache() *CacheFields { return c }

type Cacheable interface{ Cache() *CacheFields }

type EmptyResult struct {
	ResultBase
}
