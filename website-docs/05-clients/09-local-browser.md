# Local Browser: connection and deployment

AI KB connects the browser on a user's computer to Smart Reasoning conversations through the BrowserSkill daemon (`bsk`) and the [BrowserSkill](https://github.com/Tencent/BrowserSkill) browser extension. The extension supports Chrome and Microsoft Edge (Chromium 125 or later). Other Chromium browsers are not guaranteed to work. The browser runs on the user's computer and the daemon runs next to the AI KB app, so no skills sandbox is needed.

## Install the extension

This section covers getting a supported extension version into the user's browser. The extension must be **0.3.1 or later**. Install it one of these ways:

- Chrome: [Chrome Web Store](https://chromewebstore.google.com/detail/hhcmgoofomhgciiibhipgmgkgnoenaoi)
- Edge: [Edge Add-ons](https://microsoftedge.microsoft.com/addons/detail/browserskill/emacgiaaaiojkkpkddmmdfhmokgmnikg)
- Matching ZIP: when the store is unreachable, download it under "Install manually (alternative)" on the Browser connection page. Unzip it, open `chrome://extensions` (`edge://extensions` in Edge), turn on Developer mode and choose Load unpacked.

When a connected extension is older than 0.3.1, the Browser connection page asks you to update it.

## Pair and use

This section pairs one browser with your account and turns it on for a conversation.

1. Open Toolbox → Browser connection in the sidebar. Old Settings → Browser connection links redirect here.
2. Copy the single-use pairing link the page generates, paste it in the extension under Connection settings → Remote connection, and save.
3. Turn on Local browser in the Smart Reasoning input bar, then submit the task. Pairing does not turn the option on for every turn.
4. The first browser call opens a separate task window. The preview in the conversation can locate the page, and pause, resume or end the current browser task.

<Screenshot
  src="/screenshots/browser-connection.png"
  caption="Toolbox → Browser connection: pairing link, connection status and extension install options"
  hint="A paired, online browser: the connection status on the tab, device information, the pairing link area, the Chrome and Edge store links with 'Install manually (alternative)', and the 'Show connection status in the sidebar' switch. A green status dot is visible next to Toolbox in the sidebar." />

The Browser connection tab shows the connection status: Connected, Offline or Not paired. Once paired, the small browser icon to the right of Toolbox in the sidebar also shows a status dot, green for connected and orange for offline. To hide it, turn off "Show connection status in the sidebar" on the Browser connection page. That setting is stored only in the current browser. The same page also has "Browser search instructions", which set your preferred search engine and search URL (up to 4000 characters, saved to your user account).

Authorization is stored per workspace and user. Conversations in the same workspace share the device authorization but each has its own task. Each workspace and user pair currently keeps one device authorization, so activating a new device replaces the old one.

A pairing link is valid for five minutes. A device token is valid for 90 days and rotates automatically after 30 days. Generating a new link does not disconnect the old device right away. The authorization is replaced only when the new link is activated. Revoking the device, deactivating the user or removing the workspace member stops further use.

### Human help and task recovery

A task can operate the tabs it created. New tabs opened inside the task window by a click or a key press, and that pass the origin check, can also be operated, and they stay open when the task ends. Separate popup windows and the user's existing tabs are borrowed with `tab_borrow`: the task operates them after the user approves according to the browser settings, and hands them back with `tab_return` when done. When a task ends, it closes the pages it explicitly created and returns the borrowed ones.

Dragging a page into the task window by hand does not authorize it. If an unauthorized page is already in the task window, the user must first move it to a normal window, then the task can borrow it. The borrow confirmation needs an HTTP(S) page in a normal window to display on. Extension settings pages, new tab pages and separate popups cannot host it. If the original window is gone after a borrow, returning the page may open a normal fallback window. Returned pages are not closed when the task ends.

Login, CAPTCHA and authorization steps start with `request_help`. Open the browser from the preview, complete the step, then confirm in the browser help prompt. The agent then looks at the page again and continues. Typing "please log in" in the chat does not create a human help prompt. Help waits at most five minutes. A timeout or a cancel keeps the pages as they are and pauses the task.

Pause cancels the current browser call but does not revoke the pairing. After you click Resume, if the agent has already finished its turn, send a message telling it to continue. After an error or a server restart, a task does not automatically replay clicks, submits or similar actions. Check the page before you continue.

A task that succeeds normally ends on its own. When the user asks to keep the pages, when human help is involved or when the task is interrupted, the pages can be kept. "Last preview retained" in the preview means polling has stopped, not that browser control has been released. Ending the browser task does not stop the whole conversation.

## Single-instance deployment

This section gets the daemon and extension package running on one app instance.

The matching app image contains the daemon, the extension ZIP and the license, with the file paths preset. The matching frontend Nginx handles the WebSocket. When you update, update the app, the frontend and the users' extensions together.

For a native deployment, build from the repository root:

```bash
./scripts/build_browserskill.sh
```

The build needs Git, Node.js, Python 3, Rust/Cargo and a C compiler, plus CMake on Linux. `scripts/browserskill-release.json` pins the source version. The script builds the upstream BrowserSkill source with no extra patches and writes the output to `artifacts/browserskill/`. Use the daemon for your operating system and architecture, and point the variables at the actual install path:

```dotenv
BROWSERSKILL_BINARY=/opt/ai-kb/browserskill/bsk
BROWSERSKILL_EXTENSION_PATH=/opt/ai-kb/browserskill/browser-skill-ai-kb-0.3.1.zip
BROWSERSKILL_MAX_CONNECTIONS=32
```

By default the pairing WSS address is derived from the origin of the user's current page, port included. Set the override only when the gateway uses a separate domain or a special path:

```dotenv
BROWSERSKILL_PUBLIC_URL=wss://kb.example.com/api/v1/local-browser/extension
```

On the same machine, plain WS on localhost works. A remote deployment needs a WSS certificate the browser trusts. On an internal network, a trusted enterprise CA works. Setting `BROWSERSKILL_BINARY=` explicitly to empty turns the feature off. After you change Docker environment variables, recreate the containers with `docker compose up -d app frontend`.

Upgrade the matching extension and daemon together. The extension must be 0.3.1 or later (Chrome Web Store, Edge Add-ons or the matching ZIP). Overwriting the original unzipped folder and reloading keeps the extension ID. If a reinstall changes the ID, pair again. Keep `BrowserSkill-LICENSE` when you redistribute.

## Multiple replicas and the ingress proxy

This section makes the Local Browser work when several app replicas sit behind one entry point.

All app replicas share one database. Device authorizations, connection leases and interruption markers are persisted in it. Each app starts the shared daemon on demand, and other replicas hand tasks over signed internal RPC to the node that holds the extension connection.

```dotenv
# Different on each replica; must reach that node directly, not a load balancer address
BROWSERSKILL_INTERNAL_URL=http://10.0.0.12:8080
# Same on all replicas; inject a random value of at least 32 characters from a Secret
BROWSERSKILL_CLUSTER_SECRET=<shared-random-secret>
```

On Kubernetes, the Pod IP can form the direct address. Do not replace the shared database with a separate SQLite file per replica. Plain internal HTTP is only for trusted, isolated networks. Use HTTPS with a trusted certificate when the traffic must be confidential.

The ingress proxy must:

- Forward the WebSocket Upgrade on `/api/v1/local-browser/extension`, and the `/authorize` POST under it.
- Block `/api/v1/local-browser/internal` from the public entry point and allow it only between app nodes. The matching frontend already rejects this path. Handle it yourself when you expose the app directly or use a custom Ingress.
- Avoid logging the Authorization header, the Sec-WebSocket-Protocol header and the authorization request body.

A connection lease lasts 45 seconds and is renewed every ten seconds. Recovery after a node failure depends on the old lease expiring and on the extension's reconnect backoff. After the service recovers, check the task status. Operations with an unknown outcome are not replayed automatically.

## Troubleshooting and acceptance

Use this table to narrow down a failure, then run the acceptance checks below before going live.

| Symptom | Check |
| --- | --- |
| No matching extension download, or the feature is unavailable | Daemon and extension file paths, execute permission, target architecture and app image version |
| Extension version too old | Update to 0.3.1 or later from the Chrome Web Store, Edge Add-ons or the matching ZIP. Overwrite the original unzipped folder, then reload |
| Authorization succeeds but WSS fails | Public address and port, trusted certificate, WebSocket Upgrade on the outer proxy |
| Device is connected but tasks do not run | The per-turn switch, whether the task is paused, whether it waits for a tab borrow or human help |
| No human help prompt | Whether the execution record calls `request_help`, and whether human help is disabled in the extension |
| Fails after switching replicas | Whether the database is shared, whether the direct node address is correct, the shared secret and the network policy |
| Preview is stuck | Whether the page is hidden, whether the task kept its last preview, and the specific CDP timeout in the extension. The preview is a low-frame-rate screenshot stream, not video |

The default of 32 is an active-device guard per node, not a throughput guarantee. Before going live, verify pairing, reconnect, revoke, human help and cross-replica routing on the target cluster, and watch daemon CPU, RSS, screenshot traffic, RPC latency and reconnect time. The real-extension test is `internal/browserskill/extension_test.go`, which uses a separate test browser profile directory.
