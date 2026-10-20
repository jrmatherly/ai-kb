# Test instructions for store reviewers

Each store gets the text under "For the reviewer" with `<demo server URL>` and `<demo API key>` replaced by the live values. Edge: the `EDGE_CERT_NOTES` secret in the `extension-stores` environment holds it and the publish job sends it as the certification notes (`gh secret set EDGE_CERT_NOTES --env extension-stores < notes.txt`); a manual Partner Center submission pastes it into Notes for certification. Chrome: the dashboard's Test instructions page keeps it across versions; leave Username and Password empty and put the text in Additional instructions, which allows 500 characters, so use the short form under "Chrome, 500 characters" below and paste it without the backticks around the two values. With the live server URL, the current 46-character key and no backticks it is 497 characters (measured 2026-10-20); with the backticks it is 501 and does not fit. Replace `<demo server URL>` and `<demo API key>` with the live values first. The key must be an API key with the `chat`, `retrieve` and `ingest` capabilities, limited to one demo knowledge base, so the reviewer can read and write that knowledge base and nothing else. Rotate it after the review.

## For the reviewer

The extension needs an AI KB server. A demo server is provided for this review.

Server URL: `<demo server URL>`
API key: `<demo API key>`

The key can read and write one demo knowledge base only.

1. Open the options page. Go to chrome://extensions (edge://extensions in Edge), open Details for AI KB and click Extension options. The page opens in a tab.
2. Connect. Enter the server URL above, keep the "API key" tab selected, paste the key and click Connect. The browser asks for access to the demo server. Allow it. A Connected card appears with "Can read knowledge bases" and "Can chat".
3. Clip a page and ask a question. Open any article, for example a Wikipedia page, and click the AI KB toolbar icon, then "Smart clip". The side panel opens on the Capture tab with the extracted article. Pick the demo knowledge base if it is not already selected and click Save. Switch to the Chat tab, type a question about the article and press Enter. The answer streams in. The Recent tab lists the saved clip.

## Chrome, 500 characters

Needs an AI KB server; demo provided. Server: `<demo server URL>`. API key: `<demo API key>` (one demo knowledge base).

1. chrome://extensions > AI KB > Details > Extension options. Enter the server URL, keep the "API key" tab, paste the key, Connect, allow access. A Connected card appears.
2. Open any article (e.g. Wikipedia), click the AI KB icon > Smart clip. Pick the demo knowledge base, Save. On the Chat tab ask about the article; the answer streams.
