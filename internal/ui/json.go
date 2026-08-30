package ui

import (
	"encoding/json"

	"github.com/DiAndEn0/tunnel-snoop/internal/model"
)

func FormatJSON(tunnels []model.Tunnel) ([]byte, error) {
	return json.MarshalIndent(tunnels, "", "  ")
}
