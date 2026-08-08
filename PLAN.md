# Mock Electrum Server - Implementation Plan

## Overview

Mock Electrum server implementing the ElectrumX JSON-RPC protocol for Odoo crypto_investment module testing. Eliminates dependency on external BlockCypher/Etherscan APIs.

## Goals

- Provide deterministic test data for BTC balance/transaction queries
- Support Electrum protocol methods used by crypto_investment
- Run standalone without Bitcoin Core or external dependencies
- Sub-100MB Docker image with health checks

## Architecture

```
┌─────────────────┐     JSON-RPC      ┌──────────────────┐
│  Odoo Tests     │ ───────────────► │  mock-electrum   │
│ (crypto_invest) │                  │  (Go/Gin)        │
└─────────────────┘                  └────────┬─────────┘
                                              │
                                              ▼
                                       ┌──────────────────┐
                                       │  Fixtures        │
                                       │  (test data)     │
                                       └──────────────────┘
```

## Protocol Implementation

### Required ElectrumX Methods

| Method | Description | Response Format |
|--------|-------------|-----------------|
| `server.version` | Protocol version | `["mock-electrum/1.0", "1.4"]` |
| `server.features` | Server capabilities | Dictionary with genesis_hash, hosts, etc. |
| `blockchain.scripthash.get_balance` | Address balance | `{"confirmed": 100000, "unconfirmed": 0}` (satoshis) |
| `blockchain.scripthash.get_history` | TX history | `[{"height": 800000, "tx_hash": "..."}]` |
| `blockchain.scripthash.listunspent` | UTXO set | `[{"tx_hash": "...", "tx_pos": 0, "value": 50000, "height": 800000}]` |
| `blockchain.transaction.get` | Raw transaction | Hex string |
| `blockchain.headers.subscribe` | Block notifications | `{hex, height}` (optional, can be static) |

### Script Hash Calculation

Convert BTC address to script hash:
```python
import hashlib

def address_to_scripthash(address):
    # Legacy: OP_DUP OP_HASH160 <20 bytes> OP_EQUALVERIFY OP_CHECKSIG
    # SegWit: OP_HASH160 <20 bytes> OP_EQUAL  
    # Native: OP_0 <32 bytes>
    
    if address.startswith('bc1'):
        # Native SegWit - bech32
        # For testing, use SHA256 of address
        return hashlib.sha256(address.encode()).hexdigest()
    else:
        # Legacy/P2SH - base58
        from .base58 import base58_to_script
        script = base58_to_script(address)
        return hashlib.sha256(bytes.fromhex(script)).hexdigest()
```

## File Structure

```
services/mock-electrum/
├── cmd/main/main.go              # Entry point, JSON-RPC server
├── internal/
│   ├── handler/
│   │   ├── electrum.go           # Electrum method handlers
│   │   ├── scripthash.go         # Address→scripthash conversion
│   │   └── health.go             # /health, /ready endpoints
│   └── middleware/
│       └── middleware.go         # Request logging, recovery
├── fixtures/
│   ├── test_addresses.json       # Known test addresses + derivations
│   └── test_utxos.json           # Mock UTXO data (satoshis)
├── k8s/
│   └── manifest.yaml             # K8s Deployment + Service
├── Dockerfile                    # Multi-stage build, scratch base
├── go.mod                        # Go 1.26.4
├── go.sum
├── Makefile                      # lint, test, run, docker-build
├── .github/workflows/build.yaml  # CI: lint → test → build
└── docs/README.md                # API documentation
```

## Configuration

| Env Variable | Default | Description |
|--------------|---------|-------------|
| `LOG_LEVEL` | `info` | debug, info, warn, error |
| `PORT` | `50001` | TCP port for Electrum protocol |
| `GENESIS_HASH` | `000000000019d6689c085ae165831de934c...` | BTC genesis block |
| `DEFAULT_BALANCE` | `100000000` | 1 BTC in satoshis for unknown addresses |
| `DEFAULT_HISTORY_LEN` | `10` | Number of mock txs to return |

## Test Fixtures

### `fixtures/test_addresses.json`

