# Test instructions for store reviewers

Paste the text under "For the reviewer" into the review notes of both stores. Replace `<demo server URL>` and `<demo API key>` with the live values first. The key must be an API key with the `chat`, `retrieve` and `ingest` capabilities, limited to one demo knowledge base, so the reviewer can read and write that knowledge base and nothing else. Rotate it after the review.

## For the reviewer

The extension needs an AI KB server. A demo server is provided for this review.

Server URL: `<demo server URL>`
API key: `<demo API key>`

The key can read and write one demo knowledge base only.

1. Open the options page. Go to chrome://extensions (edge://extensions in Edge), open Details for AI KB and click Extension options. The page opens in a tab.
2. Connect. Enter the server URL above, keep the "API key" tab selected, paste the key and click Connect. The browser asks for access to the demo server. Allow it. A Connected card appears with "Can read knowledge bases" and "Can chat".
3. Clip a page and ask a question. Open any article, for example a Wikipedia page, and click the AI KB toolbar icon, then "Smart clip". The side panel opens on the Capture tab with the extracted article. Pick the demo knowledge base if it is not already selected and click Save. Switch to the Chat tab, type a question about the article and press Enter. The answer streams in. The Recent tab lists the saved clip.
