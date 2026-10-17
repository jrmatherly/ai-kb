# Browser extension

The AI KB browser extension for Chrome and Edge asks your AI KB server about the page you are reading and saves pages, selections and notes into a knowledge base. It talks only to the server you connect it to. It is a different extension from the one [Local Browser](09-local-browser.md) uses to let agents drive your browser.

## Install

This section installs the extension from a release build or from source.

- **Release build.** Builds for Chrome and Edge are attached to the [extension releases](https://github.com/jrmatherly/ai-kb/releases?q=extension-v), tagged `extension-v<version>`, once published. Until then, use the from-source steps below. Settings → Publish & Integrations → Browser Extension in the web UI links to the releases too.
- **From source, for development.** Build the extension with Node 24 or later:

  ```bash
  cd extension
  npm ci
  npm run build
  ```

  Open `chrome://extensions` (`edge://extensions` in Edge), turn on Developer mode, choose Load unpacked and pick `extension/dist`. An unpacked build gets its own extension ID, derived from the folder path, so its sign-in return URL differs from a store build's (see [Sign in with your identity provider](#sign-in-with-your-identity-provider)).

## Connect

This section connects the extension to your server.

Open the options page from `chrome://extensions` (Details, then Extension options). It opens in a tab.

1. Enter the **Server URL**, the address of your AI KB site, such as `https://kb.example.com`. The extension removes a trailing slash or `/api/v1` and adds `https://` to an address without a scheme. It accepts an `http://` address with the warning "Your key, password or sign-in will travel unencrypted".
2. Pick a sign-in tab and fill it in. The three tabs are described in the next sections.
3. Click **Connect**. The browser asks once for access to that one server ("Read and change your data on `<host>`"). This is the only site permission the extension requests, and it survives a browser restart. Declining shows "Allow access to the server to connect".

When the server reports several workspaces for your account, a **Workspace** list appears. Choose one and click Connect again. After a successful connect the **Connected** card shows the workspace and your email, the **Last used knowledge base** picker and the **Quick clip** switch. **Sign out** keeps the server address. **Forget server** clears it as well.

### API key

Create a key under Settings → Publish & Integrations → API Integration (see [Configure an API key for a program](../03-features/01-tenant-auth.md#configure-an-api-key-for-a-program)) with these [capabilities](../03-features/01-tenant-auth.md#capabilities):

| Capability | What the extension uses it for |
| --- | --- |
| `chat` | Agents and conversations in the side panel |
| `retrieve` | Listing and reading knowledge bases |
| `ingest` | Saving clips and notes |

A key limited to some knowledge bases sees and writes only those. After connecting with a key, the Connected card checks the key and shows "Can read knowledge bases" or "Cannot read knowledge bases", and "Can chat" or "Cannot chat: create a key with the chat capability".

Conversations started under a key belong to the key, not to a person, so everyone who uses the same key sees the same conversation history. Give each person their own key, or have them sign in.

A platform key has no workspace of its own. The server answers 409 until you enter a **Workspace ID**, which the extension then sends with every request.

### Email and password

This tab signs you in with your AI KB account. The password is sent to the server once to obtain tokens and is not stored. The extension keeps the access and refresh tokens and refreshes them when they expire.

### Sign in with your identity provider

The sign-in tab appears only when the server has [OIDC single sign-on](../03-features/01-tenant-auth.md#oidc-single-sign-on) enabled. Its label is "Sign in with" plus the provider name the server reports (`OIDC_AUTH_PROVIDER_DISPLAY_NAME`, or "SSO" when that is unset). It opens the identity provider in a browser window and returns to the extension when you finish.

The server redirects the window only to a return URL that exactly matches one of its configured entries. The tab shows this browser's **Return URL** with a Copy button. It has the form `https://<extension-id>.chromiumapp.org/oidc`. Each store assigns its own extension ID, so a Chrome build and an Edge build have different return URLs.

The list is empty by default, and an empty list turns extension sign-in off. The [configuration reference](../01-getting-started/04-configuration.md) and the [`/auth/oidc/start` reference](../04-api/02-api-auth.md#get-api-v1-auth-oidc-start) describe the parameter.

An operator enables extension sign-in once per server:

1. Find each extension ID. In Chrome, read it at `chrome://extensions` with Developer mode on, or take the last part of the Chrome Web Store listing URL. In Edge, read it from the product page in Partner Center, or at `edge://extensions` with Developer mode on. The Return URL field on the options page shows the same ID.
2. List every return URL, comma-separated, in `OIDC_AUTH_EXTENSION_RETURN_URLS` (or `oidc_auth.extension_return_urls` in `config.yaml`):

   ```bash
   OIDC_AUTH_EXTENSION_RETURN_URLS=https://<chrome-id>.chromiumapp.org/oidc,https://<edge-id>.chromiumapp.org/oidc
   ```

   On a development server, also add the return URL of your unpacked build. Do not list development IDs on a production server: any side-loaded extension can claim an unpacked ID and receive tokens for a signed-in user.
3. Restart the server.
4. With several replicas, give every replica the same `JWT_SECRET`. It signs the sign-in state, so a sign-in started on one replica and finished on another fails when the secrets differ.

### Plain HTTP and local network servers

A server on `http://` works, but the key, password or tokens cross the network unencrypted. That is why the options page warns. Use it only on a network you trust.

A server on a private address, such as `http://192.168.1.81:5180`, needs nothing extra. Connect requests the same single-server permission as for any other server. In our tests on Chrome for Testing 156 and Edge 155, requests from the extension to a LAN server succeeded with no Local Network Access prompt, with or without that permission.

## Features

This section lists what the extension does once it is connected.

The side panel has three tabs, **Chat**, **Capture** and **Recent**, and follows the active tab of its window.

- **Chat.** The Chat tab talks to the agents your account can use. Pick an agent, and knowledge bases when the agent lets you choose. Answers stream in, with numbered citations that open the cited knowledge base in the web UI. **Stop** ends a running answer, **History** lists earlier conversations, and an answer that waits for an approval links to the conversation in the web UI. On a page the extension can read, the **Ask about this page** chip starts a question with the page title and address.
- **Smart clip.** Smart clip extracts the main article of the page as Markdown and opens it in the editor, where you check the title and text, pick a knowledge base and save. The saved entry shows "Browser Extension" as its source in the web UI, and the text starts with a `Source:` line holding the page address (plus `Author:` and `Published:` when the page states them).
- **Select clip.** Select clip saves the text you selected. With nothing selected, drag a rectangle over the part of the page you want. Escape cancels.
- **Quick note.** Quick note opens an empty note titled `Note <date>` with no page attached.
- **Quick clip.** Turn on the **Quick clip** switch on the options page. When a last used knowledge base is set, the context menu items and the clip shortcuts then save straight to it without opening the editor. Without a last used knowledge base they open the editor. The toolbar popup and the side panel always open the editor. A quick clip always creates a new entry, and the side panel shows the result with **Edit** and **Done**. Each save updates the last used knowledge base.
- **Recent.** The Recent tab lists your recent captures (up to 50) with their parsing status. Clipping a page you saved before offers to update the existing entry or save a new one.

### Shortcuts and menus

| Command | Default shortcut | Context menu item |
| --- | --- | --- |
| Open the AI KB side panel | none | |
| Ask about this page | Alt+Shift+A | Ask AI KB about this (on a selection) |
| Clip this page | Alt+Shift+C | Clip page to AI KB |
| Clip the selection | Alt+Shift+S | Clip selection to AI KB (on a selection) |
| Write a note | Alt+Shift+N | Quick note |

Change or add shortcuts at `chrome://extensions/shortcuts` (`edge://extensions/shortcuts` in Edge). The options page lists the current bindings. The browser leaves a suggested shortcut unassigned when another extension already holds it. The options page then shows "not set" for it.

## Permissions

This section explains each permission the extension asks for.

| Permission | Why |
| --- | --- |
| `storage` | Keeps the connection, preferences and recent captures, and hands a clip from the background to the side panel |
| `sidePanel` | Shows the side panel |
| `activeTab` | Reads the current page, only after you click the toolbar icon, press a shortcut or use the context menu on it |
| `scripting` | Runs the page extractor in that page, under the `activeTab` grant |
| `contextMenus` | Adds the right-click items |
| `identity` | Opens your identity provider's sign-in window and receives the result |
| Host access to your server | Requested once, for the one server you connect to, from the Connect button |

The extension does not request the `tabs` permission or access to any site other than your server. It learns a page's address and title only from the `activeTab` grant, which belongs to one tab and ends when that tab moves to another site. After you switch to another tab, the side panel shows "Click the AI KB icon, press `<shortcut>`, or use the right-click menu to clip this page" instead of the page chip. Each of those three actions grants access to that tab.

## Known limitations

This section lists what the extension and the server do not support.

- **200,000 characters.** The server accepts at most 200,000 characters per entry, counting the `Source:` lines. The editor counts characters as you type. Above the limit, **Trim to the limit** replaces Save and cuts at a line break near the limit.
- **Too short.** The extension does not save an empty note or an article clip under 50 characters.
- **HTML the server removes.** The server strips `<script>`, `<iframe>`, `<object>`, `<embed>`, `<form>`, `<input>` and `<button>` markup, `javascript:` and `vbscript:` links and inline event handlers such as `onclick=` from the saved text.
- **Titles.** The server rejects a title containing that markup or control characters. The editor spots such a title before saving and offers **Use cleaned title**.
- **Pages that cannot be clipped.** The extension cannot clip browser pages (`chrome://`, `edge://`, `about:`), other extensions' pages, local files, `view-source:` pages, PDFs or the Chrome Web Store.
- **Images in answers.** Images stored on your server load through its `/files` route. That route refuses a key limited to some knowledge bases, so under such a key the images show "image unavailable". Images from other sites load only when you click "Load image from `<host>`".

## Troubleshooting

This section maps the messages you may see to their causes.

- **Cannot connect.** "Enter a valid server URL" means the address is not an `http://` or `https://` URL, or has a query, fragment or user name in it. A browser network error such as "Failed to fetch" means the server is unreachable from this browser. Check the address, the certificate and any proxy. "Allow access to the server to connect" means the permission prompt was declined. Click Connect again.
- **401 on connect.** "The server rejected this key or sign-in" means the key was revoked or mistyped, or the account cannot sign in. On a key, "Cannot read knowledge bases" or "Cannot chat" means it lacks the `retrieve` or `chat` capability.
- **"This key cannot write to this knowledge base".** A save got 403 because the key lacks `ingest` or because the knowledge base is outside the key's allowed list. Pick another knowledge base or use another key.
- **"This page cannot be clipped".** The page is one of those listed under Known limitations.
- **No access to this tab yet.** "Click the AI KB icon, press `<shortcut>`, or use the right-click menu to clip this page" means the extension has no access to this tab yet (see Permissions). Any of those three actions gives it access. When the shortcut was never assigned, the hint says "the shortcut".
- **"This server does not allow extension sign-in yet".** The server answered 400 to the extension's return URL. An operator must add the exact value of the options page's **Return URL** field to `OIDC_AUTH_EXTENSION_RETURN_URLS` and restart the server ([steps](#sign-in-with-your-identity-provider)). The message links to those steps. The extension asks the server's start route before it opens a sign-in window, so a server that cannot be reached shows "Cannot reach the server" instead, with the browser's error.
- **"Signed out elsewhere. Sign in again."** The server refused the refresh token, for example after the session was revoked or the refresh token expired. Sign in again on the options page. A save that gets the same refusal shows "Signed out elsewhere".
- **Parsing failed.** A saved entry whose processing fails shows "Parsing failed:" with the error message the server recorded for the entry. The entry is saved. The web UI shows it with the same status. "Parsing is taking longer than two minutes" only means the extension stopped waiting. Recent picks up the final status later.

## Privacy

This section describes what the extension stores and sends. It is the privacy policy for the store listings.

**Stored in your browser.** The extension keeps these in its local storage, in plain text in your browser profile: the server address, your API key or the access and refresh tokens from a sign-in, the workspace ID, the last used knowledge base, the Quick clip setting and the list of your recent captures (title, page address, knowledge base, time and parsing status). Web pages cannot read them, and neither can the extractor the extension runs inside a page. The extension never stores your password. A clip or question waiting for the side panel is kept in session storage and is gone when the browser closes. Sign out or Forget server on the options page removes the credential. Removing the extension removes everything.

**Sent to your server.** The extension sends your credential with every request, the questions you ask, and, when you clip or ask about a page, that page's title, address and the text you save. They go only to the AI KB server you connected to, which you or your organization operate.

**Sent nowhere else.** The extension has no analytics and no telemetry, and sends nothing to its developers or any other party. Images from other sites inside an answer are fetched from those sites only when you click to load them.

The extension handles two kinds of data: website content (page text, selections, titles and addresses, sent only to your server) and authentication information (an API key, or an email and password or a sign-in through your server's identity provider, exchanged for tokens stored locally). It does not sell this data or use it for anything other than asking and saving to your server.
