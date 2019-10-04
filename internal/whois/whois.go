package whois

import (
	"fmt"
	"io"
	"net"
	"regexp"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"golang.org/x/net/idna"

	"gitlab.esta.spb.ru/arseny/whois-proxy/internal/config"
	"gitlab.esta.spb.ru/arseny/whois-proxy/internal/server"
	"gitlab.esta.spb.ru/arseny/whois-proxy/internal/storage"
)

// it's dirty pkg - I know =/ , but it is logical to test

type (
	ProxyWhoisServer struct {
		server *server.Server
		cfg    *config.Service
		logger *logrus.Logger
		cache  *storage.WhoisDataStorage

		defaultWhoisHost string
		defaultWhoisPort string
	}
)

func NewWhoisProxyServer(cfg *config.Service, logger *logrus.Logger) (*ProxyWhoisServer, error) {
	tcpServer, err := server.New("tcp", cfg.Host, cfg.Port, cfg.MaxCntConnect)
	if err != nil {
		return nil, errors.WithMessagef(err, "can't create new tcp server")
	}

	return &ProxyWhoisServer{
		server:           tcpServer,
		cfg:              cfg,
		logger:           logger,
		cache:            storage.New(time.Duration(cfg.CacheTTL)*time.Second, time.Duration(cfg.CacheReset)*time.Second),
		defaultWhoisHost: strings.Split(cfg.DefaultWhois, ":")[0],
		defaultWhoisPort: strings.Split(cfg.DefaultWhois, ":")[1],
	}, nil
}

func (w *ProxyWhoisServer) Start() error {
	chErr := make(chan error)
	go func() {
		for {
			err := <-chErr
			w.logger.WithError(err).Error("tcp server problem")
		}
	}()

	err := w.server.ListenAndServe(w.TCPHandler, chErr)
	if err != nil {
		return errors.WithMessagef(err, "can't ListenAndServe for whois server")
	}
	w.logger.Infof("Whois Proxy Server starts at %s", w.server.Addr())

	return nil
}

func (w *ProxyWhoisServer) TCPHandler(conn net.Conn) (err error) {
	var (
		request  string
		response string
	)

	defer func() {
		if err != nil { // чтобы не затирать входящую ошибку
			_ = conn.Close()
		} else {
			err = conn.Close()
		}
	}()

	request, err = readFromConnection(conn, w.cfg.MaxLenBuffer, time.Duration(w.cfg.ReadTimeout)*time.Second)
	if err != nil {
		_ = writeToConnection(conn, time.Duration(w.cfg.WriteTimeout)*time.Second, "error read socket")
		return err
	}

	if len(request) < 2 {
		_ = writeToConnection(conn, time.Duration(w.cfg.WriteTimeout)*time.Second, "empty request")
		return nil // disable this error, because it's raise by TCP health-check usually
	}

	response, err = w.processRequest(request)

	err = writeToConnection(conn, time.Duration(w.cfg.WriteTimeout)*time.Second, response)

	return err
}

func (w *ProxyWhoisServer) processRequest(request string) (string, error) {
	w.logger.Debugf("Request: %s", request)
	fqdn := strings.Split(request, "\r\n")[0]

	// TODO подумать над этим местом, по-хорошему проверка нужна
	// err := isValidHostname()
	// if err != nil {
	//	 w.logger.Warningf("Hostname not valid (regexp): %s", fqdn)
	//	 return fmt.Sprintf(w.cfg.ErrorMsgTemplate, fqdn), nil // think about response
	// }

	fqdn, err := convertToPunycode(fqdn)
	if err != nil {
		w.logger.Warningf("Hostname not valid (idna: ToASCII): %s", fqdn)
		return fmt.Sprintf(w.cfg.ErrorMsgTemplate, fqdn), nil // think about response
	}

	// determining which server will apply for who who info
	whoisHost, whoisPort, err := w.getWhoisServer(fqdn)
	if err != nil {
		return "", errors.WithMessagef(err, "error while getWhoisServer()")
	}
	w.logger.Debugf("whoisServer: %s:%s", whoisHost, whoisPort)

	// get whois info (from cache or make request to whoisServer)
	whoisInfo, err := w.getWhoisInfoCached(fqdn, whoisHost, whoisPort)
	if err != nil {
		return "", err
	}

	return whoisInfo, nil
}

func convertToPunycode(fqdn string) (string, error) {
	p := idna.New(
		idna.MapForLookup(),
		idna.Transitional(true),
		idna.StrictDomainName(true))

	// convert to idna.Punycode
	return p.ToASCII(fqdn)
}

//nolint:gocritic
func (w *ProxyWhoisServer) getWhoisServer(fqdn string) (string, string, error) {
	domainZones := getPossibleDomainZone(fqdn)
	w.logger.Debug("domainZones:", domainZones)
	if len(domainZones) == 0 {
		return "", "", errors.New("len(domainZones) == 0")
	}

	for _, zone := range domainZones {
		if addr, found := w.cfg.DomainZoneWhois[zone]; found {
			return strings.Split(addr, ":")[0], strings.Split(addr, ":")[1], nil
		}
	}

	return w.defaultWhoisHost, w.defaultWhoisPort, nil
}

// https://play.golang.org/p/BPYT1SZN1cA
// super.site.beget.ru --> [site.beget.ru  beget.ru  ru]
func getPossibleDomainZone(fqdn string) []string {
	s := strings.Split(fqdn, ".")
	if len(s) > 1 {
		return recursiveAdd(s[1:])
	}
	return []string{}
}

