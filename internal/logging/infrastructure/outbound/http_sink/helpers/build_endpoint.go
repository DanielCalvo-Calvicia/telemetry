package helpers

// BuildEndpoint joins the host (with its scheme), the port and the path into the URL of the log receiver,
// for example "http://localhost" + "3000" + "/logs" gives "http://localhost:3000/logs".
func BuildEndpoint(host string, port string, path string) string {
	return host + ":" + port + path
}
