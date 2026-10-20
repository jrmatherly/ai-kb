"""Render-only checks for externally run services and the optional Secret keys."""

import unittest

from test_app_storage import CHART_SECRETS, RenderMixin, readme_values, render


EXISTING = ("--set", "secrets.existingSecret=fixture-secret")
GENERATED = [*CHART_SECRETS, "--set", "secrets.jwtSecret=jwt"]


class ExternalServicesTests(RenderMixin, unittest.TestCase):
    def documents(self, values=None, secrets=EXISTING):
        return super().documents(values, secrets=secrets)

    def container(self, documents, component):
        return self.deployment(documents, component)["spec"]["template"]["spec"]["containers"][0]

    def env(self, values=None, component="app"):
        return self.container(self.documents(values), component)["env"]

    def secret_data(self, values=None, extra=()):
        documents = self.documents(values, secrets=[*GENERATED, *extra])
        return next(doc for doc in documents if doc["kind"] == "Secret")["stringData"]

    def assert_fails(self, values, message, secrets=EXISTING):
        result = render(values, secrets=secrets)
        self.assertNotEqual(result.returncode, 0, values)
        self.assertIn(message, result.stderr)

    def test_redis_db_defaults_to_zero_and_follows_external_redis(self):
        external = {"redis": {"enabled": False}, "externalRedis": {"addr": "cache:6379", "db": 7}}
        for values, expected in (({}, "0"), (external, "7"),
                                 ({"externalRedis": {"addr": "cache:6379", "db": 7}}, "0")):
            entries = [e for e in self.env(values) if e["name"] == "REDIS_DB"]
            self.assertEqual(entries, [{"name": "REDIS_DB", "value": expected}], values)

    def test_grpc_auth_token_is_optional_on_app_and_docreader(self):
        documents = self.documents()
        for component in ("app", "docreader"):
            env = self.container(documents, component)["env"]
            self.assertIn({"name": "GRPC_AUTH_TOKEN", "valueFrom": {"secretKeyRef": {
                "name": "fixture-secret", "key": "GRPC_AUTH_TOKEN", "optional": True}}}, env, component)

    def test_generated_secret_writes_grpc_auth_token_only_when_set(self):
        self.assertNotIn("GRPC_AUTH_TOKEN", self.secret_data())
        data = self.secret_data(extra=["--set", "secrets.grpcAuthToken=tok"])
        self.assertEqual(data["GRPC_AUTH_TOKEN"], "tok")

    def test_system_signing_key_is_optional_and_never_generated(self):
        self.assertIn({"name": "SYSTEM_SIGNING_KEY", "valueFrom": {"secretKeyRef": {
            "name": "fixture-secret", "key": "SYSTEM_SIGNING_KEY", "optional": True}}}, self.env())
        self.assertNotIn("SYSTEM_SIGNING_KEY", self.secret_data())
        data = self.secret_data(extra=["--set", "secrets.systemSigningKey=sign"])
        self.assertEqual(data["SYSTEM_SIGNING_KEY"], "sign")

    def test_startup_probe_default_and_override(self):
        app = self.container(self.documents(), "app")
        self.assertEqual(app["startupProbe"], {"httpGet": {"path": "/health", "port": "http"},
                                               "periodSeconds": 5, "failureThreshold": 60})
        self.assertEqual(app["livenessProbe"]["initialDelaySeconds"], 30)
        app = self.container(self.documents({"app": {"startupProbe": {"httpGet": None, "tcpSocket": {"port": "http"},
                                                                      "failureThreshold": 120}}}), "app")
        self.assertEqual(app["startupProbe"], {"tcpSocket": {"port": "http"}, "failureThreshold": 120,
                                               "periodSeconds": 5})
        app = self.container(self.documents({"app": {"startupProbe": None}}), "app")
        self.assertNotIn("startupProbe", app)

    def test_external_neo4j_renders_env_and_secret_keys(self):
        values = {"externalNeo4j": {"uri": "bolt://graph:7687"}, "neo4j": {"username": "graph", "password": "pw"}}
        env = self.env(values)
        neo4j = [e for e in env if e["name"].startswith("NEO4J_")]
        self.assertEqual(neo4j, [
            {"name": "NEO4J_ENABLE", "value": "true"},
            {"name": "NEO4J_URI", "value": "bolt://graph:7687"},
            {"name": "NEO4J_USERNAME", "valueFrom": {"secretKeyRef": {"name": "fixture-secret", "key": "NEO4J_USERNAME"}}},
            {"name": "NEO4J_PASSWORD", "valueFrom": {"secretKeyRef": {"name": "fixture-secret", "key": "NEO4J_PASSWORD"}}},
        ])
        data = self.secret_data(values)
        self.assertEqual((data["NEO4J_USERNAME"], data["NEO4J_PASSWORD"]), ("graph", "pw"))
        self.assertFalse(any(e["name"].startswith("NEO4J_") for e in self.env()))
        self.assertNotIn("NEO4J_PASSWORD", self.secret_data())

    def test_external_neo4j_needs_the_neo4j_password_in_the_generated_secret(self):
        self.assert_fails({"externalNeo4j": {"uri": "bolt://graph:7687"}},
                          "neo4j.password is required when neo4j.enabled is true or externalNeo4j.uri is set",
                          secrets=GENERATED)

    def test_bundled_and_external_neo4j_together_fail(self):
        self.assert_fails({"neo4j": {"enabled": True, "password": "pw"},
                           "externalNeo4j": {"uri": "bolt://graph:7687"}},
                          "set either neo4j.enabled (bundled Neo4j) or externalNeo4j.uri, not both")

    def test_neo4j_credentials_change_the_app_secret_checksum(self):
        def checksum(password):
            documents = self.documents({"externalNeo4j": {"uri": "bolt://graph:7687"}, "neo4j": {"password": password}})
            return self.deployment(documents, "app")["spec"]["template"]["metadata"]["annotations"]["checksum/secret"]
        self.assertNotEqual(checksum("one"), checksum("two"))

    def test_extra_env_from_renders_env_from(self):
        self.assertNotIn("envFrom", self.container(self.documents(), "app"))
        source = {"secretRef": {"name": "provider-keys"}}
        app = self.container(self.documents({"app": {"extraEnvFrom": [source]}}), "app")
        self.assertEqual(app["envFrom"], [source])

    def test_docreader_extra_env_renders(self):
        env = self.env({"docreader": {"extraEnv": [{"name": "LOG_LEVEL", "value": "DEBUG"}]}}, "docreader")
        self.assertEqual(env[-1], {"name": "LOG_LEVEL", "value": "DEBUG"})

    def test_db_sslmode_default_override_and_validation(self):
        external = {"postgresql": {"enabled": False}, "externalDatabase": {"host": "db"}}
        cases = (({}, "disable"), (external, "disable"),
                 ({**external, "externalDatabase": {"host": "db", "sslMode": "require"}}, "require"),
                 ({"externalDatabase": {"sslMode": "require"}}, "disable"))
        for values, expected in cases:
            self.assertIn({"name": "DB_SSLMODE", "value": expected}, self.env(values), values)
        self.assert_fails({**external, "externalDatabase": {"host": "db", "sslMode": "on"}},
                          "externalDatabase.sslMode must be one of disable, require, verify-ca, verify-full")

    def test_readme_external_services_example_renders(self):
        values = readme_values("values-external-services.yaml")
        documents = self.documents(values, secrets=())
        env = {e["name"]: e.get("value") for e in self.container(documents, "app")["env"]}
        self.assertEqual((env["DB_HOST"], env["DB_SSLMODE"], env["REDIS_ADDR"], env["REDIS_DB"], env["NEO4J_URI"]),
                         ("db.example.internal", "require", "cache.example.internal:6379", "3",
                          "bolt://graph.example.internal:7687"))
        names = {doc["metadata"]["name"] for doc in documents if doc["kind"] in ("Deployment", "Secret")}
        self.assertEqual(names, {"audit-ai-kb-app", "audit-ai-kb-docreader", "audit-ai-kb-frontend",
                                 "audit-ai-kb-website"})

    def test_grpc_auth_token_change_rolls_docreader(self):
        def annotation(token):
            documents = self.documents({"secrets": {"grpcAuthToken": token}}, secrets=GENERATED)
            return self.deployment(documents, "docreader")["spec"]["template"]["metadata"]["annotations"]["checksum/grpc-auth-token"]
        self.assertNotEqual(annotation("one"), annotation("two"))

    def test_migrate_fail_fast_defaults_on_and_follows_app_env(self):
        self.assertIn({"name": "AIKB_MIGRATE_FAIL_FAST", "value": "true"}, self.env())
        for override in ("false", False):
            self.assertIn({"name": "AIKB_MIGRATE_FAIL_FAST", "value": "false"},
                          self.env({"app": {"env": {"AIKB_MIGRATE_FAIL_FAST": override}}}), override)


if __name__ == "__main__":
    unittest.main()
