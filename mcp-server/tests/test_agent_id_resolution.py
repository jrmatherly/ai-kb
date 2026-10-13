from aikb_mcp_server import AIKBClient


def test_resolve_agent_id_accepts_non_uuid_agent_id(monkeypatch):
    client = AIKBClient("http://example.test/api/v1", "")
    agents = {
        "data": [
            {
                "id": "builtin-wiki-researcher",
                "name": "维基问答",
            }
        ]
    }
    monkeypatch.setattr(client, "_request", lambda *args, **kwargs: agents)

    assert (
        client.resolve_agent_id("builtin-wiki-researcher")
        == "builtin-wiki-researcher"
    )


def test_resolve_agent_id_accepts_agent_name_case_insensitively(monkeypatch):
    client = AIKBClient("http://example.test/api/v1", "")
    agents = {
        "data": [
            {
                "id": "builtin-wiki-researcher",
                "name": "Wiki Questioner",
            }
        ]
    }
    monkeypatch.setattr(client, "_request", lambda *args, **kwargs: agents)

    assert client.resolve_agent_id("wiki questioner") == "builtin-wiki-researcher"
