// Package deeptap is the official Go SDK for the DeepTap research API.
//
// The SDK exposes a single Client type with one method per canonical
// surface:
//
//   - Search        -> POST /v1/search          (depth 1 / 2)
//   - Research      -> POST /v1/research        (depth 3, SSE)
//   - StreamResearch-> POST /v1/research        (yield SSE events)
//   - Extract       -> POST /v1/extract
//   - Map           -> POST /v1/map
//   - Facts         -> POST /v1/facts/query
//   - ReportDMCA    -> POST /v1/dmca/report
//   - SubmitDMCACounterNotice -> POST /v1/dmca/counter-notice
//   - AgentCard     -> GET  /.well-known/agent-card.json
//   - A2AJSONRPC    -> POST /a2a/jsonrpc
//
// Authentication modes:
//
//   - Bearer (API key) via WithAPIKey
//   - DPoP-bound session token via WithDPoPToken
//   - Mutual TLS via WithMTLS (sets the underlying http.Client transport)
//
// Quickstart:
//
//	client, err := deeptap.New(deeptap.WithAPIKey(os.Getenv("DEEPTAP_API_KEY")))
//	if err != nil { log.Fatal(err) }
//	defer client.Close()
//
//	res, err := client.Search(ctx, deeptap.SearchRequest{Query: "vector dbs"})
package deeptap
