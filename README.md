# elasticspectre

[![CI](https://github.com/ppiankov/elasticspectre/actions/workflows/ci.yml/badge.svg)](https://github.com/ppiankov/elasticspectre/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ppiankov/elasticspectre)](https://goreportcard.com/report/github.com/ppiankov/elasticspectre)

**elasticspectre** — Elasticsearch and OpenSearch waste auditor. Part of [SpectreHub](https://github.com/ppiankov/spectrehub).

## What it is

- Audits Elasticsearch and OpenSearch clusters for stale indices, shard sprawl, and missing lifecycle policies
- Detects unassigned shards, oversized shards, replica waste, and frozen candidates
- Checks snapshot policies and authentication status
- Estimates storage and heap savings per finding
- Outputs text, JSON, and SpectreHub formats

## What it is NOT

- Not a monitoring tool — point-in-time auditor
- Not a remediation tool — reports only, never modifies the cluster
- Not a performance tuner — flags waste, not query optimization
- Not a security scanner — checks auth status, not RBAC

## Quick start

### Homebrew

```sh
brew tap ppiankov/tap
brew install elasticspectre
```

### From source

```sh
git clone https://github.com/ppiankov/elasticspectre.git
cd elasticspectre
make build
```

### Usage

```sh
elasticspectre audit --url http://localhost:9200 --format json
```

## CLI commands

| Command | Description |
|---------|-------------|
| `elasticspectre audit` | Audit cluster for waste and hygiene issues |
| `elasticspectre init` | Generate config file |
| `elasticspectre version` | Print version |

## SpectreHub integration

elasticspectre feeds Elasticsearch/OpenSearch waste findings into [SpectreHub](https://github.com/ppiankov/spectrehub) for unified visibility across your infrastructure.

```sh
spectrehub collect --tool elasticspectre
```

## Safety

elasticspectre operates in **read-only mode**. It inspects and reports — never modifies, deletes, or alters your indices.

## License

MIT — see [LICENSE](LICENSE).

---

Built by [Obsta Labs](https://github.com/ppiankov)
