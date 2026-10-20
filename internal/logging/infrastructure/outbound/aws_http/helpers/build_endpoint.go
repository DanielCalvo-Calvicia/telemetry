package helpers

// BuildEndpoint joins the host (with its scheme), the port and the path into the URL of the AWS log receiver,
// for example "http://aws-microservice" + "8080" + "/api/v1/cloudwatch/logs/add" gives "http://aws-microservice:8080/api/v1/cloudwatch/logs/add".
func BuildEndpoint(host string, port string, path string) string {
	return host + ":" + port + path
}
