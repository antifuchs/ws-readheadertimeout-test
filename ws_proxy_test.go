package main

import (
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"
	"time"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

func TestViaProxy(t *testing.T) {
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
			t.Log(err)
			t.Fail()
			return
		}
		err = wsjson.Write(ctx, c, v)
		if err != nil {
			t.Log(err)
			t.Fail()
			return
		}
		c.Close(websocket.StatusNormalClosure, "")
	})

	ts := httptest.NewServer(fn)

	// reverse proxy to send data through:
	backendURL, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatal(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(backendURL)
	proxyTS := httptest.NewUnstartedServer(proxy)
	proxyTS.Config.ReadHeaderTimeout = 1 * time.Second
	proxyTS.Start()
	defer proxyTS.Close()

	// Send a bit of data to the server, through the proxy:
	url, err := url.Parse(proxyTS.URL)
	if err != nil {
		t.Fatal(err)
	}
	url.Scheme = "ws"

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
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
}
