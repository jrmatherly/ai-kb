# Technical Proposal: Support Knowledge Base POC

**Project code name**: Nova-KB-POC\
**Version**: 0.3 draft\
**Author**: Zhang Ming (R&D)\
**Reviewers**: Chen Hao (Product), Zhao Lei (QA)\
**Last updated**: 2024-01-18

## 1. Background and goals

NovaTech's support team handles about 400 installation and troubleshooting tickets a day. The current solution relies on Confluence full-text search. Frontline engineers say "search finds the paragraph, but not the sentence I can send the customer."

POC goals:

1. Import internal documents such as product manuals, meeting notes and FAQs.
2. Answer natural-language questions with an answer plus a **traceable source**.
3. Deliver an intranet demo within 4 weeks for the Q1 planning midterm review.

## 2. System architecture

```text
[Corpus]    Confluence export / Markdown / PDF manuals
      ↓
[AI KB]     docreader parsing → chunking → vector + BM25 hybrid retrieval
      ↓
[Models]    Embedding: local bge-m3
            LLM: company API gateway → DeepSeek-V3
      ↓
[Access]    iframe embed in the support ticket system (phase 2)
```

### 2.1 Key roles

| Role | Name | Responsibilities |
| --- | --- | --- |
| Technical lead | Zhang Ming | Deployment, corpus pipeline, retrieval tuning |
| Product contact | Li Wei | Requirement priorities, acceptance question set |
| QA lead | Zhao Lei | Label 100 acceptance questions, accuracy statistics |
| Approver | Chen Hao | Resource requests, scope cuts |

### 2.2 Relation to the hub product

The first POC corpus centers on **Smart Home Hub Pro** (see the *Product Manual*), plus the milestones and owners in the *Q1 Product Meeting Notes*, so it can answer "who owns" and "when does it ship" questions.

## 3. Corpus list (first batch)

| No. | Document | Format | Estimated chunks |
| --- | --- | --- | --- |
| 1 | Smart Home Hub Pro Product Manual | Markdown / PDF | 40 |
| 2 | Q1 Product Meeting Notes | Markdown | 25 |
| 3 | Employee Handbook: Expenses and Leave (support travel) | Markdown | 30 |
| 4 | Top 50 ticket FAQ | Excel import | 50 |

**About 145 knowledge units in total**, enough for the minimum March 1 intranet demo.

## 4. Retrieval strategy

- **Default**: vector + BM25 hybrid retrieval, RRF fusion, Top-5 sent to the LLM.
- **Optional**: enable rerank (`bge-reranker-v2-m3`) for better accuracy on ambiguous terms.
- **Not enabled yet**: knowledge graph (Neo4j is not in the POC environment), automatic wiki generation.

### 4.1 Chunking parameters (suggested)

| Parameter | Value | Notes |
| --- | --- | --- |
| chunk_size | 512 | Matches manual paragraph length |
| chunk_overlap | 64 | Keeps table context |
| Parent-child chunking | On | Retrieval hits child chunks, generation uses parent chunks |

## 5. Acceptance cases (examples)

| Question | Expected answer | Expected source |
| --- | --- | --- |
| How many devices can Hub Pro connect at most? | 256, ≤ 120 recommended | Product Manual: Specifications |
| When is the Q1 knowledge base POC accepted? | Live on the intranet by 2024-03-01 | Q1 Meeting Notes |
| What is the accommodation cap in tier-1 cities? | 600 CNY / night | Employee Handbook: Travel expenses |
| Which version targets Matter certification? | Firmware 3.5, staged rollout by end of March | Q1 Meeting Notes |
| Who owns the support knowledge base POC? | Zhang Ming | Meeting Notes / this proposal |

Zhao Lei will expand these into 100 formal acceptance questions.

## 6. Risks

1. **Stale corpus**: Product Manual v2.1 conflicts with the upcoming v2.2. Set up a "corpus owner" role; Li Wei checks monthly.
2. **Hallucination**: the prompt requires "answer only from the citations; if there is no basis, say you don't know."
3. **Permissions**: one workspace during the POC. Production needs knowledge base ACLs split by regional support team.

## 7. Milestones

| Date | Deliverable |
| --- | --- |
| 2024-01-20 | AI KB test environment ready, first 3 Markdown files imported |
| 2024-02-01 | FAQ Excel imported, hybrid retrieval + rerank tuned |
| 2024-02-05 | Midterm review demo |
| 2024-03-01 | 100-question acceptance report |
