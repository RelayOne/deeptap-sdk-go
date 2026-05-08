package deeptap

import "time"

// ---------------------------------------------------------------------------
// Common envelope blocks
// ---------------------------------------------------------------------------

// RateLimitView is the per-request rate-limit block returned on most
// envelopes. Limit is the per-tier ceiling, Remaining is the number of
// requests left in the current bucket, ResetMS is the milliseconds
// until the bucket refills.
type RateLimitView struct {
	Limit     int   `json:"limit"`
	Remaining int   `json:"remaining"`
	ResetMS   int64 `json:"reset_ms"`
}

// ---------------------------------------------------------------------------
// /v1/search
// ---------------------------------------------------------------------------

// ProviderClass is "safe" (sandboxed providers) or "fast" (raw providers
// that require a legal acknowledgement). The empty value defers to the
// caller's per-key / org default.
type ProviderClass string

const (
	ProviderSafe ProviderClass = "safe"
	ProviderFast ProviderClass = "fast"
)

// SafeMode controls firewall behaviour. The empty value defers to the
// org / endpoint default.
type SafeMode string

const (
	SafeModeOff      SafeMode = "off"
	SafeModeModerate SafeMode = "moderate"
	SafeModeStrict   SafeMode = "strict"
	SafeModeAgent    SafeMode = "agent"
)

// SearchRequest is the POST /v1/search body.
type SearchRequest struct {
	Query         string        `json:"query"`
	Depth         int           `json:"depth,omitempty"`
	ProviderClass ProviderClass `json:"provider_class,omitempty"`
	Count         int           `json:"count,omitempty"`
	Country       string        `json:"country,omitempty"`
	Language      string        `json:"language,omitempty"`
	SafeMode      SafeMode      `json:"safe_mode,omitempty"`
	SubQueries    []string      `json:"sub_queries,omitempty"`
	IncludeLedger bool          `json:"include_ledger,omitempty"`
	SourceSuggest bool          `json:"source_suggest,omitempty"`
}

// SearchResult mirrors the server's per-result struct.
type SearchResult struct {
	URL           string     `json:"url"`
	NormalizedURL string     `json:"normalized_url,omitempty"`
	Title         string     `json:"title"`
	Snippet       string     `json:"snippet"`
	Domain        string     `json:"domain,omitempty"`
	Score         float64    `json:"score,omitempty"`
	PublishedAt   *time.Time `json:"published_at,omitempty"`
	RerankScore   *float64   `json:"rerank_score,omitempty"`
}

// DecompositionView is the LLM-decomposition metadata stamped on the
// search envelope when a Decomposer ran.
type DecompositionView struct {
	Model            string   `json:"model,omitempty"`
	Provider         string   `json:"provider,omitempty"`
	GenerationID     string   `json:"generation_id,omitempty"`
	SubQueries       []string `json:"sub_queries,omitempty"`
	TokensPrompt     int      `json:"tokens_prompt,omitempty"`
	TokensCompletion int      `json:"tokens_completion,omitempty"`
	CostUSD          float64  `json:"cost_usd,omitempty"`
	LatencyMS        int64    `json:"latency_ms,omitempty"`
}

// RerankerView is the per-request rerank metadata stamped on the
// envelope when the rerank step ran.
type RerankerView struct {
	Model          string `json:"model,omitempty"`
	Implementation string `json:"implementation,omitempty"`
	LatencyMS      int64  `json:"latency_ms,omitempty"`
	DocsScored     int    `json:"docs_scored,omitempty"`
	Error          string `json:"error,omitempty"`
}

// SearchResponse is the 200 envelope from POST /v1/search.
type SearchResponse struct {
	RequestID            string             `json:"request_id"`
	AsOf                 time.Time          `json:"as_of"`
	ResponseTimeMS       int64              `json:"response_time_ms"`
	CreditsUsed          float64            `json:"credits_used"`
	CacheHit             bool               `json:"cache_hit"`
	CacheHitType         string             `json:"cache_hit_type,omitempty"`
	CacheKeysHit         []string           `json:"cache_keys_hit,omitempty"`
	PromptInjectionScore float64            `json:"prompt_injection_score"`
	Provider             string             `json:"provider"`
	Depth                int                `json:"depth"`
	RoundsExecuted       int                `json:"rounds_executed,omitempty"`
	StopReason           string             `json:"stop_reason,omitempty"`
	Results              []SearchResult     `json:"results"`
	Warnings             []string           `json:"warnings,omitempty"`
	UnsafeReasons        []string           `json:"unsafe_reasons,omitempty"`
	FreshnessClass       string             `json:"freshness_class,omitempty"`
	FreshnessReason      string             `json:"freshness_reason,omitempty"`
	Decomposition        *DecompositionView `json:"decomposition,omitempty"`
	Reranker             *RerankerView      `json:"reranker,omitempty"`
	RateLimit            *RateLimitView     `json:"rate_limit,omitempty"`
}

