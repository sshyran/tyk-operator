module github.com/TykTechnologies/tyk-operator

go 1.15

require (
	github.com/cenkalti/backoff/v4 v4.1.1
	github.com/cucumber/godog v0.11.0
	github.com/go-logr/logr v0.4.0
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/client-go v0.21.2
	k8s.io/utils v0.0.0-20210527160623-6fdb442a123b
	moul.io/http2curl/v2 v2.2.2
	sigs.k8s.io/controller-runtime v0.9.3
	sigs.k8s.io/e2e-framework v0.0.2
	sigs.k8s.io/gateway-api v0.3.0
)
