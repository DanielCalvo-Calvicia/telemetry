"""The ported tests of the six detectors: the inputs and expected values are the ones of the old library."""

import unittest

from internal.tests.domain.operations.masking.helpers import (
    FAKE_ACCESS_KEY_ID,
    FAKE_JWT,
    FAKE_PRIVATE_KEY,
    FAKE_SECRET_KEY,
)
from telemetry.internal.domain.operations.masking.detect_aws_access_key import new_aws_access_key_detector
from telemetry.internal.domain.operations.masking.detect_bearer_credential import new_bearer_credential_detector
from telemetry.internal.domain.operations.masking.detect_connection_string_password import (
    new_connection_string_password_detector,
)
from telemetry.internal.domain.operations.masking.detect_credential_pair import new_credential_pair_detector
from telemetry.internal.domain.operations.masking.detect_jwt import new_jwt_detector
from telemetry.internal.domain.operations.masking.detect_private_key_block import new_private_key_block_detector
from telemetry.internal.domain.operations.masking.text_detector import TextDetector


class DetectorsTest(unittest.TestCase):
    def test_each_detector_hides_what_it_finds_and_keeps_the_rest(self):
        aws = new_aws_access_key_detector()
        bearer = new_bearer_credential_detector()
        jwt = new_jwt_detector()
        private_key = new_private_key_block_detector()
        connection = new_connection_string_password_detector()
        pair = new_credential_pair_detector()
        cases = [
            ("aws key inside a text", aws, "key " + FAKE_ACCESS_KEY_ID + " used", "key *****MASKED***** used"),
            ("aws temporary prefix", aws, "ASIAIOSFODNN7EXAMPLE", "*****MASKED*****"),
            ("aws key in a longer word", aws, "XAKIAIOSFODNN7EXAMPLEX", "XAKIAIOSFODNN7EXAMPLEX"),
            ("text without a key", aws, "nothing to hide", "nothing to hide"),
            ("bearer header", bearer, "Authorization: Bearer abcdef123456", "Authorization: Bearer *****MASKED*****"),
            ("bearer lower case", bearer, "sent bearer abcdef123456 now", "sent bearer *****MASKED***** now"),
            ("basic header", bearer, "Authorization: Basic dXNlcjpwYXNz", "Authorization: Basic *****MASKED*****"),
            ("basic as a word", bearer, "basic configuration needed", "basic configuration needed"),
            ("token as a word", bearer, "invalid token supplied here", "invalid token supplied here"),
            ("jwt", jwt, "jwt=" + FAKE_JWT + " end", "jwt=*****MASKED***** end"),
            ("not a jwt", jwt, "eyJ is only a prefix", "eyJ is only a prefix"),
            (
                "private key block",
                private_key,
                "before " + FAKE_PRIVATE_KEY + " after",
                "before *****MASKED***** after",
            ),
            (
                "private key block without a type",
                private_key,
                "-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----",
                "*****MASKED*****",
            ),
            (
                "cut private key block",
                private_key,
                "start -----BEGIN EC PRIVATE KEY-----\nAAAA\nBBBB",
                "start *****MASKED*****",
            ),
            (
                "a public key is not hidden",
                private_key,
                "-----BEGIN PUBLIC KEY-----\nAAAA\n-----END PUBLIC KEY-----",
                "-----BEGIN PUBLIC KEY-----\nAAAA\n-----END PUBLIC KEY-----",
            ),
            (
                "connection string password",
                connection,
                "postgres://admin:hunter2@db.local:5432/app",
                "postgres://admin:*****MASKED*****@db.local:5432/app",
            ),
            ("url without credentials", connection, "https://example.com:8080/path", "https://example.com:8080/path"),
            ("url with only a user", connection, "https://user@example.com/path", "https://user@example.com/path"),
            ("password pair", pair, "password=hunter2 user=bob", "password=*****MASKED***** user=bob"),
            ("prefixed name", pair, "db_password: s3cret!", "db_password: *****MASKED*****"),
            ("upper case name", pair, "API_KEY=abc123", "API_KEY=*****MASKED*****"),
            ("json pair", pair, '{"token": "abc 123", "user": "bob"}', '{"token": *****MASKED*****, "user": "bob"}'),
            ("single quoted pair", pair, "secret='my value' next", "secret=*****MASKED***** next"),
            (
                "aws secret key pair",
                pair,
                "aws_secret_access_key = " + FAKE_SECRET_KEY,
                "aws_secret_access_key = *****MASKED*****",
            ),
            (
                "presigned url signature",
                pair,
                "X-Amz-Signature=abcdef012345&X-Amz-Expires=300",
                "X-Amz-Signature=*****MASKED*****&X-Amz-Expires=300",
            ),
            (
                "name that only starts with a keyword",
                pair,
                "secret_id=abc token_count=5",
                "secret_id=abc token_count=5",
            ),
        ]
        for description, detector, text, expected in cases:
            with self.subTest(description):
                self.assertEqual(detector.apply(text), expected)

    def test_the_detector_names(self):
        cases = [
            (new_aws_access_key_detector(), "aws_access_key"),
            (new_bearer_credential_detector(), "bearer_credential"),
            (new_jwt_detector(), "jwt"),
            (new_private_key_block_detector(), "private_key_block"),
            (new_connection_string_password_detector(), "connection_string_password"),
            (new_credential_pair_detector(), "credential_pair"),
        ]
        for detector, expected_name in cases:
            self.assertEqual(detector.name, expected_name)

    def test_an_unknown_replacement_kind_is_refused(self):
        with self.assertRaises(ValueError):
            TextDetector("x", new_jwt_detector()._pattern, "other")