// ---------------------------------------------------------------------------
// /v1/research  (depth 3, SSE)
// ---------------------------------------------------------------------------

// ResearchRequest is the POST /v1/research body.
type ResearchRequest struct {
	Query         string        `json:"query"`
	ProviderClass ProviderClass `json:"provider_class,omitempty"`
	SafeMode      SafeMode      `json:"safe_mode,omitempty"`
	IncludeLedger bool          `json:"include_ledger,omitempty"`
	MaxRounds     int           `json:"max_rounds,omitempty"`
	TimeoutMS     int           `json:"timeout_ms,omitempty"`
}

// ResearchStreamEvent is one SSE frame from /v1/research.
type ResearchStreamEvent struct {
	Event string         `json:"event"`
	Data  map[string]any `json:"data"`
}

// ResearchResponse is the JSON payload of the final "event: final"
// frame emitted at the end of a research stream.
type ResearchResponse struct {
	RequestID      string             `json:"request_id"`
	AsOf           time.Time          `json:"as_of"`
	ResponseTimeMS int64              `json:"response_time_ms"`
	CreditsUsed    float64            `json:"credits_used"`
	CacheHit       bool               `json:"cache_hit"`
	CacheHitType   string             `json:"cache_hit_type,omitempty"`
	CacheKeysHit   []string           `json:"cache_keys_hit,omitempty"`
	Depth          int                `json:"depth"`
	RoundsExecuted int                `json:"rounds_executed"`
	StopReason     string             `json:"stop_reason,omitempty"`
	Provider       string             `json:"provider,omitempty"`
	Results        []SearchResult     `json:"results"`
	Warnings       []string           `json:"warnings,omitempty"`
	UnsafeReasons  []string           `json:"unsafe_reasons,omitempty"`
	Decomposition  *DecompositionView `json:"decomposition,omitempty"`
	RateLimit      *RateLimitView     `json:"rate_limit,omitempty"`
}

// ---------------------------------------------------------------------------
// /v1/extract
// ---------------------------------------------------------------------------

// ExtractMode is "excerpt" (snippet-grade extraction, default) or
// "full" (complete article body where available).
type ExtractMode string

const (
	ExtractModeExcerpt ExtractMode = "excerpt"
	ExtractModeFull    ExtractMode = "full"
)

// ExtractRequest is the POST /v1/extract body.
type ExtractRequest struct {
	URLs      []string    `json:"urls"`
	Query     string      `json:"query,omitempty"`
	Mode      ExtractMode `json:"mode,omitempty"`
	SafeMode  SafeMode    `json:"safe_mode,omitempty"`
	TimeoutMS int         `json:"timeout_ms,omitempty"`
}

// ExtractResult is one entry in ExtractResponse.Results.
type ExtractResult struct {
	URL                  string    `json:"url"`
	NormalizedURL        string    `json:"normalized_url,omitempty"`
	Title                string    `json:"title,omitempty"`
	Domain               string    `json:"domain,omitempty"`
	Excerpt              string    `json:"excerpt,omitempty"`
	Text                 string    `json:"text,omitempty"`
	ContentLength        int32     `json:"content_length,omitempty"`
	ExtractedAt          time.Time `json:"extracted_at,omitempty"`
	Warnings             []string  `json:"warnings,omitempty"`
	SanitizedContent     string    `json:"sanitized_content,omitempty"`
	UntrustedContent     string    `json:"untrusted_content,omitempty"`
	TrustedSnippet       string    `json:"trusted_snippet,omitempty"`
	PromptInjectionScore float64   `json:"prompt_injection_score,omitempty"`
	UnsafeReasons        []string  `json:"unsafe_reasons,omitempty"`
}

// FirewallView is the per-request firewall metadata stamped on the
// extract envelope.
type FirewallView struct {
	Layer1StrippedBytes   int      `json:"layer1_stripped_bytes"`
	Layer1PatternsMatched []string `json:"layer1_patterns_matched,omitempty"`
	Layer2Model           string   `json:"layer2_model,omitempty"`
	Layer2Implementation  string   `json:"layer2_implementation"`
	Layer2LatencyMS       int64    `json:"layer2_latency_ms,omitempty"`
	Layer2DocsScored      int      `json:"layer2_docs_scored,omitempty"`
}

