# Store listing

Text for the Chrome Web Store and Microsoft Edge Add-ons listings. Both stores get the same text. Every claim below matches `extension/manifest.json` and the [extension docs](https://ai-kb-docs.matherly.net/docs/05-clients/06-browser-extension).

## Title

AI KB

## Short description

Ask your AI KB about any page, and clip pages, selections and notes into a knowledge base.

(90 characters, the same text as the manifest `description`.)

## Long description

AI KB connects your browser to an AI KB server that you or your organization run. You need such a server to use this extension; the extension talks to that one server and to nothing else.

Chat. Open the side panel and ask the chat agents configured on your server about anything, or click "Ask about this page" to ask about the page you are reading. Answers stream in with numbered citations that open the cited knowledge base in the web UI.

Clip. Smart clip extracts the main article as Markdown and opens it in the editor, where you check the title and text, pick a knowledge base and save. Select clip saves the text you selected, or lets you drag a rectangle over the part of the page you want. Quick note opens an empty note with no page attached. The Recent tab lists your last captures with their parsing status.

Quick clip. Turn it on in the options page and the right-click menu items and keyboard shortcuts save straight to your last used knowledge base, without opening the editor.

Shortcuts. Alt+Shift+A asks about the page, Alt+Shift+C clips it, Alt+Shift+S clips the selection and Alt+Shift+N writes a note (Option+Shift on a Mac). Change them at chrome://extensions/shortcuts.

Sign in. Connect with an API key, with your email and password, or through your server's identity provider when the server has single sign-on enabled.

Privacy. The extension reads a page only after you click its icon, press a shortcut or use its right-click menu on that page. It requests access to one website, your server, when you click Connect. It has no analytics and no telemetry. The full privacy policy is at <https://ai-kb-docs.matherly.net/docs/05-clients/06-browser-extension#privacy>.

Server setup is documented at <https://ai-kb-docs.matherly.net>.

## Category

- Chrome Web Store: **Workflow & Planning**. Google replaced Productivity in 2023; its successors are Education, Functionality & UI, Household, Privacy & Security, Tools and Workflow & Planning, and Workflow & Planning is defined as "extensions to help users perform their tasks more efficiently", with document editors among its examples (<https://developer.chrome.com/docs/webstore/best-practices>). The Notion and Evernote web clippers are listed there.
- Microsoft Edge Add-ons: **Productivity**. Edge still uses its own list (Accessibility, Blogging, Communication, Developer tools, Entertainment, News & weather, Photos, Productivity, Search tools, Shopping, Social, Sports).

## Links

- Homepage and website: <https://ai-kb-docs.matherly.net>
- Support: <https://github.com/jrmatherly/ai-kb/issues>

## Single purpose

Ask and save pages to your own AI KB server

## Privacy policy URL

<https://ai-kb-docs.matherly.net/docs/05-clients/06-browser-extension#privacy>

## Permission justifications

| Permission | Justification |
| --- | --- |
| `storage` | Keeps the server address, credential, preferences and recent captures, and hands a clip from the background to the side panel |
| `sidePanel` | Shows the chat, capture and recent tabs in the browser side panel |
| `activeTab` | Reads the title, address and text of the current page, only after the user clicks the toolbar icon, presses a shortcut or uses the context menu on it |
| `scripting` | Runs the page extractor in that page, under the `activeTab` grant, to turn the article or selection into Markdown |
| `contextMenus` | Adds the right-click items that ask about or clip the page or selection |
| `identity` | Signs you in to your own server through its identity provider |
| Optional host permissions `https://*/*` and `http://*/*` | Reach the AI KB server you configure. Requested once, for that single origin, when the user clicks Connect on the options page |

The extension does not request the `tabs` permission, and declares no `host_permissions`, `content_scripts`, `web_accessible_resources` or `externally_connectable`.

## Privacy practices

Data the extension handles:

- Website content: the title, address, selected text and article text of pages the user asks about or clips.
- Authentication information: an API key, or an email and password, or a sign-in through the server's identity provider. The password is sent once to obtain tokens and is not stored. The key or tokens are stored in the extension's local storage.

Certifications:

- The data goes only to the AI KB server the user connected, which the user or their organization operates.
- The data is not sold to third parties.
- The data is not used or transferred for purposes unrelated to the extension's single purpose.
- The data is not used or transferred to determine creditworthiness or for lending purposes.
- The extension runs no remote code. Every script is bundled in the package.

## Store IDs

Filled in after the first upload (plan Task 18, Step 3).

- Chrome Web Store extension ID: `<chrome extension id>`
- Edge Add-ons product ID: `<edge product id>`
