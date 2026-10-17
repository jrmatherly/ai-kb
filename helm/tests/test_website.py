"""Render-only checks for the website component and its Ingress path."""

import unittest

import yaml

from test_app_storage import render


def documents(values=None):
    result = render(values)
    assert result.returncode == 0, result.stderr
    return [doc for doc in yaml.safe_load_all(result.stdout) if doc]


def ingress_paths(docs):
    ingress = next(doc for doc in docs if doc["kind"] == "Ingress")
    return [(p["path"], p["backend"]["service"]["name"]) for p in ingress["spec"]["rules"][0]["http"]["paths"]]


class WebsiteTests(unittest.TestCase):
    def test_default_renders_the_website_and_routes_docs_before_the_frontend(self):
        docs = documents({"ingress": {"enabled": True}})
        deployment = next(doc for doc in docs if doc["kind"] == "Deployment" and doc["metadata"]["name"].endswith("-website"))
        container = deployment["spec"]["template"]["spec"]["containers"][0]
        self.assertTrue(container["image"].startswith("ghcr.io/jrmatherly/ai-kb-website:"))
        self.assertEqual(container["readinessProbe"]["httpGet"]["path"], "/docs/")
        self.assertEqual(ingress_paths(docs), [("/api", "app"), ("/docs", "website"), ("/", "frontend")])

    def test_disabled_removes_the_workload_and_the_path(self):
        docs = documents({"ingress": {"enabled": True}, "website": {"enabled": False}})
        self.assertFalse([doc for doc in docs if doc["metadata"]["name"].endswith("-website") or doc["metadata"]["name"] == "website"])
        self.assertEqual(ingress_paths(docs), [("/api", "app"), ("/", "frontend")])

    def test_service_name_override_reaches_the_ingress(self):
        docs = documents({"ingress": {"enabled": True}, "website": {"service": {"name": "ai-kb-website"}}})
        self.assertIn(("/docs", "ai-kb-website"), ingress_paths(docs))


if __name__ == "__main__":
    unittest.main()
