package whois

import (
	"fmt"
	"net"
	"time"

	"github.com/pkg/errors"
)

func Client(host, port, fqdn string) (string, error) {
	addr := fmt.Sprintf("%s:%s", host, port)

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return "", errors.WithMessagef(err, "can't connect to: %s", addr)
	}

	err = writeToConnection(conn, time.Millisecond*100, fqdn)
	if err != nil {
		return "", errors.WithMessage(err, "can't send to connect")
	}

	return readFromConnection(conn, 4096, time.Second*5)
}
