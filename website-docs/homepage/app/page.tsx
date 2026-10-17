import { existsSync } from "node:fs";
import { join } from "node:path";
import { BrandLogo } from "./brand-logo";
import { Icon } from "./ui";
import { homeAssets } from "../../shared/header";
import { ProductGallery, type GalleryShot, type GallerySlide } from "./product-gallery";
import { Header, ProductVideo } from "./interactive";
import { clients, dataSources, modelProviders, IntegrationMark } from "./brands";
import s from "./home.module.css";

const repo = "https://github.com/jrmatherly/ai-kb";
const docs = "/docs/";
const upstream = "https://github.com/Tencent/WeKnora";
const guide = (path: string) => `${docs}${path}.html`;
const modes = [
  { number: "01", icon: "search", label: "RAG", title: "Answers you can check", description: "Semantic and keyword retrieval find the relevant material. Every answer cites its sources, and you can open the original passage to check it.", tags: ["Hybrid retrieval", "Multimodal parsing", "Source citations"], link: "03-features/05-retrieval-engines" },
  { number: "02", icon: "agent", label: "Agent", title: "Get tasks done with knowledge and tools", description: "Agents search knowledge bases and the web, call MCP tools and skills, process files and run scripts in a sandbox, drive the browser on your computer, and, with long-term memory on, remember the preferences you confirm across sessions.", tags: ["Multi-step reasoning", "Skills and sandbox", "Local Browser", "MCP tools", "Long-term memory"], link: "03-features/07-agent" },
  { number: "03", icon: "wiki", label: "Wiki", title: "Turn documents into a Wiki", description: "Generates interlinked Wiki pages and a graph of their links from your documents, with browsing, editing and version rollback.", tags: ["Automatic organization", "Wiki graph", "Version rollback"], link: "03-features/14-wiki" },
];

