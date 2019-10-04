package whois

import (
	"strings"
	"testing"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"gitlab.esta.spb.ru/arseny/whois-proxy/internal/config"
)

// go test -covermode=count -coverprofile=coverage.cov && go tool cover -html=coverage.cov

func TestNewWhoisProxyServer(t *testing.T) {
	// start TCP whois proxy server
	cfg := config.Service{
		Host:             "localhost",
		Port:             "50000",
		MaxCntConnect:    1,
		MaxLenBuffer:     4096,
		ReadTimeout:      1,
		WriteTimeout:     1,
		CacheTTL:         1,
		CacheReset:       84600,
		ErrorMsgTemplate: "",
		DefaultWhois:     "whois.myorderbox.com:43",
		DomainZoneWhois:  nil,
		AddWhoisDescInfo: nil,
	}

	whois, err := NewWhoisProxyServer(&cfg, &logrus.Logger{})
	if err != nil || whois == nil {
		t.Errorf("server not created. err: %v", err)
	}
}

func TestNewWhoisProxyServer_Negative(t *testing.T) {
	// start TCP whois proxy server
	cfg := config.Service{
		Host:          "localhost",
		Port:          "50000",
		MaxCntConnect: 1,
	}

	cfg.MaxCntConnect = 0
	_, err := NewWhoisProxyServer(&cfg, &logrus.Logger{})
	if err == nil {
		t.Error("no error")
	}
}

func TestWhoisClient(t *testing.T) {
	whoisExpectedInfo := `% By submitting a query to RIPN's Whois Service
% you agree to abide by the following terms of use:
% http://www.ripn.net/about/servpol.html#3.2 (in Russian) 
% http://www.ripn.net/about/en/servpol.html#3.2 (in English).`

	whois, err := Client("whois.tcinet.ru", "43", "example.com")
	if err != nil {
		t.Errorf("err get whois info. err: %v", err)
	}
	if strings.Join(strings.Split(whois, "\n")[0:4], "\n") != whoisExpectedInfo {
		t.Errorf("unexpecteed whois info")
	}
}

func TestWhoisClient_Negative(t *testing.T) {
	_, err := Client("localhost", "43", "example.com")
	if err == nil {
		t.Errorf("no error")
	}
}

func TestWhoisProxyServer_Start(t *testing.T) {
	// start TCP whois proxy server
	cfg := config.Service{
		Host:             "localhost",
		Port:             "50001",
		MaxCntConnect:    1,
		MaxLenBuffer:     4096,
		ReadTimeout:      1,
		WriteTimeout:     1,
		CacheTTL:         1,
		CacheReset:       84600,
		DefaultWhois:     "whois.myorderbox.com:43",
		DomainZoneWhois:  map[string]string{},
		AddWhoisDescInfo: map[string][]string{},
	}

	server, err := NewWhoisProxyServer(&cfg, &logrus.Logger{})
	if err != nil || server == nil {
		t.Errorf("proxy server whois not created. err: %v", err)
	}

	err = server.Start()
	if err != nil {
		t.Errorf("error while server.Start. err: %v", err)
	}

	_, err = Client("localhost", "50001", "example.com")
	if err != nil {
		t.Errorf("whois proxy server unavailable. err:%v", err)
	}
}

func TestWhoisProxyServer_Start_Negative(t *testing.T) {
	// start TCP whois proxy server
	cfg := config.Service{
		Host:             "abcds",
		Port:             "50000",
		MaxCntConnect:    1,
		MaxLenBuffer:     4096,
		ReadTimeout:      1,
		WriteTimeout:     1,
		CacheTTL:         1,
		CacheReset:       84600,
		DefaultWhois:     "whois.myorderbox.com:43",
		DomainZoneWhois:  map[string]string{},
		AddWhoisDescInfo: map[string][]string{},
	}

	server, err := NewWhoisProxyServer(&cfg, &logrus.Logger{})
	if err != nil || server == nil {
		t.Errorf("proxy server whois not created. err: %v", err)
	}

	err = server.Start()
	if err == nil {
		t.Error("no err about fail server.Start")
	}
}

