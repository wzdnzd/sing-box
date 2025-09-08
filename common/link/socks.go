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

var _ Link = (*Socks)(nil)

func init() {
	common.Must(RegisterParser(&Parser{
		Name:   "Socks",
		Scheme: []string{"socks4", "socks5"},
		Parse: func(u *url.URL) (Link, error) {
			return ParseSocks(u)
		},
	}))
}

// Socks represents a parsed socks link
type Socks struct {
	Version  string `json:"version,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Host     string `json:"host,omitempty"`
	Port     uint16 `json:"port,omitempty"`
	Remarks  string `json:"remarks,omitempty"`
}

// ParseSocks parses a socks link
func ParseSocks(u *url.URL) (*Socks, error) {
	link := &Socks{}
	switch u.Scheme {
	case "socks4":
		link.Version = "4"
	case "socks5":
		link.Version = "5"
	default:
		return nil, E.New("not a socks link")
	}
	port, err := strconv.ParseUint(u.Port(), 10, 16)
	if err != nil {
		return nil, E.Cause(err, "invalid port")
	}
	link.Host = u.Hostname()
	link.Port = uint16(port)
	link.Remarks = u.Fragment

	if uname := u.User.Username(); uname != "" {
		link.Username = uname
	}
	if pass, ok := u.User.Password(); ok {
		link.Password = pass
	}
	return link, nil
}

// Outbound implements Link
func (l *Socks) Outbound() (*option.Outbound, error) {
	opt := &option.SOCKSOutboundOptions{
		ServerOptions: option.ServerOptions{
			Server:     l.Host,
			ServerPort: l.Port,
		},
		Version: l.Version,
	}
	if l.Username != "" {
		opt.Username = l.Username
	}
	if l.Password != "" {
		opt.Password = l.Password
	}
	return &option.Outbound{
		Type:    C.TypeSOCKS,
		Tag:     l.Remarks,
		Options: opt,
	}, nil
}

// URL implements Link
func (l *Socks) URL() (string, error) {
	var uri url.URL
	switch l.Version {
	case "4":
		uri.Scheme = "socks4"
	case "5":
		uri.Scheme = "socks5"
	}
	uri.Host = fmt.Sprintf("%s:%d", l.Host, l.Port)
	uri.Fragment = l.Remarks

	if l.Username != "" || l.Password != "" {
		uri.User = url.UserPassword(l.Username, l.Password)
	}
	return uri.String(), nil
}