const productShotExists = (src: string) => existsSync(join(process.cwd(), "public", src));
type SlideSource = Omit<GallerySlide, "shots"> & ({ image: string; alt: string } | { shots: Omit<GalleryShot, "available">[] });
const withAvailability = (slide: SlideSource): GallerySlide => {
  const shots = "shots" in slide ? slide.shots : [{ image: slide.image, alt: slide.alt }];
  return { name: slide.name, icon: slide.icon, title: slide.title, description: slide.description, link: slide.link, external: slide.external, shots: shots.map(shot => ({ ...shot, available: productShotExists(`${homeAssets}/product/${shot.image}.png`) })) };
};
const agentSlides = [
  { name: "Local Browser", icon: "browser", shots: [
    { label: "Run a task", icon: "browser", image: "local-browser-task", alt: "Product screenshot: a Smart Reasoning conversation driving the local browser, with the task preview and pause, resume and end controls in the chat" },
    { label: "Connect the extension", icon: "plug", image: "browser-connection", alt: "Product screenshot: the Toolbox browser connection page with the BrowserSkill install buttons, the pairing link and the how-to-use steps" },
  ], title: "Drive the browser on your computer", description: "Through the open source BrowserSkill extension, the agent opens pages and fills in forms in your own Chrome or Edge, and hands login and CAPTCHA steps to you.", link: guide("05-clients/09-local-browser"), external: { label: "BrowserSkill", href: "https://github.com/Tencent/BrowserSkill", logo: "browserskill.png" } },
  { name: "MCP Server", icon: "plug", image: "mcp-server-endpoint", title: "Publish knowledge bases to other AI tools", description: "Create an MCP endpoint for a workspace. Claude, Cursor and other clients connect to it to search the knowledge base and ask questions.", alt: "Product screenshot: MCP endpoint connection details, with the endpoint URL and the mcpServers configuration for Cursor and Claude Desktop", link: guide("03-features/08-mcp") },
  { name: "Conversation control", icon: "branch", image: "chat-steer-queue", title: "Steer a conversation while it runs", description: "Add requirements while an answer is running, or fork or rewind from any earlier question. Generated files are collected on the Artifacts page.", alt: "Product screenshot: follow-up requirements queued above the input box while an answer is generated", link: guide("03-features/18-chat-experience") },
].map(withAvailability);
const wikiSlides = [
  { name: "Wiki graph", icon: "channels", image: "wiki-graph", title: "Follow the links to related knowledge", description: "See how pages relate in the Wiki graph. Click an entry to read it.", alt: "Wiki graph: the links between an everyday expense reimbursement entry and related pages, with the entry details on the right" },
  { name: "Page browser", icon: "wiki", image: "wiki-browser", title: "Organized by topic, sources kept", description: "With the Wiki on, people, products and concepts are extracted from knowledge base documents into pages with source citations, browsable by directory.", alt: "Wiki browser: a directory organized by topic, the annual leave page, linked entries and source document citations" },
  { name: "Version history", icon: "history", image: "wiki-revision-history", title: "Edit any time, trace every change", description: "Revise pages directly or let an agent help maintain them. Compare version diffs and restore earlier content when needed.", alt: "Wiki version history: the annual leave page versions, the content diff and the rollback entry" },
].map(withAvailability);
const sandboxSlides = [
  { name: "Session sandbox", icon: "sandbox", image: "skill-sandbox-chat", title: "Keep working in the same sandbox", description: "Docker, E2B and Cube are supported. Every turn of a conversation shares one workspace, and generated files can be previewed and downloaded.", alt: "Product screenshot: the agent generates a Word document from the knowledge base and opens the file preview beside the conversation", link: guide("03-features/22-skills-sandbox") },
  { name: "Desktop and terminal", icon: "monitor", shots: [
    { label: "Graphical desktop", icon: "monitor", image: "sandbox-desktop", alt: "Product screenshot: the Desktop tab of the sandbox panel beside the conversation, showing the XFCE desktop and file manager inside the sandbox" },
    { label: "Interactive terminal", icon: "terminal", image: "sandbox-terminal", alt: "Product screenshot: the sandbox terminal beside the conversation, listing the Word file generated in the workspace output directory" },
  ], title: "Open the desktop and terminal, see every step", description: "Open an interactive terminal, or a graphical desktop on Cube and E2B desktop templates, beside the conversation to watch each step the agent takes and take over when needed.", link: guide("03-features/22-skills-sandbox") },
  { name: "Skill catalog", icon: "skills", image: "skill-catalog", title: "Install, manage and reuse skills", description: "Install skills from ClawHub, SkillHub, a GitHub or GitLab repository, or a ZIP file, and manage and reuse them across the workspace.", alt: "Product screenshot: the Toolbox skill management page, listing the docx, pptx and pdf skills and the sandboxes they are installed in", link: guide("03-features/22-skills-sandbox") },
].map(withAvailability);

