"""Render-only checks for App rollout strategies, storage topologies and backing-service wiring.

Run with Helm on PATH and PyYAML installed:
    python3 -m unittest discover -s helm/tests -v
"""

import os
from pathlib import Path
import re
import subprocess
import tempfile
import unittest

import yaml


CHART = Path(os.environ.get("HELM_CHART_DIR", Path(__file__).resolve().parents[1]))


CHART_SECRETS = ["--set", "secrets.dbPassword=db", "--set", "secrets.redisPassword=redis"]


def render(values=None, secrets=("--set", "secrets.existingSecret=fixture-secret")):
    with tempfile.TemporaryDirectory() as directory:
        arguments = []
        for index, layer in enumerate(values if isinstance(values, list) else [values or {}]):
            path = Path(directory) / f"values-{index}.yaml"
            path.write_text(yaml.safe_dump(layer))
            arguments.extend(["-f", str(path)])
        return subprocess.run(
            [
                "helm", "template", "audit", str(CHART),
                "--namespace", "audit",
                *secrets,
                *arguments,
            ],
            capture_output=True, text=True,
        )


class AppStorageTests(unittest.TestCase):
    def documents(self, values=None, **kwargs):
        result = render(values, **kwargs)
        self.assertEqual(result.returncode, 0, result.stderr)
        return [doc for doc in yaml.safe_load_all(result.stdout) if doc]

    def app(self, documents):
        return next(doc for doc in documents
                    if doc["kind"] == "Deployment"
                    and doc["metadata"]["name"] == "audit-ai-kb-app")

    def assert_data_claim(self, app, name):
        volumes = app["spec"]["template"]["spec"]["volumes"]
        volume = next(v for v in volumes if v["name"] == "data-files")
        self.assertEqual(volume["persistentVolumeClaim"]["claimName"], name)

    def readme_values(self, filename):
        blocks = re.findall(r"```yaml\n(.*?)\n```", (CHART / "README.md").read_text(), re.S)
        return yaml.safe_load(next(block for block in blocks
                                   if block.startswith(f"# {filename}\n")))

    def env(self, values):
        app = self.app(self.documents(values))
        return {e["name"]: e.get("value") for e in app["spec"]["template"]["spec"]["containers"][0]["env"]}

    def test_default_strategy_recreates_with_chart_pvc(self):
        for values in ({}, {"app": {"strategy": None}}):
            app = self.app(self.documents(values))
            self.assertEqual(app["spec"]["strategy"], {"type": "Recreate"}, values)

    def test_default_strategy_rolls_on_shared_or_no_volume(self):
        for data_files in ({"accessMode": "ReadWriteMany"}, {"enabled": False}):
            app = self.app(self.documents({"dataFiles": {"persistence": data_files}}))
            self.assertEqual(app["spec"]["strategy"], {
                "type": "RollingUpdate",
                "rollingUpdate": {"maxSurge": 1, "maxUnavailable": 0},
            }, data_files)

    def test_existing_single_attach_claim_still_recreates(self):
        app = self.app(self.documents({"dataFiles": {"persistence": {"existingClaim": "x"}}}))
        self.assertEqual(app["spec"]["strategy"], {"type": "Recreate"})

    def test_explicit_rolling_update_overrides_default(self):
        app = self.app(self.documents({"app": {"strategy": {"type": "RollingUpdate"}}}))
        self.assertEqual(app["spec"]["strategy"]["type"], "RollingUpdate")

    def test_access_mode_reaches_the_chart_pvc(self):
        documents = self.documents({"dataFiles": {"persistence": {"accessMode": "ReadWriteMany"}}})
        pvc = next(doc for doc in documents if doc["kind"] == "PersistentVolumeClaim"
                   and doc["metadata"]["name"] == "audit-ai-kb-data-files")
        self.assertEqual(pvc["spec"]["accessModes"], ["ReadWriteMany"])

    def test_external_database_host_and_port(self):
        env = self.env({"postgresql": {"enabled": False},
                        "externalDatabase": {"host": "db.example", "port": 5433}})
        self.assertEqual((env["DB_HOST"], env["DB_PORT"]), ("db.example", "5433"))

    def test_disabled_components_require_external_addresses(self):
        for values, message in (
            ({"postgresql": {"enabled": False}}, "externalDatabase.host is required"),
            ({"redis": {"enabled": False}}, "externalRedis.addr is required"),
        ):
            result = render(values)
            self.assertNotEqual(result.returncode, 0, values)
            self.assertIn(message, result.stderr)

    def test_bundled_database_ignores_external_host(self):
        env = self.env({"externalDatabase": {"host": "db.example", "port": 5433}})
        self.assertEqual((env["DB_HOST"], env["DB_PORT"]), ("postgres", "5432"))

    def test_external_redis_addr(self):
        env = self.env({"redis": {"enabled": False}, "externalRedis": {"addr": "cache:6380"}})
        self.assertEqual(env["REDIS_ADDR"], "cache:6380")
        self.assertEqual(self.env({})["REDIS_ADDR"], "redis:6379")

    def secret_checksums(self, jwt):
        documents = self.documents({"neo4j": {"enabled": True, "password": "neo"}},
                                   secrets=[*CHART_SECRETS, "--set", f"secrets.jwtSecret={jwt}"])
        return {doc["metadata"]["name"]: doc["spec"]["template"]["metadata"].get("annotations", {}).get("checksum/secret")
                for doc in documents if doc["kind"] in ("Deployment", "StatefulSet")}

    def test_app_carries_a_stable_secret_checksum(self):
        first, second = self.secret_checksums("one"), self.secret_checksums("two")
        self.assertEqual([name for name, checksum in first.items() if checksum], ["audit-ai-kb-app"])
        self.assertRegex(first["audit-ai-kb-app"], r"^[0-9a-f]{64}$")
        self.assertNotEqual(first["audit-ai-kb-app"], second["audit-ai-kb-app"])
        self.assertEqual(first, self.secret_checksums("one"))

    def test_extra_volumes_and_mounts(self):
        volume = {"name": "builtin-models", "configMap": {"name": "models"}}
        mount = {"name": "builtin-models", "mountPath": "/config/builtin_models.yaml",
                 "subPath": "builtin_models.yaml"}
        pod = self.app(self.documents({"app": {"extraVolumes": [volume],
                                               "extraVolumeMounts": [mount]}}))["spec"]["template"]["spec"]
        self.assertEqual(pod["volumes"][-1], volume)
        self.assertEqual(pod["containers"][0]["volumeMounts"][-1], mount)

    def test_recreate_omits_rolling_update(self):
        app = self.app(self.documents({"app": {"strategy": {"type": "Recreate"}}}))
        self.assertEqual(app["spec"]["strategy"], {"type": "Recreate"})

    def test_custom_rolling_update_preserves_zero_and_percentage(self):
        app = self.app(self.documents({"app": {"strategy": {
            "type": "RollingUpdate",
            "rollingUpdate": {"maxSurge": 0, "maxUnavailable": "25%"},
        }}}))
        self.assertEqual(app["spec"]["strategy"]["rollingUpdate"], {
            "maxSurge": 0, "maxUnavailable": "25%",
        })

    def test_invalid_strategy_is_rejected(self):
        result = render({"app": {"strategy": {"type": "Invalid"}}})
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("app.strategy.type must be RollingUpdate or Recreate", result.stderr)

    def test_production_example_uses_single_replica_rwo_and_recreate(self):
        documents = self.documents(self.readme_values("values-production.yaml"))
        app = self.app(documents)
        self.assertEqual(app["spec"]["replicas"], 1)
        self.assertEqual(app["spec"]["strategy"], {"type": "Recreate"})
        self.assert_data_claim(app, "audit-ai-kb-data-files")
        pvc = next(doc for doc in documents
                   if doc["kind"] == "PersistentVolumeClaim"
                   and doc["metadata"]["name"] == "audit-ai-kb-data-files")
        self.assertEqual(pvc["spec"]["accessModes"], ["ReadWriteOnce"])
        env = app["spec"]["template"]["spec"]["containers"][0]["env"]
        self.assertIn({"name": "STORAGE_TYPE", "value": "local"}, env)

    def test_shared_storage_example_references_existing_claim(self):
        values = self.readme_values("values-production.yaml")
        override = self.readme_values("values-shared-storage.yaml")
        documents = self.documents([values, override])
        app = self.app(documents)
        self.assertEqual(app["spec"]["replicas"], 3)
        self.assertEqual(app["spec"]["strategy"]["type"], "RollingUpdate")
        self.assert_data_claim(app, "ai-kb-shared-files")
        self.assertFalse(any(doc["kind"] == "PersistentVolumeClaim"
                             and doc["metadata"]["name"] == "audit-ai-kb-data-files"
                             for doc in documents))


if __name__ == "__main__":
    unittest.main()
