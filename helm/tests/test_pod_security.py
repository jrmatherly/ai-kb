"""Render-only checks that every workload fits the restricted Pod Security Standard."""

import unittest

import yaml

from test_app_storage import render

WORKLOADS = {"Deployment", "StatefulSet"}


def pods(values=None):
    result = render(values)
    assert result.returncode == 0, result.stderr
    for doc in yaml.safe_load_all(result.stdout):
        if doc and doc["kind"] in WORKLOADS:
            yield doc["metadata"]["name"], doc["spec"]["template"]["spec"]


class PodSecurityTests(unittest.TestCase):
    def test_every_pod_is_restricted_and_read_only(self):
        seen = []
        for name, spec in pods({"ingress": {"enabled": False}, "redis": {"enabled": False}}):
            seen.append(name)
            pod = spec.get("securityContext") or {}
            self.assertTrue(pod.get("runAsNonRoot"), f"{name}: pod runAsNonRoot")
            self.assertEqual((pod.get("seccompProfile") or {}).get("type"), "RuntimeDefault", f"{name}: seccomp")
            for container in spec["containers"] + spec.get("initContainers", []):
                ctx = container.get("securityContext") or {}
                label = f"{name}/{container['name']}"
                self.assertTrue(ctx.get("runAsNonRoot"), f"{label}: runAsNonRoot")
                self.assertGreater(ctx.get("runAsUser", 0), 0, f"{label}: runAsUser")
                self.assertIs(ctx.get("allowPrivilegeEscalation"), False, f"{label}: allowPrivilegeEscalation")
                self.assertEqual((ctx.get("capabilities") or {}).get("drop"), ["ALL"], f"{label}: capabilities")
                self.assertNotIn("add", ctx.get("capabilities") or {}, f"{label}: no added capabilities")
                self.assertIs(ctx.get("readOnlyRootFilesystem"), True, f"{label}: readOnlyRootFilesystem")
        self.assertEqual(sorted(seen), sorted(["audit-ai-kb-app", "audit-ai-kb-docreader", "audit-ai-kb-frontend", "audit-ai-kb-postgres", "audit-ai-kb-website"]))

    def test_bundled_redis_is_restricted_too(self):
        [(name, spec)] = [(n, s) for n, s in pods() if n.endswith("-redis")]
        ctx = spec["containers"][0]["securityContext"]
        self.assertEqual((ctx["runAsUser"], ctx["readOnlyRootFilesystem"], ctx["capabilities"]["drop"]), (999, True, ["ALL"]))
        self.assertEqual(spec["securityContext"]["fsGroup"], 999)


if __name__ == "__main__":
    unittest.main()
