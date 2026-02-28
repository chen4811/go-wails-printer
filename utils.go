package main

import (
	"encoding/json"
)

// parseJSON 解析 JSON
func parseJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// formatJSON 格式化 JSON
func formatJSON(v interface{}) []byte {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil
	}
	return data
}
