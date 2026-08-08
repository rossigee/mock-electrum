package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"os"

	"github.com/rossigee/mock-electrum/internal/handler/scripthash"
)

type Config struct {
	LogLevel        string `yaml:"log_level"`
	Port            string `yaml:"port"`
	GenesisHash     string `yaml:"genesis_hash"`
	DefaultBalance  int64  `yaml:"default_balance"`
	DefaultHistoryLen int   `yaml:"default_history_len"`
}

func LoadConfig() *Config {
	cfg := &Config{
		LogLevel:         "info",
		Port:             "50001",
		GenesisHash:      "000000000019d6689c085ae165831de934c64aae2274baeef00fac1b1dc2b4da",
		DefaultBalance:   100000000,
		DefaultHistoryLen: 10,
	}

	data, err := os.ReadFile("/config.yaml")
	if err != nil {
		slog.Debug("no config file found, using defaults")
		return cfg
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		slog.Warn("failed to parse config, using defaults", slog.Any("error", err))
	}
	return cfg
}

type ElectrumHandler struct {
	cfg        *Config
	addresses  map[string]TestAddress
	utxos      map[string][]UTXO
	historyLen int
}

type TestAddress struct {
	Address     string `json:"address"`
	Scripthash  string `json:"scripthash"`
	Type        string `json:"type"`
	Derivation  string `json:"derivation_path,omitempty"`
}

type UTXO struct {
	TXHash  string `json:"tx_hash"`
	TXPos   int    `json:"tx_pos"`
	Value   int64  `json:"value"`
	Height  int    `json:"height"`
}

type Balance struct {
	Confirmed   int64 `json:"confirmed"`
	Unconfirmed int64 `json:"unconfirmed"`
}

type HistoryItem struct {
	Height  int    `json:"height"`
	TXHash  string `json:"tx_hash"`
}

type Transaction struct {
	Hex string `json:"hex"`
}

func NewElectrumHandler(cfg *Config) *ElectrumHandler {
	h := &ElectrumHandler{
		cfg:         cfg,
		addresses:   make(map[string]TestAddress),
		utxos:       make(map[string][]UTXO),
		historyLen:  cfg.DefaultHistoryLen,
	}

	h.loadFixtures()
	return h
}

func NewElectrumHandlerWithFixtures(cfg *Config, addresses map[string]TestAddress, utxos map[string][]UTXO) *ElectrumHandler {
	h := &ElectrumHandler{
		cfg:         cfg,
		addresses:   addresses,
		utxos:       utxos,
		historyLen:  cfg.DefaultHistoryLen,
	}
	return h
}

func (h *ElectrumHandler) loadFixtures() {
	h.loadAddresses()
	h.loadUTXOs()
}

func (h *ElectrumHandler) loadAddresses() {
	data, err := os.ReadFile("/fixtures/test_addresses.json")
	if err != nil {
		slog.Debug("no addresses fixture found")
		return
	}

	var fixture struct {
		Addresses []TestAddress `json:"addresses"`
	}

	if err := json.Unmarshal(data, &fixture); err != nil {
		slog.Warn("failed to parse addresses fixture", slog.Any("error", err))
		return
	}

	for _, addr := range fixture.Addresses {
		h.addresses[addr.Address] = addr
		h.addresses[addr.Scripthash] = addr
	}
	slog.Info("loaded addresses", slog.Int("count", len(h.addresses)))
}

func (h *ElectrumHandler) loadUTXOs() {
	data, err := os.ReadFile("/fixtures/test_utxos.json")
	if err != nil {
		slog.Debug("no UTXOs fixture found")
		return
	}

	var fixture struct {
		UTXOs map[string][]UTXO `json:"utxos"`
	}

	if err := json.Unmarshal(data, &fixture); err != nil {
		slog.Warn("failed to parse UTXOs fixture", slog.Any("error", err))
		return
	}

	h.utxos = fixture.UTXOs
	slog.Info("loaded UTXOs", slog.Int("count", len(h.utxos)))
}

func (h *ElectrumHandler) HandleMethod(method string, params interface{}) (interface{}, error) {
	switch method {
	case "server.version":
		return h.serverVersion(params)
	case "server.features":
		return h.serverFeatures(params)
	case "blockchain.scripthash.get_balance":
		return h.getBalance(params)
	case "blockchain.scripthash.get_history":
		return h.getHistory(params)
	case "blockchain.scripthash.listunspent":
		return h.listUnspent(params)
	case "blockchain.transaction.get":
		return h.getTransaction(params)
	case "blockchain.headers.subscribe":
		return h.headersSubscribe(params)
	default:
		return nil, errors.New("method not found")
	}
}

