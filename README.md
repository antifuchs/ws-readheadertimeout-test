# Test repro repository for https://github.com/boinkor-net/tsnsrv/issues/21

This reduces my problem down to a fairly easily-reproducible set of test cases. What I see:

* The stdlib net listeners all work the way you'd expect: websocket
  connections can be established, whether directly or through the
  httputil.ReverseProxy.

* The listeners created by tailscale's tsnet.Server do not always
  work: The proxy listening on a tsnet listener consistently has
  websocket connections time out, and the direct http handler
  *occasionally* (but not always) times out for me.

## To run the tests

The stdlib tests don't require anything special.

The tailscale tests require the following env vars to be set:

* `TS_AUTHKEY` - an authentication key that creates a tailscale node with ACL tags that allow your local machine (also that tailnet) to connect to its :80 port.
* `TSNET_SRV_NAME` - a free machine name on your tailnet, used for the reverse proxy test
* `TSNET_DIRECT_SRV_NAME` - a free machine name on your tailnet, used for the direct connection test
