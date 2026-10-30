"""The ported tests of the rule of the sensitive field names, and the new active list."""

import unittest

from telemetry.internal.domain.operations.masking.is_sensitive_field_name import (
    is_sensitive_field_name,
    normalize_field_name,
    normalize_field_names,
)


class IsSensitiveFieldNameTest(unittest.TestCase):
    def test_the_names_of_the_old_library(self):
        for name in [
            "password",
            "db_password",
            "Password",
            "dbPassword",
            "access-key",
            "accessKey",
            "secret",
            "secret_string",
            "SecretString",
            "secret_binary",
            "client_secret",
            "api_key",
            "authorization",
            "access_token",
            "session_token",
            "request_body",
            "body",
            "message_body",
            "MessageBody",
            "content",
            "private_key",
            "x-amz-signature",
        ]:
            with self.subTest(name):
                self.assertTrue(is_sensitive_field_name(name))
        for name in [
            "",
            "key",
            "bucket",
            "secret_id",
            "secret_name",
            "secret_arn",
            "token_count",
            "queue_url",
            "error",
            "message",
            "size",
            "bodyguard",
        ]:
            with self.subTest(name):
                self.assertFalse(is_sensitive_field_name(name))

    def test_a_given_list_replaces_the_default(self):
        self.assertTrue(is_sensitive_field_name("user_pin", ["pin"]))
        self.assertFalse(is_sensitive_field_name("password", ["pin"]))
        self.assertFalse(is_sensitive_field_name("password", []))

    def test_normalization(self):
        self.assertEqual(normalize_field_name("Access-Key.Id x"), "access_key_id_x")
        self.assertEqual(normalize_field_name("dbPassword2Token"), "db_password2_token")
        self.assertEqual(normalize_field_names(["Pin", "", "SSN"]), ("pin", "ssn"))