// ExtractResponse is the 200 envelope from POST /v1/extract.
type ExtractResponse struct {
	RequestID             string          `json:"request_id"`
	AsOf                  time.Time       `json:"as_of"`
	ResponseTimeMS        int64           `json:"response_time_ms"`
	CreditsUsed           float64         `json:"credits_used"`
	CacheHit              bool            `json:"cache_hit"`
	CacheHitType          string          `json:"cache_hit_type,omitempty"`
	CacheKeysHit          []string        `json:"cache_keys_hit,omitempty"`
	PromptInjectionScore  float64         `json:"prompt_injection_score"`
	UnsafeReasons         []string        `json:"unsafe_reasons,omitempty"`
	SanitizedContentBytes int             `json:"sanitized_content_bytes,omitempty"`
	UntrustedContentBytes int             `json:"untrusted_content_bytes,omitempty"`
	Firewall              *FirewallView   `json:"firewall,omitempty"`
	SafeModeApplied       string          `json:"safe_mode_applied,omitempty"`
	Results               []ExtractResult `json:"results"`
	RateLimit             *RateLimitView  `json:"rate_limit,omitempty"`
}

// ---------------------------------------------------------------------------
// /v1/map
// ---------------------------------------------------------------------------

// MapRequest is the POST /v1/map body.
type MapRequest struct {
	URL               string        `json:"url"`
	MaxDepth          int           `json:"max_depth,omitempty"`
	MaxBreadth        int           `json:"max_breadth,omitempty"`
	Limit             int           `json:"limit,omitempty"`
	AllowExternal     bool          `json:"allow_external,omitempty"`
	IncludeSubdomains bool          `json:"include_subdomains,omitempty"`
	Exclude           []string      `json:"exclude,omitempty"`
	ProviderClass     ProviderClass `json:"provider_class,omitempty"`
}

// MapSources is the per-source URL count breakdown returned by /v1/map.
type MapSources struct {
	RobotsSitemapURLs int `json:"robots_sitemap_urls"`
	SitemapURLs       int `json:"sitemap_urls"`
	HTMLCrawlURLs     int `json:"html_crawl_urls"`
	PagesFetched      int `json:"pages_fetched"`
}

// MapResponse is the 200 envelope from POST /v1/map.
type MapResponse struct {
	RequestID            string     `json:"request_id"`
	AsOf                 time.Time  `json:"as_of"`
	ResponseTimeMS       int64      `json:"response_time_ms"`
	URL                  string     `json:"url"`
	CreditsUsed          float64    `json:"credits_used"`
	Results              []string   `json:"results"`
	Sources              MapSources `json:"sources"`
	Dropped              int        `json:"dropped"`
	Truncated            bool       `json:"truncated"`
	PromptInjectionScore float64    `json:"prompt_injection_score"`
}

// ---------------------------------------------------------------------------
// /v1/facts/query
// ---------------------------------------------------------------------------

// FactType is the temporal-decay class for a Fact.
type FactType string

const (
	FactPermanent     FactType = "permanent"
	FactSlowDecay     FactType = "slow_decay"
	FactModerateDecay FactType = "moderate_decay"
	FactFastDecay     FactType = "fast_decay"
	FactVolatile      FactType = "volatile"
)

// Stance reports the relationship between a piece of evidence and the
// fact it supports, contradicts, or is neutral about.
type Stance string

const (
	StanceSupports    Stance = "supports"
	StanceContradicts Stance = "contradicts"
	StanceNeutral     Stance = "neutral"
)

// Evidence is one supporting row for a Fact.
type Evidence struct {
	SourceURL    string    `json:"source_url,omitempty"`
	Domain       string    `json:"domain,omitempty"`
	Stance       Stance    `json:"stance,omitempty"`
	NLIScore     float64   `json:"nli_score,omitempty"`
	DiscoveredAt time.Time `json:"discovered_at,omitempty"`
}

// Fact mirrors the server's FactResult wire shape.
type Fact struct {
	ID                  string     `json:"id"`
	SubjectQID          string     `json:"subject_qid,omitempty"`
	SubjectText         string     `json:"subject_text"`
	Predicate           string     `json:"predicate"`
	ObjectText          string     `json:"object_text"`
	ObjectType          string     `json:"object_type,omitempty"`
	FactType            FactType   `json:"fact_type,omitempty"`
	ConfidenceBase      float64    `json:"confidence_base"`
	EffectiveConfidence float64    `json:"effective_confidence"`
	LastConfirmedAt     time.Time  `json:"last_confirmed_at"`
	EvidenceCount       int        `json:"evidence_count"`
	ConflictFlag        bool       `json:"conflict_flag"`
	Evidence            []Evidence `json:"evidence,omitempty"`
}

