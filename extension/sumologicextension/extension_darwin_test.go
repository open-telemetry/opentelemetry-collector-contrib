package sumologicextension

import "os"

func init() {
	// fqdn seems to hang running in github actions on darwin amd64. This
	// bypasses it.
	// https://github.com/SumoLogic/sumologic-otel-collector/issues/1295
	hostname = os.Hostname
}
