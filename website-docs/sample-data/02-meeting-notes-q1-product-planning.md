# Product Meeting Notes: 2024 Q1 Planning

**Topic**: Smart home product line Q1 priority review\
**Time**: 2024-01-15 14:00–16:30\
**Location**: HQ Building A, 3F, Wutong meeting room\
**Note taker**: Li Wei (Product Operations)

## Attendees

| Name | Department | Role |
| --- | --- | --- |
| Chen Hao | Product | Product Director |
| Zhang Ming | R&D | Lead, Knowledge Base and AI Module |
| Wang Xue | Design | Lead, Interaction Design |
| Zhao Lei | QA | QA Manager |
| Liu Yang | Sales | Sales Director, East China |

## 1. Q1 core goals

1. **Hub Pro firmware 3.5**: pass Matter 1.2 certification and start a staged rollout by the end of March.
2. **NovaHome App 4.0**: rebuild the device list and the scene editor, internal beta by mid-February.
3. **Enterprise knowledge assistant**: an in-house RAG solution that answers installation and troubleshooting questions for the support team (Zhang Ming's team owns the POC).

## 2. Feature priorities (MoSCoW)

| Priority | Feature | Owner | Target date |
| --- | --- | --- | --- |
| Must | Matter bridge stability fixes | Hardware team | 2024-02-01 |
| Must | Drag-and-drop scene editor redesign | Wang Xue | 2024-02-15 |
| Must | Support knowledge base POC live on the intranet | Zhang Ming | 2024-03-01 |
| Should | Offline recognition with a local voice pack | Voice team | 2024-03-15 |
| Could | Weekly energy usage report | Data team | Revisit in Q2 |

## 3. Key decisions

### 3.1 Support knowledge base POC scope

- **First corpus**: product manuals, installation guides and the Top 50 ticket FAQ, about 200 documents.
- **Acceptance criteria**: on 100 random support questions, Top-3 hit rate ≥ 85%, and every answer must link its source.
- **Technology**: a self-hosted AI KB instance, a local embedding model, and the chat model through the company's unified API gateway.
- **Owner**: Zhang Ming; product contact: Li Wei.

### 3.2 App 4.0 experience principles

- The device list groups by "Room" by default, with a one-tap switch to a "By type" view.
- Scene editing moves from "forms" to "cards + timeline", so new users learn it faster.

### 3.3 Sales feedback

Liu Yang reported that East China customers care most about **working offline** and **cascading multiple hubs** (villa scenarios). Cascading goes into Q2 research. This quarter delivers only a technical feasibility report.

## 4. Risks and dependencies

| Risk | Impact | Mitigation |
| --- | --- | --- |
| Tight Matter certification schedule | Firmware release slips | Submit pre-certification materials 2 weeks early |
| Uneven support corpus quality | Answer accuracy misses the target | Zhao Lei's team samples and labels, weekly review |
| Rate limits on the unified API gateway | POC demo lags | Request a dedicated quota, separate workspace for the demo environment |

## 5. Action items

- [ ] Zhang Ming: deploy the AI KB test environment and import the first corpus by January 20 (Chen Hao approves resources)
- [ ] Wang Xue: deliver a high-fidelity scene editor prototype by January 25
- [ ] Li Wei: email POC progress to attendees every Friday
- [ ] Zhao Lei: submit the cleaned Top 50 FAQ Excel by February 1

## 6. Next meeting

- **Time**: 2024-02-05 14:00\
- **Agenda**: App 4.0 beta feedback, knowledge base POC midterm review
