package utils

import "strings"

// SanitizeStorageConnectivityError converts a raw storage connectivity error
// into a safe, user-facing message. It deliberately avoids echoing the raw
// driver/network error so responses never leak internal hostnames, IPs, ports
// or TLS/certificate details. Callers that need the full error must log it
// server-side instead of returning it to the client.
func SanitizeStorageConnectivityError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "Endpoint url cannot have fully qualified paths"):
		return "Invalid endpoint: drop the http(s):// prefix and enter only host:port (e.g. minio.example.com:9000)"
	case strings.Contains(msg, "no such host"):
		return "DNS resolution failed; check that the address is correct"
	case strings.Contains(msg, "connection refused"):
		return "Connection refused; check that the service is running and the port is correct"
	case strings.Contains(msg, "no route to host"):
		return "No route to the target address; check the network configuration"
	case strings.Contains(msg, "i/o timeout") || strings.Contains(msg, "deadline exceeded") || strings.Contains(msg, "context deadline"):
		return "Connection timed out; check the network or the service status"
	case strings.Contains(msg, "403") || strings.Contains(msg, "AccessDenied") || strings.Contains(msg, "access denied"):
		return "Authentication failed; check that the access credentials are correct"
	case strings.Contains(msg, "certificate") || strings.Contains(msg, "tls") || strings.Contains(msg, "x509"):
		return "TLS/SSL certificate error; check the SSL configuration"
	case strings.Contains(msg, "404") || strings.Contains(msg, "NoSuchBucket"):
		return "Bucket does not exist; check the name and region"
	default:
		return "Connection failed; check that the configuration is correct"
	}
}
