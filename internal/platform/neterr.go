package platform

import (
	"errors"
	"net"
	"net/url"
)

func isNetErr(err error) bool {
	var ne net.Error
	var ue *url.Error
	return errors.As(err, &ne) || errors.As(err, &ue)
}
