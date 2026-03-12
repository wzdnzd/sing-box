package link_test

import (
	"testing"

	"github.com/sagernet/sing-box/common/link"
)

func TestAnyTLS(t *testing.T) {
	runTests(t, link.ParseAnyTLS, TestCases[*link.AnyTLS]{
		{
			Link: "anytls://letmein@example.com/?sni=real.example.com#remarks",
			Want: &link.AnyTLS{
				Auth:    "letmein",
				Host:    "example.com",
				Port:    443,
				SNI:     "real.example.com",
				Remarks: "remarks",
			},
		},
		{
			Link: "anytls://letmein@example.com/?sni=127.0.0.1&insecure=1#remarks",
			Want: &link.AnyTLS{
				Auth:     "letmein",
				Host:     "example.com",
				Port:     443,
				SNI:      "127.0.0.1",
				Insecure: true,
				Remarks:  "remarks",
			},
		},
		{
			Link: "anytls://0fdf77d7-d4ba-455e-9ed9-a98dd6d5489a@[2409:8a71:6a00:1953::615]:8964/?insecure=1#remarks",
			Want: &link.AnyTLS{
				Auth:     "0fdf77d7-d4ba-455e-9ed9-a98dd6d5489a",
				Host:     "2409:8a71:6a00:1953::615",
				Port:     8964,
				Insecure: true,
				Remarks:  "remarks",
			},
		},
	})
}
