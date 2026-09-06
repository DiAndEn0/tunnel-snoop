package ui

import (
	"encoding/json"

	"github.com/DiAndEn0/tunnel-snoop/internal/model"
)

// FormatJSON serializes a slice of tunnels to indented JSON. A nil slice is
// normalized to an empty slice to ensure valid JSON array output (`[]`).
func FormatJSON(tunnels []model.Tunnel) ([]byte, error) {
	if tunnels == nil {
		tunnels = []model.Tunnel{}
	}
	return json.MarshalIndent(tunnels, "", "  ")
}
