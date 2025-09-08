package link_test

import (
	"testing"

	"github.com/sagernet/sing-box/common/link"
)

func TestHTTP(t *testing.T) {
	runTests(t, link.ParseHTTP, TestCases[*link.HTTP]{
		{
			Link: "http://user:pass@example.com:8080#remark1",
			Want: &link.HTTP{
				Username: "user",
				Password: "pass",
				Host:     "example.com",
				Port:     8080,
				TLS:      false,
				Remarks:  "remark1",
			},
		},
		{
			Link: "https://user:pass@example.com:8443#remark2",
			Want: &link.HTTP{
				Username: "user",
				Password: "pass",
				Host:     "example.com",
				Port:     8443,
				TLS:      true,
				Remarks:  "remark2",
			},
		},
		{
			Link: "http://example.com:80",
			Want: &link.HTTP{
				Host: "example.com",
				Port: 80,
			},
		},
		{
			Link: "http://example.com:80#🇺🇸",
			Want: &link.HTTP{
				Host:    "example.com",
				Port:    80,
				Remarks: "🇺🇸",
			},
		},
	})
}