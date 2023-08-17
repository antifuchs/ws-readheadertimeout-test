package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"

	"tailscale.com/tsnet"
	"tailscale.com/types/logger"
)

// TestViaTSnetProxy sets up a reverse proxy on the tailnet identified
// by an auth key on the OS environment, using a name also passed on
// the os environment. This test requires these credentials & names,
// sorry.
func TestViaTSnetDirect(t *testing.T) {
	svcName := os.Getenv("TSNET_DIRECT_SRV_NAME")
	tsAuthkey := os.Getenv("TS_AUTHKEY")
	if svcName == "" {
		t.Fatal("Need $TSNET_DIRECT_SRV_NAME on the environment")
	}
	if tsAuthkey == "" {
		t.Fatal("Need $TS_AUTHKEY on the environment")
	}

	fn := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}
		defer c.Close(websocket.StatusInternalError, "the sky is falling")

		ctx, cancel := context.WithTimeout(r.Context(), time.Second*10)
		defer cancel()

		var v interface{}
		err = wsjson.Read(ctx, c, &v)
		if err != nil {
			t.Fatal(err)
		}
		err = wsjson.Write(ctx, c, v)
		if err != nil {
			t.Fatal(err)
		}
		c.Close(websocket.StatusNormalClosure, "")
	})

	// Server that listens on the tsnet:
	srv := &tsnet.Server{
		Hostname:   svcName,
		Logf:       logger.Discard, // t.Logf,
		Ephemeral:  true,
		ControlURL: os.Getenv("TS_URL"),
		AuthKey:    tsAuthkey,
	}
	serverCtx, cancelServer := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelServer()
	status, err := srv.Up(serverCtx)
	if err != nil {
		t.Fatal(err)
	}
	proxyTS := http.Server{
		Handler:           fn,
		ReadHeaderTimeout: 1 * time.Second, // change this to 0 to make the test succeed
	}
	listener, err := srv.Listen("tcp", ":80")
	if err != nil {
		t.Fatal(err)
	}
	go proxyTS.Serve(listener)
	// wait for it to be up:
	addr := fmt.Sprintf("%s:80", status.TailscaleIPs[0])
BringUp:
	for {
		dialer := &net.Dialer{}
		t.Logf("Trying to dial %v", addr)
		ctx, cancel := context.WithTimeout(serverCtx, 1*time.Second)
		c, err := dialer.DialContext(ctx, "tcp", addr)
		cancel()
		switch {
		case err == nil:
			t.Log("Success, server is ready")
			c.Close()
			break BringUp
		case serverCtx.Err() != nil:
			t.Fatal("tsnet service didn't start listening within the deadline")
		default:
			t.Logf("Error %v / %v, retrying", err, errors.Unwrap(err))
			time.Sleep(20 * time.Millisecond)
		}
	}

	// Send a bit of data to the server, through the proxy:
	url, err := url.Parse(fmt.Sprintf("ws://%s", addr))
	if err != nil {
		t.Fatal(err)
	}
	url.Scheme = "ws"

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, url.String(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close(websocket.StatusInternalError, "the sky is falling")

	t.Log("Dialed")
	err = wsjson.Write(ctx, c, "hi")
	if err != nil {
		t.Fatal(err)
	}
	var v string
	err = wsjson.Read(ctx, c, &v)
	if err != nil {
		t.Fatal(err)
	}
	if v != "hi" {
		t.Fail()
	}
	t.Logf("Read back %#v", v)
	c.Close(websocket.StatusNormalClosure, "")

	// Shut down the http server:
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	proxyTS.Shutdown(cancelCtx)
}