// FactsRequest is the POST /v1/facts/query body. One of Subject or
// SubjectQID is required.
type FactsRequest struct {
	Subject         string   `json:"subject,omitempty"`
	SubjectQID      string   `json:"subject_qid,omitempty"`
	Predicate       string   `json:"predicate,omitempty"`
	MinConfidence   *float64 `json:"min_confidence,omitempty"`
	IncludeEvidence *bool    `json:"include_evidence,omitempty"`
	MaxResults      int      `json:"max_results,omitempty"`
}

// FactsResponse is the 200 envelope from POST /v1/facts/query.
type FactsResponse struct {
	RequestID            string         `json:"request_id"`
	AsOf                 time.Time      `json:"as_of"`
	ResponseTimeMS       int64          `json:"response_time_ms"`
	CreditsUsed          float64        `json:"credits_used"`
	CacheHit             bool           `json:"cache_hit"`
	PromptInjectionScore *float64       `json:"prompt_injection_score"`
	Facts                []Fact         `json:"facts"`
	RateLimit            *RateLimitView `json:"rate_limit,omitempty"`
}

// ---------------------------------------------------------------------------
// /v1/facts/bulk-query (S25)
// ---------------------------------------------------------------------------

// FactsBulkSubject is one (entity_id, predicate) tuple in a rapid
// bulk-query request.
type FactsBulkSubject struct {
	EntityID  string `json:"entity_id"`
	Predicate string `json:"predicate"`
}

// FactsBulkRequest is the POST /v1/facts/bulk-query body. Subjects must
// be 1..100; the server returns 400 with type=too_many_subjects above.
// IncludeProvenance defaults to true server-side; IncludeExplanation
// defaults to false. MaxEvidence caps the per-fact provenance entries
// at min(MaxEvidence, 10).
type FactsBulkRequest struct {
	Subjects           []FactsBulkSubject `json:"subjects"`
	MinConfidence      *float64           `json:"min_confidence,omitempty"`
	IncludeProvenance  *bool              `json:"include_provenance,omitempty"`
	IncludeExplanation *bool              `json:"include_explanation,omitempty"`
	MaxEvidence        int                `json:"max_evidence,omitempty"`
}

// FactsBulkProvenance is one source-of-record row in the evidence chain
// returned per fact. DomainTrustTier is the S24 nightly compute output
// or "unknown" when domain_profiles has no row for the source domain.
type FactsBulkProvenance struct {
	SourceURL       string    `json:"source_url"`
	Domain          string    `json:"domain,omitempty"`
	DomainTrustTier string    `json:"domain_trust_tier"`
	Stance          string    `json:"stance,omitempty"`
	NLIScore        float64   `json:"nli_score"`
	DiscoveredAt    time.Time `json:"discovered_at"`
}

// FactsBulkExplanation is the 4-component decomposition of effective
// confidence returned when IncludeExplanation=true.
type FactsBulkExplanation struct {
	BaseConfidence             float64 `json:"base_confidence"`
	DecayFactor                float64 `json:"decay_factor"`
	EvidenceCount              int     `json:"evidence_count"`
	EvidenceCountFactor        float64 `json:"evidence_count_factor"`
	EvidenceDiversity          float64 `json:"evidence_diversity"`
	EvidenceDiversityFactor    float64 `json:"evidence_diversity_factor"`
	EffectiveConfidence        float64 `json:"effective_confidence"`
	EffectiveConfidenceFormula string  `json:"effective_confidence_formula"`
}

// FactsBulkResult is one entry in the bulk-query response. Aligned
// 1:1 with the request's subjects. EffectiveConfidence is nil when no
// fact matched the subject.
type FactsBulkResult struct {
	SubjectEntityID     string                `json:"subject_entity_id"`
	Predicate           string                `json:"predicate"`
	ObjectText          string                `json:"object_text,omitempty"`
	EffectiveConfidence *float64              `json:"effective_confidence"`
	FactType            string                `json:"fact_type,omitempty"`
	LastConfirmedAt     *time.Time            `json:"last_confirmed_at,omitempty"`
	Provenance          []FactsBulkProvenance `json:"provenance,omitempty"`
	Explanation         *FactsBulkExplanation `json:"explanation,omitempty"`
	CacheHit            bool                  `json:"cache_hit"`
}

