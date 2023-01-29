package main

import (
	"context"
	"net"
	"time"

	"golang.org/x/net/proxy"
)

type dialContextFunc func(ctx context.Context, network, address string) (net.Conn, error)

func NewProxyDialer() (dialContextFunc, error) {
	baseDialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	dialSocksProxy, err := proxy.SOCKS5("tcp", PROXY, nil, baseDialer)
	if err != nil {
		return nil, err
	}

	contextDialer, ok := dialSocksProxy.(proxy.ContextDialer)
	if !ok {
		return nil, err
	}

	return contextDialer.DialContext, nil
}
