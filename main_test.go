package main

import (
	"os"
	"testing"

	"github.com/jetstack/cert-manager/test/acme/dns"
)

var (
	resolvedZone = os.Getenv("RESOLVED_ZONE_NAME")
)

func TestRunsSuite(t *testing.T) {
	// The manifest path should contain a file named config.json that is a
	// snippet of valid configuration that should be included on the
	// ChallengeRequest passed as part of the test cases.

	fixture := dns.NewFixture(&yandexCloudDNSProviderSolver{},
		dns.SetResolvedZone(resolvedZone),
		dns.SetAllowAmbientCredentials(false),
		dns.SetStrict(true),
		dns.SetManifestPath("testdata"),
		dns.SetDNSServer("84.201.185.208:53"),
	)

	fixture.RunConformance(t)
}
