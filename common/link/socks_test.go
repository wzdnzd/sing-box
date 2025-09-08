package link_test

import (
	"testing"

	"github.com/sagernet/sing-box/common/link"
)

func TestSocks(t *testing.T) {
	runTests(t, link.ParseSocks, TestCases[*link.Socks]{
		{
			Link: "socks5://user:pass@example.com:1080#remark1",
			Want: &link.Socks{
				Version:  "5",
				Username: "user",
				Password: "pass",
				Host:     "example.com",
				Port:     1080,
				Remarks:  "remark1",
			},
		},
		{
			Link: "socks4://example.com:1080#remark2",
			Want: &link.Socks{
				Version: "4",
				Host:    "example.com",
				Port:    1080,
				Remarks: "remark2",
			},
		},
		{
			Link: "socks5://example.com:1080",
			Want: &link.Socks{
				Version: "5",
				Host:    "example.com",
				Port:    1080,
			},
		},
		{
			Link: "socks5://example.com:1080#🇺🇸",
			Want: &link.Socks{
				Version: "5",
				Host:    "example.com",
				Port:    1080,
				Remarks: "🇺🇸",
			},
		},
	})
}