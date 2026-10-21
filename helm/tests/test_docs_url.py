"""Render-only checks for the AIKB_DOCS_URL the app container receives."""

import unittest

from test_website import documents


def docs_url(values=None):
    app = next(doc for doc in documents(values) if doc["kind"] == "Deployment" and doc["metadata"]["name"].endswith("-app"))
    env = app["spec"]["template"]["spec"]["containers"][0]["env"]
    return next((item["value"] for item in env if item["name"] == "AIKB_DOCS_URL"), None)


class DocsURLTests(unittest.TestCase):
    def test_in_chart_website_behind_the_ingress(self):
        self.assertEqual(docs_url({"ingress": {"enabled": True}}), "http://ai-kb.example.com/docs/")
        tls = {"ingress": {"enabled": True, "host": "kb.corp", "tls": {"enabled": True}}}
        self.assertEqual(docs_url(tls), "https://kb.corp/docs/")

    def test_unset_without_the_website_or_the_ingress(self):
        self.assertIsNone(docs_url())
        self.assertIsNone(docs_url({"ingress": {"enabled": True}, "website": {"enabled": False}}))

    def test_explicit_value_wins(self):
        values = {"ingress": {"enabled": True}, "app": {"docsUrl": "https://docs.corp/kb/"}}
        self.assertEqual(docs_url(values), "https://docs.corp/kb/")


if __name__ == "__main__":
    unittest.main()
