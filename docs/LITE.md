# AI KB Lite compared with the standard edition

Lite is for running locally fast with the simplest possible deployment. The standard edition is for collaboration across workspaces and the full enterprise feature set. The main differences:

| Aspect | Lite | Standard edition |
| --- | --- | --- |
| **Shared spaces** | No shared spaces (no member invitations, no sharing knowledge bases or agents across members) | Shared spaces, with collaboration features such as retrieval isolated per workspace |
| **Workspaces and accounts** | One workspace; works out of the box, **no sign-up needed** | Multiple workspaces; usually needs sign-up, login and organization management |
| **Document parsing** | Only the built-in **Simple** parsing engine; other parsers can be connected, for example a local docreader (`DOCREADER_ADDR`) or, with `AIKB_WEKNORACLOUD_ENABLED=true`, the hosted AI KB Cloud (WeKnoraCloud) parser | Several configurable parsing engines (high-accuracy ones included), integrated with the full document processing pipeline |
| **Deployment** | **One application, no dependencies** (no separate database, message queue or other external services) | Usually a multi-service deployment such as Docker Compose, with more dependencies and components |
| **Data location** | Data is stored and processed **entirely on the local machine** | Data can also stay local when you self-host; it depends on how you deploy |
| **Network exposure** | **Listens on all interfaces (0.0.0.0:8080) by default**, set by `server.host` in `config/config.yaml`; bind it to 127.0.0.1 to keep it local. The desktop app binds 127.0.0.1 unless public binding is turned on | You bind addresses and gateways according to your deployment and security policy |

If you do not need multi-team collaboration, a complex parsing pipeline or a multi-service architecture, Lite suits an individual or a small team trying it out locally with no dependencies. When you need shared spaces, multiple workspaces and the full set of parsing engines, use the standard edition.
