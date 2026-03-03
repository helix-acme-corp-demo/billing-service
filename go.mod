module github.com/helix-acme-corp-demo/billing-service

go 1.22

require (
	github.com/go-chi/chi/v5 v5.2.1
	github.com/helix-acme-corp-demo/cachex v0.0.0
	github.com/helix-acme-corp-demo/envelope v0.0.0
	github.com/helix-acme-corp-demo/logpipe v0.0.0
	github.com/helix-acme-corp-demo/retryx v0.0.0
)

replace (
	github.com/helix-acme-corp-demo/cachex => ../cachex
	github.com/helix-acme-corp-demo/envelope => ../envelope
	github.com/helix-acme-corp-demo/logpipe => ../logpipe
	github.com/helix-acme-corp-demo/retryx => ../retryx
)
