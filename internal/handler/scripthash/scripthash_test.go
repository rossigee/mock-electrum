package scripthash

import (
	"testing"
)

func TestAddressToScripthash(t *testing.T) {
	tests := []struct {
		address string
		wantLen int
	}{
		{"bc1qcr8te4kr609gcawutmrza0j4xv80jy8z306fyu", 64},
		{"1A1zP1eP5QGefi2DMPTfTL5SLmv7DivfNa", 64},
		{"", 64},
	}

	for _, tt := range tests {
		t.Run(tt.address, func(t *testing.T) {
			got, err := AddressToScripthash(tt.address)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tt.wantLen {
				t.Errorf("AddressToScripthash() = %v, want len %d", got, tt.wantLen)
			}
		})
	}
}

func TestScriptToScripthash(t *testing.T) {
	script := "76a91488ac"
	got, err := ScriptToScripthash(script)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 64 {
		t.Errorf("ScriptToScripthash() = %v, want len 64", got)
	}
}