package util

import "encoding/json"

func Serialize(v any) []byte {
	jsonData, _ := json.Marshal(v)
	return jsonData
}

func Deserialize(b []byte, dest any) {
	json.Unmarshal(b, dest)
}