```json
{
  "addresses": [
    {
      "address": "bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu",
      "scripthash": "a3f8e8c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8",
      "type": "bech32",
      "derivation_path": "m/84'/0'/0'/0/0"
    },
    {
      "address": "1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa",
      "scripthash": "...",
      "type": "p2pkh"
    }
  ]
}
```

### `fixtures/test_utxos.json`

```json
{
  "utxos": {
    "bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu": [
      {
        "tx_hash": "abc123def456...",
        "tx_pos": 0,
        "value": 50000000,
        "height": 800000
      },
      {
        "tx_hash": "789abc123456...",
        "tx_pos": 1,
        "value": 25000000,
        "height": 801000
      }
    ]
  }
}
```

## Dockerfile

```dockerfile
FROM golang:1.26.4-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o electrum-mock ./cmd/main

FROM scratch
COPY --from=builder /build/electrum-mock /app
COPY --from=builder /build/fixtures /fixtures
EXPOSE 50001
ENTRYPOINT ["/app"]
```

## Kubernetes Manifest

```yaml
apiVersion: v1
kind: Service
metadata:
  name: mock-electrum
  namespace: mock-services
spec:
  type: ClusterIP
  ports:
    - port: 50001
      targetPort: 50001
  selector:
    app: mock-electrum
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mock-electrum
  namespace: mock-services
spec:
  replicas: 1
  selector:
    matchLabels:
      app: mock-electrum
  template:
    spec:
      containers:
        - name: mock-electrum
          image: mock-electrum:latest
          ports:
            - containerPort: 50001
          env:
            - name: LOG_LEVEL
              value: "info"
          livenessProbe:
            tcpSocket:
              port: 50001
            initialDelaySeconds: 5
          readinessProbe:
            tcpSocket:
              port: 50001
            initialDelaySeconds: 3
```

## Integration with Odoo Tests

### Option A: Model Mocking (Recommended)

Create test that patches `crypto.blockchain_api`:

```python
# In crypto_investment/tests/test_blockchain_api.py
import pytest
from unittest.mock import patch

class TestBlockchainAPI:
    
    @patch('odoo.addons.crypto_investment.models.crypto_blockchain_api.requests.get')
    def test_get_btc_balance_uses_electrum(self, mock_get):
        mock_response = Mock()
        mock_response.json.return_value = {
            'confirmed': 100000000,
            'unconfirmed': 0
        }
        mock_get.return_value = mock_response
        
        result = self.env['crypto.blockchain.api'].get_btc_balance(test_address)
        assert result['balance'] == 1.0
```

### Option B: Config Parameter

Add to `crypto.blockchain_api`:

```python
def get_btc_balance(self, address):
    electrum_url = self.env['ir.config_parameter'].sudo().get_param(
        'crypto.electrum.url', 'https://api.blockcypher.com'
    )
    if 'blockcypher' not in electrum_url:
        return self._get_balance_electrum(electrum_url, address)
    ...
```

### Option C: Direct Electrum Client

Add `crypto.electrum.client` model:

```python
class CryptoElectrumClient(models.AbstractModel):
    _name = 'crypto.electrum.client'
    
    def connect(self, host='localhost', port=50001):
        import jsonrpc
        return jsonrpc.Client(f"http://{host}:{port}")
    
    def get_balance(self, address):
        client = self.connect()
        scripthash = self.address_to_scripthash(address)
        return client.call('blockchain.scripthash.get_balance', scripthash)
```

## Testing Strategy

1. **Unit Tests**: Test scripthash conversion, fixture loading
2. **Protocol Tests**: Verify JSON-RPC responses match ElectrumX spec
3. **Integration Tests**: Odoo calls mock-electrum, verifies balance/tx parsing

## Success Criteria

- [ ] Mock server starts on port 50001
- [ ] Returns valid Electrum protocol responses
- [ ] All defined test addresses return expected balances
- [ ] Health endpoints respond at `/health`, `/ready`
- [ ] Docker image builds to <50MB
- [ ] K8s manifest deploys successfully
- [ ] Odoo tests can query mock for balance/history

## Time Estimate

- **Setup**: 30 min
- **Core Protocol**: 2 hours
- **Fixtures**: 30 min
- **K8s + Docker**: 30 min
- **Integration**: 1 hour
- **Total**: ~4-5 hours