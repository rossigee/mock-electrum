package handler

import (
	"testing"
)

func TestElectrumHandler_ServerVersion(t *testing.T) {
	cfg := &Config{
		DefaultBalance:   100000000,
		DefaultHistoryLen: 10,
	}
	h := NewElectrumHandler(cfg)

	result, err := h.serverVersion(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	version, ok := result.([]string)
	if !ok {
		t.Fatal("expected []string result")
	}
	if len(version) != 2 {
		t.Fatalf("expected 2 version elements, got %d", len(version))
	}
	if version[0] != "mock-electrum/1.0" {
		t.Errorf("expected mock-electrum/1.0, got %s", version[0])
	}
}

func TestElectrumHandler_GetBalance_KnownScripthash(t *testing.T) {
	cfg := &Config{
		DefaultBalance:   100000000,
		DefaultHistoryLen: 10,
	}

	addresses := map[string]TestAddress{
		"a3f8e8c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8": {
			Address:    "bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu",
			Scripthash: "a3f8e8c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8",
			Type:       "bech32",
		},
	}

	utxos := map[string][]UTXO{
		"bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu": {
			{TXHash: "abc123", TXPos: 0, Value: 50000000, Height: 800000},
			{TXHash: "789abc", TXPos: 1, Value: 25000000, Height: 801000},
		},
	}

	h := NewElectrumHandlerWithFixtures(cfg, addresses, utxos)

	result, err := h.getBalance("a3f8e8c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	balance, ok := result.(Balance)
	if !ok {
		t.Fatal("expected Balance result")
	}
	if balance.Confirmed != 75000000 {
		t.Errorf("expected 75000000, got %d", balance.Confirmed)
	}
}

func TestElectrumHandler_GetBalance_UnknownAddress(t *testing.T) {
	cfg := &Config{
		DefaultBalance:   100000000,
		DefaultHistoryLen: 10,
	}
	h := NewElectrumHandler(cfg)

	result, err := h.getBalance("unknown_address")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	balance, ok := result.(Balance)
	if !ok {
		t.Fatal("expected Balance result")
	}
	if balance.Confirmed != 100000000 {
		t.Errorf("expected default 100000000, got %d", balance.Confirmed)
	}
}

func TestElectrumHandler_ServerFeatures(t *testing.T) {
	cfg := &Config{
		GenesisHash: "000000000019d6689c085ae165831de934c64aae2274baeef00fac1b1dc2b4da",
	}
	h := NewElectrumHandler(cfg)

	result, err := h.serverFeatures(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	features, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map result")
	}

	if features["genesis_hash"] != cfg.GenesisHash {
		t.Errorf("expected genesis hash %s, got %v", cfg.GenesisHash, features["genesis_hash"])
	}
}

func TestElectrumHandler_ListUnspent_KnownScripthash(t *testing.T) {
	cfg := &Config{
		DefaultBalance:   100000000,
		DefaultHistoryLen: 10,
	}

	addresses := map[string]TestAddress{
		"a3f8e8c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8": {
			Address:    "bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu",
			Scripthash: "a3f8e8c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8",
			Type:       "bech32",
		},
	}

	utxos := map[string][]UTXO{
		"bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu": {
			{TXHash: "abc123", TXPos: 0, Value: 50000000, Height: 800000},
			{TXHash: "789abc", TXPos: 1, Value: 25000000, Height: 801000},
		},
	}

	h := NewElectrumHandlerWithFixtures(cfg, addresses, utxos)

	result, err := h.listUnspent("a3f8e8c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	utxosResult, ok := result.([]UTXO)
	if !ok {
		t.Fatal("expected []UTXO result")
	}
	if len(utxosResult) != 2 {
		t.Errorf("expected 2 UTXOs, got %d", len(utxosResult))
	}
}

func TestElectrumHandler_HeadersSubscribe(t *testing.T) {
	cfg := &Config{}
	h := NewElectrumHandler(cfg)

	result, err := h.headersSubscribe(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	headers, ok := result.(map[string]interface{})
	if !ok {
		t.Fatal("expected map result")
	}

	if _, ok := headers["hex"]; !ok {
		t.Error("expected hex field")
	}
}