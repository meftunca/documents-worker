//go:build ignore

package main

// go1.25 with json v1
import (
	jsonv1 "encoding/json"
	"testing"
	// go1.25 with json v2 - Not available in current Go version
	// jsonv2 "encoding/json/v2"
)

type User struct {
	ID      int      `json:"id"`
	Name    string   `json:"name"`
	Email   string   `json:"email"`
	Tags    []string `json:"tags"`
	Active  bool     `json:"active"`
	Created int64    `json:"created"`
}

var data = []byte(`{"id":1,"name":"John Doe","email":"john@example.com","tags":["admin","user"],"active":true,"created":1693123456}`)

func BenchmarkV1Unmarshal(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var u User
		_ = jsonv1.Unmarshal(data, &u)
	}
}

// JSON v2 benchmark disabled - not available in current Go version
// func BenchmarkV2Unmarshal(b *testing.B) {
// 	for i := 0; i < b.N; i++ {
// 		var u User
// 		_ = jsonv2.Unmarshal(data, &u)
// 	}
// }
