# AI KB Lite compared with the standard edition

Lite is for running locally fast with the simplest possible deployment. The standard edition is for collaboration across workspaces and the full enterprise feature set. The main differences:

| Aspect | Lite | Standard edition |
| --- | --- | --- |
| **Shared spaces** | No shared spaces (no member invitations, no sharing knowledge bases or agents across members) | Shared spaces, with collaboration features such as retrieval isolated per workspace |
| **Workspaces and accounts** | One workspace; works out of the box, **no sign-up needed** | Multiple workspaces; usually needs sign-up, login and organization management |
| **Document parsing** | Only the built-in **Simple** parsing engine; other parsers can be connected, for example through **Cloud** | Several configurable parsing engines (high-accuracy ones included), integrated with the full document processing pipeline |
| **Deployment** | **One application, no dependencies** (no separate database, message queue or other external services) | Usually a multi-service deployment such as Docker Compose, with more dependencies and components |
| **Data location** | Data is stored and processed **entirely on the local machine** | Data can also stay local when you self-host; it depends on how you deploy |
| **Network exposure** | **Local access only by default**; configurable, **you choose whether to open it to the internet** | You bind addresses and gateways according to your deployment and security policy |

If you do not need multi-team collaboration, a complex parsing pipeline or a multi-service architecture, Lite suits an individual or a small team trying it out locally with no dependencies. When you need shared spaces, multiple workspaces and the full set of parsing engines, use the standard edition.
