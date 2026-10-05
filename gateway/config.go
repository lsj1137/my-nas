package main

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Listen       string              `yaml:"listen"`
	PublicOrigin string              `yaml:"public_origin"`
	Session      SessionConfig       `yaml:"session"`
	IPMap        []IPMapEntry        `yaml:"ip_map"`
	Users        map[string]UserConf `yaml:"users"`

	ipMap []ipRule
}

type SessionConfig struct {
	IdleTimeout     time.Duration `yaml:"idle_timeout"`
	AbsoluteTimeout time.Duration `yaml:"absolute_timeout"`
}

type IPMapEntry struct {
	CIDR string `yaml:"cidr"`
	User string `yaml:"user"`
}

type UserConf struct {
	PasswordHash string `yaml:"password_hash"`
	// Login이 false면 IP 매핑 전용 계정 (비밀번호 로그인 불가)
	Login *bool `yaml:"login"`
}

func (u UserConf) CanLogin() bool {
	return (u.Login == nil || *u.Login) && u.PasswordHash != ""
}

// 사용자 이름은 Quantum의 사용자 폴더 이름(/users/<이름>)으로도 쓰이므로 경로 문자를 막는다.
var validUsername = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,31}$`)

type ipRule struct {
	prefix netip.Prefix
	user   string
}

func LoadConfig(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := &Config{
		Listen: "127.0.0.1:8086",
		Session: SessionConfig{
			IdleTimeout:     7 * 24 * time.Hour,
			AbsoluteTimeout: 30 * 24 * time.Hour,
		},
	}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("invalid %s: %w", path, err)
	}
	return cfg, nil
}

func (c *Config) validate() error {
	u, err := url.Parse(c.PublicOrigin)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.Path != "" {
		return errors.New("public_origin must look like https://nas.example.com")
	}
	if len(c.Users) == 0 {
		return errors.New("no users defined")
	}
	for name := range c.Users {
		if !validUsername.MatchString(name) {
			return fmt.Errorf("user %q: use 1-32 chars of a-z, 0-9, '.', '_', '-' (starting with a letter or digit)", name)
		}
		// File Browser Quantum 1.5.x는 이름이 admin인 proxy 사용자를 자동으로 관리자로 만든다.
		if name == "admin" {
			return errors.New(`user name "admin" is reserved`)
		}
	}
	c.ipMap = c.ipMap[:0]
	for _, e := range c.IPMap {
		p, err := netip.ParsePrefix(e.CIDR)
		if err != nil {
			addr, aerr := netip.ParseAddr(e.CIDR)
			if aerr != nil {
				return fmt.Errorf("ip_map: bad cidr %q", e.CIDR)
			}
			p = netip.PrefixFrom(addr, addr.BitLen())
		}
		if _, ok := c.Users[e.User]; !ok {
			return fmt.Errorf("ip_map: unknown user %q", e.User)
		}
		c.ipMap = append(c.ipMap, ipRule{prefix: p.Masked(), user: e.User})
	}
	return nil
}

// UserForIP는 ip_map에서 처음 일치하는 사용자를 돌려준다.
func (c *Config) UserForIP(ip string) (string, bool) {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return "", false
	}
	addr = addr.Unmap()
	for _, r := range c.ipMap {
		if r.prefix.Contains(addr) {
			return r.user, true
		}
	}
	return "", false
}
