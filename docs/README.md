# Mock Electrum Server

Mock Electrum server implementing the ElectrumX JSON-RPC protocol for Odoo crypto_investment module testing.

## Protocol

This mock implements the ElectrumX protocol over TCP (port 50001). JSON-RPC messages are line-delimited.

### Supported Methods

| Method | Description |
|--------|-------------|
| `server.version` | Returns protocol version |
| `server.features` | Server capabilities and configuration |
| `blockchain.scripthash.get_balance` | Get balance for a scripthash |
| `blockchain.scripthash.get_history` | Get transaction history |
| `blockchain.scripthash.listunspent` | Get UTXO set |
| `blockchain.transaction.get` | Get raw transaction |
| `blockchain.headers.subscribe` | Subscribe to block headers |

### Example Usage

```python
import socket
import json

def electrum_request(method, params=None):
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.connect(('localhost', 50001))
    
    request = {"id": 1, "method": method, "params": params or []}
    sock.sendall((json.dumps(request) + '\n').encode())
    
    response = sock.recv(4096).decode()
    sock.close()
    return json.loads(response)

# Get balance
result = electrum_request("blockchain.scripthash.get_balance", ["a3f8e8c1d2e3f4a5..."])

# Get history
result = electrum_request("blockchain.scripthash.get_history", ["a3f8e8c1d2e3f4a5..."])
```

## Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `LOG_LEVEL` | info | debug, info, warn, error |
| `PORT` | 50001 | TCP port for Electrum protocol |
| `HTTP_PORT` | 8081 | HTTP port for health endpoints |
| `GENESIS_HASH` | 000000... | BTC genesis block hash |
| `DEFAULT_BALANCE` | 100000000 | Default balance in satoshis |

## Running

### Local

```bash
make run
```

### Docker

```bash
make docker-run
```

### Kubernetes

```bash
kubectl apply -f k8s/manifest.yaml
```

## Health Endpoints

HTTP health endpoints are available on port 8081:

- `GET /health` - Liveness probe
- `GET /ready` - Readiness probe

## Test Fixtures

Test data is loaded from `/fixtures/`:
- `test_addresses.json` - Known test addresses with pre-computed scripthashes
- `test_utxos.json` - Mock UTXO data in satoshis