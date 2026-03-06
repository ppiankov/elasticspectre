## Philosophy

*Principiis obsta* — resist the beginnings.

Elasticsearch clusters accumulate stale indices, orphaned replicas, and shard sprawl over time. Storage costs compound silently. A cluster that started lean becomes expensive not because traffic grew, but because nobody cleaned up. ElasticSpectre surfaces these conditions early so they can be addressed before they compound.

The tool presents evidence and lets humans decide. It does not delete indices, does not modify settings, and does not use ML where deterministic checks suffice.


## Installation

```bash
# Homebrew
brew install ppiankov/tap/elasticspectre

# From source
git clone https://github.com/ppiankov/elasticspectre.git
cd elasticspectre && make build
```


## Usage

```bash
elasticspectre audit [flags]
```

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--url` | | Elasticsearch/OpenSearch cluster URL |
| `--cloud-id` | | Elastic Cloud deployment ID (mutually exclusive with `--url`) |
| `--stale-days` | `90` | Days without writes to flag index as stale |
| `--format` | `text` | Output format: `text`, `json`, `spectrehub` |
| `--include-system` | `false` | Include system indices (.kibana, .security, etc.) |

**Environment variables:**

| Variable | Description |
|----------|-------------|
| `ELASTICSEARCH_URL` | Cluster URL (overrides config file) |
| `OPENSEARCH_URL` | Cluster URL (overrides config file) |
| `ELASTIC_CLOUD_ID` | Elastic Cloud deployment ID |

**Other commands:**

| Command | Description |
|---------|-------------|
| `elasticspectre init` | Generate `.elasticspectre.yaml` config file |
| `elasticspectre version` | Print version, commit, and build date |


## Configuration

ElasticSpectre reads `.elasticspectre.yaml` from the current directory or home directory:

```yaml
url: http://localhost:9200
stale_days: 90
format: text
include_system: false
```

Generate a sample config with `elasticspectre init`.


## Authentication

ElasticSpectre connects via HTTP and supports:

- **No auth** — for local development clusters
- **Basic auth** — username/password via config file
- **API key** — via config file
- **Elastic Cloud ID** — decodes Cloud ID to resolve HTTPS endpoint

The tool auto-detects whether the cluster is Elasticsearch or OpenSearch and adjusts API calls accordingly (ILM vs ISM, security endpoint differences).


## Output formats

**Text** (default): Human-readable table with severity, finding type, index name, message, and savings estimate.

**JSON** (`--format json`): `spectre/v1` envelope with findings and summary:
```json
{
  "schema": "spectre/v1",
  "tool": "elasticspectre",
  "target": { "type": "elasticsearch-cluster", "name": "sha256:..." },
  "findings": [...],
  "summary": {
    "total_findings": 8,
    "total_storage_savings": "45.2 GB"
  }
}
```

**SpectreHub** (`--format spectrehub`): `spectre/v1` envelope for SpectreHub ingestion.


## Architecture

```
elasticspectre/
├── cmd/elasticspectre/main.go      # Entry point (LDFLAGS version injection)
├── internal/
│   ├── commands/                   # Cobra CLI: audit, init, version
│   ├── elastic/                    # HTTP client, index/shard/cluster collectors
│   │   ├── client.go              # HTTP wrapper (basic auth, API key, Cloud ID)
│   │   ├── indices.go             # Index metadata, stats, ILM/ISM status
│   │   ├── shards.go              # Shard allocation and sizing
│   │   └── cluster.go             # Health, snapshots, security status
│   ├── analyzer/                   # Finding detection rules (10 finding types)
│   ├── config/                     # YAML config + env var loader
│   ├── report/                     # Text, JSON, SpectreHub reporters
│   └── logging/                    # Structured logging
├── Makefile
└── go.mod
```

Key design decisions:

- **Dual flavor support.** Auto-detects Elasticsearch vs OpenSearch and uses the correct APIs (ILM vs ISM, `_security/_authenticate` vs `_plugins/_security/authinfo`).
- **No external dependencies for HTTP.** Uses Go stdlib `net/http` — no Elasticsearch client library required.
- **Index filtering.** System indices (`.kibana`, `.security`, `.tasks`) excluded by default to reduce noise.
- **Shard analysis.** Checks both total shard count relative to data size and individual shard sizing against the 50 GB guideline.
- **Read-only.** ElasticSpectre never modifies indices, settings, or policies.


## Project status

**Status: Beta** · **v0.1.0** · Pre-1.0

| Milestone | Status |
|-----------|--------|
| 10 finding types (stale indices, shard issues, missing policies, auth) | Complete |
| Elasticsearch + OpenSearch dual support | Complete |
| Elastic Cloud ID support | Complete |
| 3 output formats (text, JSON, SpectreHub) | Complete |
| Config file + init command | Complete |
| CI pipeline (test/lint/build) | Complete |
| Homebrew distribution | Complete |
| SARIF output | Planned |
| Cost estimation per finding | Planned |
| v1.0 release | Planned |

Pre-1.0: CLI flags and config schemas may change between minor versions. JSON output structure (`spectre/v1`) is stable.


## Known limitations

- **No cost estimation yet.** Unlike cloud-provider spectre tools, ElasticSpectre does not estimate dollar savings per finding. Elasticsearch pricing varies widely by deployment type.
- **Stale detection uses lifetime stats.** Index stats are cumulative since creation. An index that was heavily used then went idle may not be flagged if lifetime totals are high.
- **No per-index snapshot check.** Snapshot policy is checked at cluster level, not per-index.
- **Cloud ID auth only.** Username/password and API key are supported in config but not yet exposed as CLI flags.
- **No cross-cluster support.** Scans a single cluster at a time.
- **Shard size thresholds are fixed.** The 50 GB oversized shard threshold is not configurable.