func recursiveAdd(s []string) (r []string) {
	if len(s) > 0 {
		r = append(r, strings.Join(s, "."))
		r = append(r, recursiveAdd(s[1:])...)
	}
	return
}

//nolint:unused,deadcode
func isValidHostname(fqdn string) bool {
	l := len(fqdn)
	if l > 254 {
		return false
	}

	if strings.HasSuffix(fqdn, ".") {
		fqdn = fqdn[:l-1] //  # strip exactly one dot from the right, if present
	}

	labels := strings.Split(fqdn, ".")

	r := regexp.MustCompile(`[0-9]+$`) // the TLD must be not all-numeric
	if r.MatchString(labels[len(labels)-1]) {
		return false
	}

	// Golang does not support Perl syntax ((?
	// will throw out :
	// error parsing regexp: invalid or unsupported Perl syntax: `(?!`
	// patternStr := "^((?!-)[A-Za-z0-9-]{1,63}(?<!-)\\.)+[A-Za-z]{2,6}$"
	// use regular expression without Perl syntax
	//nolint:lll
	domainRegExp := regexp.MustCompile(`^(([a-zA-Z]{1})|([a-zA-Z]{1}[a-zA-Z]{1})|([a-zA-Z]{1}[0-9]{1})|([0-9]{1}[a-zA-Z]{1})|([a-zA-Z0-9][a-zA-Z0-9-_]{1,61}[a-zA-Z0-9]))\.([a-zA-Z]{2,6}|[a-zA-Z0-9-]{2,30}\.[a-zA-Z]{2,3})$`)
	for _, l := range labels {
		if !domainRegExp.MatchString(l) {
			return false
		}
	}
	return true
}

func (w *ProxyWhoisServer) getWhoisInfo(fqdn, server, port string) (string, error) {
	return w.whoisRequest(fqdn, server, port)
}

func (w *ProxyWhoisServer) getWhoisInfoCached(fqdn, server, port string) (string, error) {
	var err error

	whoisInfo, found := w.cache.Get(fqdn)
	w.logger.Debugf("found from cache: %v", found)
	if !found {
		whoisInfo, err = w.getWhoisInfo(fqdn, server, port)
		if err != nil {
			return "", errors.WithMessagef(err, "error while getWhoisInfo()")
		}

		// Add Beget custom fields for whois
		if addInfo, found := w.cfg.AddWhoisDescInfo[fqdn]; found {
			whoisInfo, err = addCustomWhoisInfo(whoisInfo, addInfo)
			if err != nil {
				return "", errors.WithMessagef(err, "error while addCustomWhoisInfo()")
			}
		}
		w.cache.Set(fqdn, whoisInfo)
	}

	return whoisInfo, err
}

func addCustomWhoisInfo(originWhoisText string, customInfo []string) (string, error) {
	var modifyWhoisText strings.Builder
	for _, line := range strings.Split(originWhoisText, "\n") {
		_, err := fmt.Fprintf(&modifyWhoisText, "%s\n", line)
		if err != nil {
			return "", err
		}

		if strings.HasPrefix(line, "source:") {
			for _, v := range customInfo {
				_, err := fmt.Fprintf(&modifyWhoisText, "%s\n", v)
				if err != nil {
					return "", err
				}
			}
		}
	}

	return modifyWhoisText.String(), nil
}

func readFromConnection(conn net.Conn, maxLenBuf int, timeout time.Duration) (string, error) {
	err := conn.SetReadDeadline(time.Now().Add(timeout))
	if err != nil {
		return "", errors.WithMessagef(err, "error while SetReadDeadline()")
	}

	buf := make([]byte, 0, maxLenBuf)
	tmp := make([]byte, 256)
	for {
		n, err := conn.Read(tmp)
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", errors.WithMessage(err, "error while read tcp connect")
		}

		// защита от флуда
		if len(buf)+n > maxLenBuf {
			return "", errors.WithMessage(err, "buffer for read - overflow")
		}

		buf = append(buf, tmp[:n]...)

		if n > 2 && tmp[n-2] == '\r' && tmp[n-1] == '\n' { // признак конца \r\n
			break
		}
	}

	return string(buf), nil
}

func writeToConnection(conn net.Conn, timeout time.Duration, s string) error {
	err := conn.SetWriteDeadline(time.Now().Add(timeout))
	if err != nil {
		return nil
	}
	_, err = fmt.Fprintf(conn, "%s\r\n", s)
	return errors.WithMessagef(err, "error while writeToConnection()")
}

func (w *ProxyWhoisServer) whoisRequest(fqdn, host, port string) (string, error) {
	fqdn = strings.Trim(strings.TrimSpace(fqdn), ".")
	if fqdn == "" {
		return "", fmt.Errorf("domain is empty")
	}

	result, err := w.query(fqdn, host, port)
	if err != nil {
		return "", err
	}

	return result, nil
}

func (w *ProxyWhoisServer) query(domain, host, port string) (result string, err error) {
	var conn net.Conn
	conn, err = net.DialTimeout("tcp", net.JoinHostPort(host, port), time.Second*30)
	if err != nil {
		return "", err
	}

	defer func() {
		if err != nil { // чтобы не затирать входящую ошибку
			_ = conn.Close()
		} else {
			err = conn.Close()
		}
	}()

	err = writeToConnection(conn, time.Duration(w.cfg.WriteTimeout)*time.Second, fmt.Sprintf("%s\r\n", domain))
	if err != nil {
		return "", err
	}

	result, err = readFromConnection(conn, w.cfg.MaxLenBuffer, time.Duration(w.cfg.ReadTimeout)*time.Second)

	return result, err
}
