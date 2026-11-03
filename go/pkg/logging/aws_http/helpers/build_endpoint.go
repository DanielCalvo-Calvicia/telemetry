// Package helpers holds the pure pieces of the AWS HTTP adapter: the endpoint, the add_log request and the level. Nothing
// here touches the network.
package helpers

// =============================================================================
// BUILD ENDPOINT
// =============================================================================

// BuildEndpoint returns the URL of the add_log endpoint: the host (which carries its scheme, as in the old adapter),
// a colon, the port and the path. It does not check anything: ValidateSettings does.
func BuildEndpoint(host string, port string, path string) string {
	return host + ":" + port + path
}
