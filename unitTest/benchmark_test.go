package unitTest

import (
	"testing"
	easyCon "github.com/qiu-tec/easy-con.golang"
)

func BenchmarkNewProtocolMarshal(b *testing.B) {
	req := easyCon.PackReq{
		From:    "ModuleA",
		To:      "ModuleB",
		Route:   "benchmarkRoute",
		ReqTime: "2024-01-16 10:00:00.000",
		Content: make([]byte, 1024), // 1KB content
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = req.Raw()
	}
}

func BenchmarkNewProtocolUnmarshal(b *testing.B) {
	req := easyCon.PackReq{
		From:    "ModuleA",
		To:      "ModuleB",
		Route:   "benchmarkRoute",
		ReqTime: "2024-01-16 10:00:00.000",
		Content: make([]byte, 1024),
	}
	data, _ := req.Raw()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = easyCon.UnmarshalPack(data)
	}
}
