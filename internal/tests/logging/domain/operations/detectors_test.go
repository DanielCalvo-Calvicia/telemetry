package operations_test

import (
	"testing"

	operations "github.com/DanielCalvo-Calvicia/telemetry/internal/logging/domain/operations"
)

// All the credentials below are fake: the AWS examples published in its documentation or obvious placeholders.
const (
	fake_access_key_id = "AKIAIOSFODNN7EXAMPLE"
	fake_secret_key    = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
	fake_jwt           = "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ0ZXN0In0.c2lnbmF0dXJl"
	fake_private_key   = "-----BEGIN RSA PRIVATE KEY-----\nFAKEFAKEFAKEFAKE\nFAKEFAKEFAKEFAKE\n-----END RSA PRIVATE KEY-----"
)

func Test_each_detector_hides_what_it_finds_and_keeps_the_rest(t *testing.T) {
	test_cases := []struct {
		name          string
		text_detector operations.TextDetector
		input         string
		expected      string
	}{
		{"aws access key inside a text", operations.NewAWSAccessKeyDetector(), "key " + fake_access_key_id + " used", "key *****MASKED***** used"},
		{"aws access key temporary prefix", operations.NewAWSAccessKeyDetector(), "ASIAIOSFODNN7EXAMPLE", "*****MASKED*****"},
		{"aws access key is not part of a longer word", operations.NewAWSAccessKeyDetector(), "XAKIAIOSFODNN7EXAMPLEX", "XAKIAIOSFODNN7EXAMPLEX"},
		{"text without a key", operations.NewAWSAccessKeyDetector(), "nothing to hide", "nothing to hide"},

		{"bearer header", operations.NewBearerCredentialDetector(), "Authorization: Bearer abcdef123456", "Authorization: Bearer *****MASKED*****"},
		{"bearer lower case", operations.NewBearerCredentialDetector(), "sent bearer abcdef123456 now", "sent bearer *****MASKED***** now"},
		{"basic header", operations.NewBearerCredentialDetector(), "Authorization: Basic dXNlcjpwYXNz", "Authorization: Basic *****MASKED*****"},
		{"basic as an ordinary word", operations.NewBearerCredentialDetector(), "basic configuration needed", "basic configuration needed"},
		{"token as an ordinary word", operations.NewBearerCredentialDetector(), "invalid token supplied here", "invalid token supplied here"},

		{"jwt", operations.NewJWTDetector(), "jwt=" + fake_jwt + " end", "jwt=*****MASKED***** end"},
		{"not a jwt", operations.NewJWTDetector(), "eyJ is only a prefix", "eyJ is only a prefix"},

		{"private key block", operations.NewPrivateKeyBlockDetector(), "before " + fake_private_key + " after", "before *****MASKED***** after"},
		{"private key block without a header type", operations.NewPrivateKeyBlockDetector(), "-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----", "*****MASKED*****"},
		{"cut private key block is hidden to the end", operations.NewPrivateKeyBlockDetector(), "start -----BEGIN EC PRIVATE KEY-----\nAAAA\nBBBB", "start *****MASKED*****"},
		{"a public key is not hidden", operations.NewPrivateKeyBlockDetector(), "-----BEGIN PUBLIC KEY-----\nAAAA\n-----END PUBLIC KEY-----", "-----BEGIN PUBLIC KEY-----\nAAAA\n-----END PUBLIC KEY-----"},

		{"connection string password", operations.NewConnectionStringPasswordDetector(), "postgres://admin:hunter2@db.local:5432/app", "postgres://admin:*****MASKED*****@db.local:5432/app"},
		{"url without credentials", operations.NewConnectionStringPasswordDetector(), "https://example.com:8080/path", "https://example.com:8080/path"},
		{"url with only a user", operations.NewConnectionStringPasswordDetector(), "https://user@example.com/path", "https://user@example.com/path"},

		{"password pair", operations.NewCredentialPairDetector(), "password=hunter2 user=bob", "password=*****MASKED***** user=bob"},
		{"prefixed name", operations.NewCredentialPairDetector(), "db_password: s3cret!", "db_password: *****MASKED*****"},
		{"upper case name", operations.NewCredentialPairDetector(), "API_KEY=abc123", "API_KEY=*****MASKED*****"},
		{"json pair", operations.NewCredentialPairDetector(), `{"token": "abc 123", "user": "bob"}`, `{"token": *****MASKED*****, "user": "bob"}`},
		{"single quoted pair", operations.NewCredentialPairDetector(), "secret='my value' next", "secret=*****MASKED***** next"},
		{"aws secret key pair", operations.NewCredentialPairDetector(), "aws_secret_access_key = " + fake_secret_key, "aws_secret_access_key = *****MASKED*****"},
		{"presigned url signature", operations.NewCredentialPairDetector(), "X-Amz-Signature=abcdef012345&X-Amz-Expires=300", "X-Amz-Signature=*****MASKED*****&X-Amz-Expires=300"},
		{"name that only contains a keyword at the start", operations.NewCredentialPairDetector(), "secret_id=abc token_count=5", "secret_id=abc token_count=5"},
	}

	for _, test_case := range test_cases {
		t.Run(test_case.name, func(t *testing.T) {
			if actual := test_case.text_detector.Apply(test_case.input); actual != test_case.expected {
				t.Errorf("Apply(%q) = %q, want %q", test_case.input, actual, test_case.expected)
			}
		})
	}
}

func Test_detector_names(t *testing.T) {
	test_cases := []struct {
		text_detector operations.TextDetector
		expected_name string
	}{
		{operations.NewAWSAccessKeyDetector(), "aws_access_key"},
		{operations.NewBearerCredentialDetector(), "bearer_credential"},
		{operations.NewJWTDetector(), "jwt"},
		{operations.NewPrivateKeyBlockDetector(), "private_key_block"},
		{operations.NewConnectionStringPasswordDetector(), "connection_string_password"},
		{operations.NewCredentialPairDetector(), "credential_pair"},
	}

	for _, test_case := range test_cases {
		if test_case.text_detector.Name() != test_case.expected_name {
			t.Errorf("Name() = %q, want %q", test_case.text_detector.Name(), test_case.expected_name)
		}
	}
}
