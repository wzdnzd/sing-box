package link

import (
	"fmt"
	"net/url"
	"strconv"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

var _ Link = (*HTTP)(nil)

func init() {
	common.Must(RegisterParser(&Parser{
		Name:   "HTTP",
		Scheme: []string{"http", "https"},
		Parse: func(u *url.URL) (Link, error) {
			return ParseHTTP(u)
		},
	}))
}

// HTTP represents a parsed http link
type HTTP struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Host     string `json:"host,omitempty"`
	Port     uint16 `json:"port,omitempty"`
	TLS      bool   `json:"tls,omitempty"`
	Remarks  string `json:"remarks,omitempty"`
}

// ParseHTTP parses a http link
func ParseHTTP(u *url.URL) (*HTTP, error) {
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, E.New("not a http link")
	}
	port, err := strconv.ParseUint(u.Port(), 10, 16)
	if err != nil {
		return nil, E.Cause(err, "invalid port")
	}
	link := &HTTP{}
	link.Host = u.Hostname()
	link.Port = uint16(port)
	link.Remarks = u.Fragment
	link.TLS = u.Scheme == "https"
	if uname := u.User.Username(); uname != "" {
		link.Username = uname
	}
	if pass, ok := u.User.Password(); ok {
		link.Password = pass
	}
	return link, nil
}

// Outbound implements Link
func (l *HTTP) Outbound() (*option.Outbound, error) {
	opt := &option.HTTPOutboundOptions{
		ServerOptions: option.ServerOptions{
			Server:     l.Host,
			ServerPort: l.Port,
		},
	}
	if l.Username != "" {
		opt.Username = l.Username
	}
	if l.Password != "" {
		opt.Password = l.Password
	}
	if l.TLS {
		opt.TLS = &option.OutboundTLSOptions{
			Enabled: true,
		}
	}
	return &option.Outbound{
		Type:    C.TypeHTTP,
		Tag:     l.Remarks,
		Options: opt,
	}, nil
}

// URL implements Link
func (l *HTTP) URL() (string, error) {
	var uri url.URL
	if l.TLS {
		uri.Scheme = "https"
	} else {
		uri.Scheme = "http"
	}
	uri.Host = fmt.Sprintf("%s:%d", l.Host, l.Port)
	uri.Fragment = l.Remarks

	if l.Username != "" || l.Password != "" {
		uri.User = url.UserPassword(l.Username, l.Password)
	}
	return uri.String(), nil
}