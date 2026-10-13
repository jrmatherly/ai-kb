# Chrome extension (Knowledge Assistant)

The Knowledge Assistant Chrome extension lets you ask a knowledge base questions from a side panel on any web page, clip the current page, and save Markdown notes.

Install the extension from the [Chrome Web Store](https://chromewebstore.google.com/detail/jpemjbopikggjlmikmclgbmkhhopjdgd). It connects to an AI KB service you already run, and everything it imports is stored in that instance.

<Screenshot
  src="/screenshots/chrome-extension.png"
  caption="Chrome extension: side panel Q&A and web clipping"
  hint="The extension side panel open on a web page (the Q&A panel and the knowledge base picker), plus the region selection used when clipping." />

## Features

This section lists what you can do from the browser once the extension is connected.

| Capability | Description |
| --- | --- |
| Knowledge base Q&A | A chat panel in the side panel. Switch between knowledge bases and pick one of three answer modes (fast, deep or precise). Ask while you browse without leaving the page |
| Web clipping | Save the page URL, let AI extract the main content, or select a region by hand, and write it to the knowledge base you choose |
| Markdown notes | Write in the built-in editor and save the note to a knowledge base |
| Keyboard shortcuts | Asking a question, opening the side panel and other actions all take custom shortcuts |

## Install and connect

This section connects the extension to your instance with an API key.

Settings → Publish & Integrations → Chrome Extension shows the setup steps and the API address of the current instance, ready to copy. Connect in this order:

1. **Get API credentials.** Copy the API address from Settings → Publish & Integrations → API Integration, and create an API key there. Copy the key from the dialog that opens: the full key is shown only once. The Open API Info button on the setup page goes straight there. Create a dedicated key and narrow its capabilities to what the extension needs, at least retrieve and ingest (see [Configure an API key for a program](../03-features/01-tenant-auth.md#configure-an-api-key-for-a-program)).
2. **Desktop app: fix the port first.** With the AI KB desktop app, set a fixed port on the API Integration page (for example 37841). Otherwise the port changes on every start and the extension cannot connect.
3. **Install the extension** from the Chrome Web Store.
4. **Connect in the extension.** Open the extension settings, choose the enterprise/developer mode, and enter the API address and API key.

After connecting, list the knowledge bases and ask one question to confirm that the credentials and the network work.

## Troubleshooting

Use this table when the extension cannot connect, is refused, or a clip does not appear.

| Symptom | Check |
| --- | --- |
| The extension says it cannot connect | Whether the API address is reachable from the machine the browser runs on (a container-internal address or `localhost` does not work for a remote deployment), and whether the desktop app has a fixed port |
| 401 / 403 | Whether the API key was revoked, whether its capabilities cover retrieve and ingest, and, if the key is limited to certain knowledge bases, whether the target knowledge base is on its list |
| A clip does not show up in the knowledge base | Check the parsing status in the knowledge base document list. `processing` means parsing is still running. The failure reason is in the document details |

## Related pages

These pages cover the credentials, the import pipeline and the other ways to connect.

- Credentials and capabilities: [Workspaces, users and authentication](../03-features/01-tenant-auth.md)
- How clipped content is imported: [Document ingestion pipeline](../02-architecture/03-document-pipeline.md)
- Other integrations: [Claw Skill](07-claw-skill.md), [CLI](02-cli.md), [Go SDK](03-go-sdk.md)
