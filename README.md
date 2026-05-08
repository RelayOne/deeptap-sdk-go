# deeptap-sdk-go

Official Go client for the [DeepTap](https://deeptap.ai) research API
(search / research / extract / map / facts / DMCA / A2A).

## Install

```bash
go get github.com/RelayOne/deeptap-sdk-go
```

Pin to a tag (recommended):

```bash
go get github.com/RelayOne/deeptap-sdk-go@v0.1.0
```

## Quickstart

```go
package main

import (
    "context"
    "fmt"

    deeptap "github.com/RelayOne/deeptap-sdk-go"
)

func main() {
    client := deeptap.New(deeptap.WithAPIKey("dt_live_..."))
    res, err := client.Search(context.Background(), &deeptap.SearchRequest{
        Query: "who invented sqlite",
        Depth: 1,
    })
    if err != nil {
        panic(err)
    }
    fmt.Println(res.Results[0].URL)
}
```

## Endpoints covered

| Method                          | Endpoint                            | Notes                            |
| ------------------------------- | ----------------------------------- | -------------------------------- |
| `client.Search`                 | `POST /v1/search`                   | depth 1 + depth 2                |
| `client.Research`               | `POST /v1/research`                 | depth 3, returns final envelope  |
| `client.StreamResearch`         | `POST /v1/research`                 | yields each SSE event            |
| `client.Extract`                | `POST /v1/extract`                  | per-URL extraction               |
| `client.Map`                    | `POST /v1/map`                      | site link graph                  |
| `client.Facts`                  | `POST /v1/facts/query`              | structured fact lookup           |
| `client.FactsBulk`              | `POST /v1/facts/bulk-query`         | rapid bulk fact lookup           |
| `client.ReportDMCA`             | `POST /v1/dmca/report`              | DMCA takedown intake             |
| `client.SubmitDMCACounterNotice`| `POST /v1/dmca/counter-notice`      | counter-notice submission        |
| `client.AgentCard`              | `GET /.well-known/agent-card.json`  | A2A agent card                   |
| `client.A2AJSONRPC`             | `POST /a2a/jsonrpc`                 | A2A JSON-RPC 2.0                 |

## Authentication modes

```go
// Default: Bearer (API key)
deeptap.New(deeptap.WithAPIKey("dt_live_..."))

// DPoP-bound session token
deeptap.New(deeptap.WithAuth(&deeptap.DPoPAuth{
    Token:       "opaque-token",
    ProofHeader: "ey.proof",
}))
```

For mutual TLS, supply your own `*http.Client` via
`deeptap.WithHTTPClient` configured with a transport that holds the
client cert.

## Documentation

Full reference docs: <https://docs.deeptap.ai>

## Source of truth

This repository is the public release surface for the Go SDK. The
canonical source lives in the [DeepTap monorepo](https://github.com/RelayOne/deeptap)
under `sdk/go/`. Any change must land upstream first; this mirror
updates in lockstep with each tagged SDK release.

## License

MIT. See [LICENSE](LICENSE).
