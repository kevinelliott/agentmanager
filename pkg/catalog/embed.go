package catalog

import _ "embed"

// embeddedCatalogJSON is the baseline catalog baked into the binary at
// build time. It guarantees every agentmgr install — including `go install`
// users who don't have a package-managed /usr/share/agentmgr/catalog.json
// — has a working catalog before the first remote refresh.
//
// The source file is `pkg/catalog/catalog.json`, kept in sync with the
// repo-root catalog.json (the contributor-facing source and the URL
// endpoint for remote refreshes) via the Makefile `sync-catalog` target
// and the CI `check-catalog-sync` verification.
//
//go:embed catalog.json
var embeddedCatalogJSON []byte

// EmbeddedJSON returns the raw JSON bytes of the embedded catalog.
// Callers that want a parsed *Catalog should use Manager.Get instead.
func EmbeddedJSON() []byte {
	return embeddedCatalogJSON
}