export default function Home() {
  return <div className={s.site}>
    <a className={s.skipLink} href="#main">Skip to content</a>
    <Header />
    <main id="main">
      <section className={`${s.shell} ${s.hero}`} aria-labelledby="hero-title">
        <div className={s.heroGrid}>
          <h1 id="hero-title">Find the answers,<br /><em>then put knowledge to work.</em></h1>
          <div className={s.heroAside}>
            <p className={s.eyebrow}>OPEN SOURCE · AI KB</p>
            <p className={s.heroDescription}>An open source knowledge management framework for teams.<br />Bring your material together for Q&amp;A, task execution and Wiki organization.</p>
            <div className={s.actions}><a className={s.primary} href="#get-started">Get started <Icon name="arrow" /></a><a className={s.secondary} href={repo} target="_blank" rel="noreferrer"><Icon name="github" /> GitHub</a></div>
            <p className={s.heroNote}>RAG Q&amp;A / Agent reasoning / Automatic Wiki</p>
          </div>
        </div>
        <ProductVideo />
        <div className={s.trustBar}><span><Icon name="github" /> Open source · MIT License</span><span><Icon name="server" /> Runs on your own infrastructure</span><span><Icon name="model" /> Your choice of models and storage</span></div>
      </section>
      <section id="capabilities" className={`${s.shell} ${s.section}`} aria-labelledby="capabilities-title">
        <div className={s.sectionHeading}><div><p className={s.eyebrow}>01 / KNOWLEDGE AT WORK</p><h2 id="capabilities-title">Knowledge Q&amp;A, task execution and an automatic Wiki</h2></div><p>RAG answers questions, agents handle multi-step tasks,<br />and the Wiki organizes knowledge. All three share the same knowledge base.</p></div>
        <div className={s.modeGrid}>{modes.map(mode => <article className={s.mode} key={mode.label}>
          <div className={s.modeTop}><Icon name={mode.icon} /><span>{mode.number} / {mode.label.toUpperCase()}</span></div>
          <h3>{mode.title}</h3><p>{mode.description}</p><ul className={s.tags}>{mode.tags.map(tag => <li key={tag}>{tag}</li>)}</ul>
          <a className={s.textLink} href={mode.label === "Wiki" ? "#wiki" : guide(mode.link)}>Learn about {mode.label} <Icon name="arrow" /></a>
        </article>)}</div>
      </section>
      <section id="agents" className={s.release} aria-labelledby="agents-title"><div className={s.shell}>
        <div className={s.sectionHeading}><div><p className={s.eyebrow}>02 / AGENTS AT WORK</p><h2 id="agents-title">Agents drive your browser.<br />Knowledge bases plug into other AI tools.</h2></div><a className={s.textLink} href={guide("05-clients/09-local-browser")}>Learn about the Local Browser <Icon name="arrow" /></a></div>
        <ProductGallery id="agents-gallery" label="Agents at work" slides={agentSlides} />
      </div></section>
      <section id="skills-sandbox" className={`${s.release} ${s.sandboxTopic}`} aria-labelledby="skills-sandbox-title"><div className={s.shell}>
        <div className={s.sectionHeading}><div><p className={s.eyebrow}>03 / SKILLS &amp; SANDBOX</p><h2 id="skills-sandbox-title">Agents run skills<br />and generate files.</h2></div><a className={s.textLink} href={guide("03-features/22-skills-sandbox")}>Learn about skills and sandboxes <Icon name="arrow" /></a></div>
        <ProductGallery id="sandbox-gallery" label="Skills and sandbox" slides={sandboxSlides} />
      </div></section>
      <section id="wiki" className={`${s.shell} ${s.section} ${s.wiki}`} aria-labelledby="wiki-title">
        <div className={s.sectionHeading}>
          <div><p className={s.eyebrow}>04 / AUTOMATIC WIKI</p><h2 id="wiki-title">Turn documents into a Wiki you can browse.</h2></div>
          <a className={s.textLink} href={guide("03-features/14-wiki")}>How to use the Wiki <Icon name="arrow" /></a>
        </div>
        <ProductGallery id="wiki-gallery" label="Wiki" slides={wikiSlides} />
      </section>
      <section id="ecosystem" className={`${s.shell} ${s.section}`} aria-labelledby="ecosystem-title">
        <div className={s.sectionHeading}><div><p className={s.eyebrow}>05 / INTEGRATIONS</p><h2 id="ecosystem-title">Data sources and tool integrations</h2></div><p>Sync material from Feishu, Confluence, GitLab and other platforms,<br />then query and use it through IM, MCP or the API.</p></div>
        <div className={s.ecosystem}>
          <div className={s.ecosystemColumn}><Icon name="sources" /><h3>Import and sync material</h3><p>Upload files, import web pages or connect external data sources.</p><div className={s.integrations}>{dataSources.map(item => <span key={item.name}><IntegrationMark item={item} /></span>)}</div><a className={s.textLink} href={guide("03-features/10-datasource")}>Data source integration <Icon name="arrow" /></a></div>
          <div className={s.ecosystemCore}><BrandLogo /><span>Team knowledge base and agents</span><div>Understand · Retrieve · Reason · Act</div></div>
          <div className={s.ecosystemColumn}><Icon name="channels" /><h3>Use it from the tools you already have</h3><p>IM Q&amp;A, MCP clients and developer tool integrations.</p><div className={s.integrations}>{clients.map(item => <span key={item.name}><IntegrationMark item={item} /></span>)}</div><a className={s.textLink} href={guide("03-features/12-im-integration")}>Clients and channels <Icon name="arrow" /></a></div>
        </div>
        <div className={s.models}><span>Your choice of models · 30 built-in providers plus Ollama <a className={s.textLink} href={guide("03-features/06-models")}>See all <Icon name="arrow" /></a></span>{modelProviders.map(item => <p key={item.name}><IntegrationMark item={item} /></p>)}</div>
      </section>
      <section id="enterprise" className={s.enterprise} aria-labelledby="enterprise-title"><div className={s.shell}>
        <div className={s.sectionHeading}><div><p className={s.eyebrow}>06 / BUILT FOR YOUR TEAM</p><h2 id="enterprise-title">Host it yourself,<br />with permissions that fit your team.</h2></div><p>Configure data storage and member permissions,<br />and review the audit trail and task status.</p></div>
        <div className={s.enterpriseGrid}><article><Icon name="server" /><h3>Deployment and storage</h3><p>Deploys with Docker Compose or Helm on Kubernetes, or as a Lite single binary; a desktop application is in development. Models, vector databases and storage backends are interchangeable, and local inference runs through Ollama.</p></article><article><Icon name="shield" /><h3>Workspace and resource permissions</h3><p>Isolated workspaces with four roles. API keys can be limited to specific capabilities and knowledge bases, users can sign in through OIDC, and outbound requests can be restricted to an allowlist of domains.</p></article><article><Icon name="trace" /><h3>Audit and runtime monitoring</h3><p>Workspace audit logs, the task queue dashboard and Langfuse tracing help teams find problems and keep the system running.</p></article></div>
      </div></section>
      <section id="get-started" className={`${s.shell} ${s.closing}`} aria-labelledby="closing-title">
        <div className={s.sectionHeading}><div><p className={s.eyebrow}>GET STARTED</p><h2 id="closing-title">Choose how to run it.</h2></div></div>
        <div className={s.startGrid}>
          <article className={s.startCard}>
            <div className={s.startLabel}><Icon name="terminal" /><span>Single binary</span></div>
            <h3>Lite edition</h3>
            <p>One binary with SQLite and no Redis, for personal, offline or low-resource environments.</p>
            <a className={s.textLink} href={`${guide("01-getting-started/02-installation")}#lite-runtime-no-external-dependencies`}>Lite setup <Icon name="arrow" /></a>
          </article>
          <article className={s.startCard}>
            <div className={s.startLabel}><Icon name="server" /><span>Kubernetes</span></div>
            <h3>Helm chart</h3>
            <p>Install the bundled chart on Kubernetes 1.25 or later, with ParadeDB and Redis included.</p>
            <a className={s.textLink} href={`${guide("01-getting-started/02-installation")}#helm-deployment-helm`}>Helm deployment <Icon name="arrow" /></a>
          </article>
          <article className={s.startCard}>
            <div className={s.startLabel}><BrandLogo /><span>Self-hosted</span></div>
            <h3>Docker Compose</h3>
            <p>The recommended standard deployment. Configure models, storage and network yourself.</p>
            <a className={s.textLink} href={guide("01-getting-started/02-installation")}>Installation guide <Icon name="arrow" /></a>
          </article>
        </div>
      </section>
    </main>
    <footer className={`${s.shell} ${s.footer}`}><a className={s.brand} href="/" aria-label="AI KB home"><BrandLogo /></a><p>Open source · MIT License · Based on <a href={upstream} target="_blank" rel="noreferrer">Tencent/WeKnora</a></p><nav aria-label="Footer navigation"><a href={docs}>Docs</a><a href={repo} target="_blank" rel="noreferrer">GitHub <Icon name="external" /></a><a href={`${repo}/blob/main/CHANGELOG.md`} target="_blank" rel="noreferrer">Changelog</a></nav></footer>
  </div>;
}
