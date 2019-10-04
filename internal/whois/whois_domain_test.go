package whois

import (
	"testing"
)

// go test -covermode=count -coverprofile=coverage.cov && go tool cover -html=coverage.cov

func Test_getPossibleDomainZone(t *testing.T) {
	testCases := []struct {
		domain  string
		results []string
	}{
		{"example.com", []string{"com"}},
		{"test.example.com", []string{"example.com", "com"}},
		{"test.test.example.com", []string{"test.example.com", "example.com", "com"}},
		{"com", []string{}},
		{".com", []string{"com"}},
	}

	for n, test := range testCases {
		res := getPossibleDomainZone(test.domain)
		if len(res) != len(test.results) {
			t.Fatalf("unxpected len of result test #%d: %s", n, test.domain)
		}
		for i, r := range res {
			if r != test.results[i] {
				t.Errorf("unxpected result for test case #%d: %s!=%s #%d", n, r, test.results[i], i)
			}
		}
	}
}

func Test_convertToPunycode(t *testing.T) {
	testCases := []struct {
		domain string
		err    error
		result string
	}{
		{"example.com", nil, "example.com"},
		{"test.example.com", nil, "test.example.com"},
		{"test.test.example.com", nil, "test.test.example.com"},
		{"окна.рф", nil, "xn--80atjc.xn--p1ai"},
		{"a.bc", nil, "a.bc"},
		{"xn--kxae4bafwg.xn--pxaix.gr", nil, "xn--kxae4bafwg.xn--pxaix.gr"},
		{"subdomain.subdomainsubdomainsuèdomainsubdomainsubdomainsubdomainsubdomain.net", nil,
			"subdomain.xn--subdomainsubdomainsudomainsubdomainsubdomainsubdomainsubdomain-1mf.net"},
	}

	for n, test := range testCases {
		puny, err := convertToPunycode(test.domain)
		if err != test.err {
			t.Errorf("unxpected error status: %v <> %v", err, test.err)
		}
		if puny != test.result {
			t.Errorf("unxpected result for test case #%d: %s <> %s", n, puny, test.result)
		}
	}
}