func (h *ElectrumHandler) serverVersion(params interface{}) (interface{}, error) {
	return []string{"mock-electrum/1.0", "1.4"}, nil
}

func (h *ElectrumHandler) serverFeatures(params interface{}) (interface{}, error) {
	return map[string]interface{}{
		"genesis_hash": h.cfg.GenesisHash,
		"hosts": map[string]interface{}{
			"localhost": map[string]interface{}{
				"tcp_port":  50001,
				"ssl_port":  0,
			},
		},
		"server_version":   "mock-electrum/1.0",
		"protocol_min":     "1.4",
		"protocol_max":     "1.4",
		"pruning":          nil,
		"default_port":     50001,
		"description":      "Mock Electrum Server for Testing",
		"fee_estimation":   true,
		"etags":            true,
		"compaction":       true,
		"historical_fees":  true,
		"mempool_feerate":  true,
		"recent_reversed_headers": true,
		"validate_proof":  true,
	}, nil
}

func (h *ElectrumHandler) getBalance(params interface{}) (interface{}, error) {
	scripthash, err := h.extractStringParam(params)
	if err != nil {
		return nil, err
	}

	if addr, ok := h.addresses[scripthash]; ok {
		utxos := h.utxos[addr.Address]
		var total int64
		for _, utxo := range utxos {
			total += utxo.Value
		}
		return Balance{Confirmed: total, Unconfirmed: 0}, nil
	}

	return Balance{Confirmed: h.cfg.DefaultBalance, Unconfirmed: 0}, nil
}

func (h *ElectrumHandler) getHistory(params interface{}) (interface{}, error) {
	scripthash, err := h.extractStringParam(params)
	if err != nil {
		return nil, err
	}

	var addr string
	if a, ok := h.addresses[scripthash]; ok {
		addr = a.Address
	} else {
		addr = scripthash
	}

	utxos := h.utxos[addr]
	if len(utxos) == 0 {
		history := make([]HistoryItem, h.historyLen)
		for i := 0; i < h.historyLen; i++ {
			hash := sha256.Sum256([]byte(addr + string(rune(i))))
			history[i] = HistoryItem{
				Height:  800000 + i*1000,
				TXHash:  hex.EncodeToString(hash[:]),
			}
		}
		return history, nil
	}

	history := make([]HistoryItem, len(utxos))
	seen := make(map[string]bool)
	for i, utxo := range utxos {
		if !seen[utxo.TXHash] {
			history[i] = HistoryItem{
				Height: utxo.Height,
				TXHash: utxo.TXHash,
			}
			seen[utxo.TXHash] = true
		}
	}

	return history, nil
}

func (h *ElectrumHandler) listUnspent(params interface{}) (interface{}, error) {
	scripthash, err := h.extractStringParam(params)
	if err != nil {
		return nil, err
	}

	if addr, ok := h.addresses[scripthash]; ok {
		if utxos, ok := h.utxos[addr.Address]; ok {
			return utxos, nil
		}
	}

	return []UTXO{}, nil
}

func (h *ElectrumHandler) getTransaction(params interface{}) (interface{}, error) {
	txHash, err := h.extractStringParam(params)
	if err != nil {
		return nil, err
	}

	mockTx := "010000000001" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		"00" +
		"ffffffff" +
		"02" +
		"203aba0100000000001976a914" + txHash[:40] + "88ac" +
		"00ca9a3b00000000001976a914" + txHash[40:80] + "88ac" +
		"024730440220" + txHash[:64] + "0220" + txHash[64:] + "014104" + txHash[:64] +
		"ffffffff"

	return Transaction{Hex: mockTx}, nil
}

func (h *ElectrumHandler) headersSubscribe(params interface{}) (interface{}, error) {
	headerHex := "000000208c11b0d9a0c1f8c7a2e1d3f5a8b9c0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5" +
		"0000000000000000000000000000000000000000000000000000000000000000" +
		"4d04ffff001d"

	return map[string]interface{}{
		"hex":   headerHex,
		"count": 800000,
	}, nil
}

func (h *ElectrumHandler) extractStringParam(params interface{}) (string, error) {
	if params == nil {
		return "", errors.New("missing params")
	}

	switch p := params.(type) {
	case string:
		return p, nil
	case []interface{}:
		if len(p) == 0 {
			return "", errors.New("missing param")
		}
		if s, ok := p[0].(string); ok {
			return s, nil
		}
		return "", errors.New("param must be string")
	default:
		return scripthash.AddressToScripthash(p.(string))
	}
}