func Test_getWhoisServer(t *testing.T) {
	testCases := []struct {
		domain     string
		host, port string
		err        error
	}{
		{"example.com", "whois.myorderbox.com", "43", nil},
		{"example.online", "whois.myorderbox.com", "43", nil},
		{"example.site", "whois.myorderbox.com", "43", nil},
		{"example.net", "whois.myorderbox.com", "43", nil},
		{"example.ru", "whois.tcinet.ru", "43", nil},
		{"example.su", "whois.tcinet.ru", "43", nil},
		{"xn--80atjc.xn--p1ai", "whois.tcinet.ru", "43", nil},
		{"", "", "", errors.New("len(domainZones) == 0")},
	}

	// start TCP whois proxy server
	cfg := config.Service{
		Host:          "localhost",
		Port:          "50000",
		MaxCntConnect: 1,
		MaxLenBuffer:  4096,
		ReadTimeout:   1,
		WriteTimeout:  1,
		CacheTTL:      1,
		CacheReset:    84600,
		DefaultWhois:  "whois.myorderbox.com:43",
		DomainZoneWhois: map[string]string{
			"ru":       "whois.tcinet.ru:43",
			"xn--p1ai": "whois.tcinet.ru:43",
			"su":       "whois.tcinet.ru:43"},
		AddWhoisDescInfo: map[string][]string{},
	}

	server, err := NewWhoisProxyServer(&cfg, &logrus.Logger{})
	if err != nil || server == nil {
		t.Errorf("proxy server whois not created. err: %v", err)
	}

	for n, test := range testCases {
		host, port, err := server.getWhoisServer(test.domain)
		if test.err == nil && err != nil || test.err != nil && err == nil {
			t.Fatalf("unxpected error result in #%d test for domain: %s  error: %v  expected err: %v",
				n, test.domain, err, test.err)
		}

		if host != test.host {
			t.Fatalf("unxpected host return in #%d test for domain: %s  host: %s  expected: %s",
				n, test.domain, host, test.host)
		}

		if port != test.port {
			t.Fatalf("unxpected port return in #%d test for domain: %s port: %s  expected: %s",
				n, test.domain, port, test.port)
		}
	}
}

func Test_WhoisServer_ALL(t *testing.T) {
	testCases := []struct {
		domain    string
		whoisInfo string
		err       error
	}{
		{"krotov.ru", "", nil},
		{"rubleff.ru", "", nil},
		{"example.online", "", nil},
		{"example.site", "", nil},
		{"example.net", "", nil},
		{"example.ru", "", nil},
		{"example.su", "", nil},
		{"xn--80atjc.xn--p1ai", "", nil},
		{"", "", errors.New("")},
	}

	// start TCP whois proxy server
	cfg := config.Service{
		Host:          "localhost",
		Port:          "50000",
		MaxCntConnect: 1,
		MaxLenBuffer:  4096,
		ReadTimeout:   30,
		WriteTimeout:  30,
		CacheTTL:      300,
		CacheReset:    84600,
		DefaultWhois:  "whois.myorderbox.com:43",
		DomainZoneWhois: map[string]string{
			"ru":       "whois.tcinet.ru:43",
			"xn--p1ai": "whois.tcinet.ru:43",
			"su":       "whois.tcinet.ru:43"},
		AddWhoisDescInfo: map[string][]string{
			"rubleff.ru": {"descr:         rubleff@gmail.com"},
			"krotov.ru":  {"descr:         Domain for sale!", "descr:         rubleff@gmail.com"}},
	}

	server, err := NewWhoisProxyServer(&cfg, &logrus.Logger{})
	if err != nil || server == nil {
		t.Errorf("proxy server whois not created. err: %v", err)
	}

	for n, test := range testCases {
		s, err := server.processRequest(test.domain)
		_ = s
		if test.err == nil && err != nil || test.err != nil && err == nil {
			t.Fatalf("unxpected error result in #%d test for domain: %s  error: %v  expected err: %v",
				n, test.domain, err, test.err)
		}
	}
}