// FactsBulkResponse is the 200 envelope from POST /v1/facts/bulk-query.
// CacheStatus is one of "hit" | "miss" | "mixed".
type FactsBulkResponse struct {
	RequestID            string            `json:"request_id"`
	AsOf                 time.Time         `json:"as_of"`
	ResponseTimeMS       int64             `json:"response_time_ms"`
	CreditsUsed          float64           `json:"credits_used"`
	CacheStatus          string            `json:"cache_hit"`
	PromptInjectionScore *float64          `json:"prompt_injection_score"`
	Results              []FactsBulkResult `json:"results"`
	Stats                *FactsBulkStats   `json:"stats,omitempty"`
	RateLimit            *RateLimitView    `json:"rate_limit,omitempty"`
}

// FactsBulkStats is the per-stage timing block carried in the response.
type FactsBulkStats struct {
	CacheLookupMS int64 `json:"cache_lookup_ms"`
	DBLookupMS    int64 `json:"db_lookup_ms"`
	ProvenanceMS  int64 `json:"provenance_ms"`
	TotalMS       int64 `json:"total_ms"`
}

// ---------------------------------------------------------------------------
// /v1/dmca/report (and counter-notice)
// ---------------------------------------------------------------------------

// DMCAReport is the POST /v1/dmca/report body.
type DMCAReport struct {
	RequesterName      string   `json:"requester_name"`
	RequesterEmail     string   `json:"requester_email"`
	RequesterAddress   string   `json:"requester_address"`
	RequesterPhone     string   `json:"requester_phone"`
	AgentOf            string   `json:"agent_of,omitempty"`
	SwornStatement     bool     `json:"sworn_statement"`
	GoodFaithStatement bool     `json:"good_faith_statement"`
	OriginalWork       string   `json:"original_work"`
	URLs               []string `json:"urls"`
	EvidenceURL        string   `json:"evidence_url,omitempty"`
}

// DMCAReceipt is the 200 envelope from POST /v1/dmca/report.
type DMCAReceipt struct {
	DMCARequestID        string `json:"dmca_request_id"`
	Status               string `json:"status"`
	ExpectedActionWithin string `json:"expected_action_within"`
}

// DMCACounterNotice is the POST /v1/dmca/counter-notice body.
type DMCACounterNotice struct {
	DMCARequestID         string   `json:"dmca_request_id"`
	Token                 string   `json:"token"`
	SubmitterName         string   `json:"submitter_name"`
	SubmitterEmail        string   `json:"submitter_email"`
	SubmitterAddress      string   `json:"submitter_address"`
	ConsentToJurisdiction bool     `json:"consent_to_jurisdiction"`
	SwornStatement        bool     `json:"sworn_statement"`
	OriginalURLSet        []string `json:"original_url_set"`
}

// DMCACounterNoticeReceipt is the 200 envelope from /v1/dmca/counter-notice.
type DMCACounterNoticeReceipt struct {
	CounterNoticeID string    `json:"counter_notice_id"`
	ReinstateAt     time.Time `json:"reinstate_at"`
}

// ---------------------------------------------------------------------------
// A2A
// ---------------------------------------------------------------------------

// AgentCard is the subset of the A2A AgentCard schema published by
// DeepTap. The wire shape is governed by the upstream a2a-go module;
// the SDK exposes the canonical headline fields and tolerates the
// rest via the Extra map.
type AgentCard struct {
	Name        string           `json:"name,omitempty"`
	Description string           `json:"description,omitempty"`
	Version     string           `json:"version,omitempty"`
	URL         string           `json:"url,omitempty"`
	Skills      []map[string]any `json:"skills,omitempty"`
}

// A2AJSONRPCRequest is the POST /a2a/jsonrpc body envelope.
type A2AJSONRPCRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
	ID      any    `json:"id,omitempty"`
}

// A2AJSONRPCResponse is the 200 envelope from POST /a2a/jsonrpc.
type A2AJSONRPCResponse struct {
	JSONRPC string         `json:"jsonrpc"`
	ID      any            `json:"id,omitempty"`
	Result  map[string]any `json:"result,omitempty"`
	Error   map[string]any `json:"error,omitempty"`
}

// ProblemDocument mirrors RFC 7807 Problem Details. The DeepTap API
// returns this for every non-2xx response; the SDK turns it into a
// typed APIError.
type ProblemDocument struct {
	Type      string `json:"type,omitempty"`
	Title     string `json:"title,omitempty"`
	Status    int    `json:"status,omitempty"`
	Detail    string `json:"detail,omitempty"`
	Instance  string `json:"instance,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}
