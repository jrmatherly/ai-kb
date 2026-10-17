# AI KB website and documentation

`website-docs/` holds the product website, the documentation, the shared styles and every build and deploy script. It can be copied out and built on its own; nothing outside this directory is needed. The website and the documentation share one domain, one build and one deployable artifact.

Product, deployment, API and development documentation is maintained here. The site is published in English only. Useful content from the old `docs/` directory has been merged by topic, and outdated or duplicate text has been deleted. The [migration record](MIGRATION.md) lists what moved where and which engineering resources remain in `docs/`. It is for maintainers and is not published on the site.

- `/`: the product homepage.
- `/docs/`: redirects straight to the quick start.
- `/docs/...`: the full documentation, with category navigation, search and sidebar.

In production the documentation is path-mounted on the application host: `ai-kb.matherly.net/` is the AI KB web UI and the gateway forwards only the `/docs/` prefix to this site, so the homepage is not served there and the documentation logo links to `/docs/`. The site uses only these two paths. The site root contains only `index.html`, `404.html` and `docs/`. The homepage scripts, styles and images live under `/docs/_home/`, so an upstream gateway only needs to forward `/` (exact match) and the `/docs/` prefix. `build` rejects any page that references a resource outside these two paths.

The top bar is 64px tall and spans the full width. Both the homepage and the documentation show the logo from the repository README and share colors, fonts and the light or dark preference. The logo returns to the homepage in the current tab. The homepage and the documentation each have navigation suited to their pages.

## Publishing: static files and Nginx

### 1. Build the deployable archive

Use Node.js 24 and run the commands in the repository's `website-docs/` directory. On the first build, or after a dependency lock file changes, install dependencies first:

```bash
cd website-docs
npm run setup
npm run build
npm run package:site
```

`build` compiles the homepage, checks documentation links and Mermaid diagrams, compiles the documentation, audits the homepage content and validates resource paths in the merged output. When every step passes it replaces `static-site/`. `package:site` packs `static-site/` into an archive under `releases/`.

### 2. Upload and extract

Upload only the archive to the server. The server needs neither Node.js nor the AI KB backend nor a separate documentation service.

Extract each release into a new, empty directory so that files from an older site do not mix in. `20261013-1` below is an example release number; use a new one for each release:

```bash
sudo mkdir -p /srv/www/aikb/releases/20261013-1
sudo tar -xzf ai-kb-site-v2026.10.13.tar.gz -C /srv/www/aikb/releases/20261013-1
```

After extraction the directory contains `index.html`, `404.html` and `docs/` directly, with no extra `static-site/` level.

### 3. Point the domain root at the release

Add a site configuration based on the [deploy/nginx.conf](deploy/nginx.conf) template and change:

```nginx
server_name your-domain;
root /srv/www/aikb/releases/20261013-1;
```

If the domain already has an HTTPS configuration, keep its certificate and listen settings and merge the template's `root`, `index` and `location` rules into the existing site configuration. The domain must resolve to this server.

Keep the documentation's `.html` route resolution rule. Do not fall back to the homepage `index.html` for every unknown path. The current build is deployed at the domain root; serving it from a subdirectory such as `/aikb/` is not supported.

Test the configuration, then reload:

```bash
sudo nginx -t
sudo nginx -s reload
```

### 4. Confirm the release

Open these paths:

- `/`: the homepage.
- `/docs/`: the quick start.
- `/docs/03-features/23-memory`: the long-term memory page; opening it directly and reloading both work.
- `/docs/not-found`: returns 404.

Then check that search, the light and dark toggle and the logo link back to the homepage work.

For later updates, repeat the build, package and upload into a new directory, then change the Nginx `root` and reload. To roll back, point `root` back at the previous release directory. Remove old release directories once the new version is stable.

## Optional: Docker deployment

The image builds from a clean checkout. The host needs no Node.js and no pre-generated static files. The Dockerfile has two stages: a Node.js 24 stage installs dependencies from both lock files and builds the homepage and the documentation, and the final Nginx image holds only the static output and the server configuration.

CI publishes the image as `ghcr.io/jrmatherly/ai-kb-website` (the Website Docs workflow): `main` on every push to main, the release tag and bare version on a `v*` tag, and `latest` from the weekly rebuild of the newest release. To build it yourself, from the repository root:

```bash
docker build -t ai-kb-site:2026.10.13 website-docs
docker run -d --name ai-kb-site --restart unless-stopped -p 8080:80 ai-kb-site:2026.10.13
```

Open `http://<server-address>:8080/`. For a domain with HTTPS, have the existing reverse proxy forward to that port. The image already contains the homepage, the documentation and the Nginx routing configuration.

Nginx in the container listens on port 80 by default. The image's `/docker-entrypoint.sh` writes the port from the `WEBSITE_NGINX_PORT` environment variable into the Nginx configuration at startup, so you can change it without rebuilding. Set it when you use `--network host` or when the platform requires the container to listen on a given port:

```bash
docker run -d --name ai-kb-site --restart unless-stopped -e WEBSITE_NGINX_PORT=8080 -p 8080:8080 ai-kb-site:2026.10.13
```

A platform that replaces the entrypoint with its own startup command skips that substitution. In that case bake the port in at build time with `--build-arg WEBSITE_NGINX_PORT=8080`.

To change only the port exposed on the host, change the host side of `-p`, for example `-p 9000:80`.

You can also copy just the `website-docs/` directory and run `docker build -t ai-kb-site:2026.10.13 .` inside it. The build context must be `website-docs/`. `.dockerignore` excludes host dependencies, old build output and release archives.

The domain root `/` serves the homepage and `/docs/` serves the documentation, so the reverse proxy must cover the whole site and keep the request path.

After building, you can run the following check on a machine with Node.js 24 and Docker; it needs no npm dependencies. It starts temporary containers, cleans them up, and checks routing for both parts of the site, static assets, 404 pages, compression and response headers:

```bash
cd website-docs
npm run test:docker -- ai-kb-site:2026.10.13
```

## Local development

Use Node.js 24 LTS (the directory has an `.nvmrc`, so with nvm you can run `nvm use`). After the first checkout, install dependencies (the documentation and the homepage each install from their own lock file):

```bash
cd website-docs
npm run setup
npm run build
npm run preview
```

The preview runs at `http://127.0.0.1:3000/`. After changing sources, rebuild and reload. Use `npm run preview -- --port 8080` for another port.

```bash
npm run check
npm run test:integration
```

The integration check needs Google Chrome installed. It verifies theme synchronization, navigation on both parts of the site, search, mobile controls and direct documentation routes. Set `SITE_TEST_URL` to test a different site.

For hot reload during development, use `npm run dev:homepage` (homepage) and `npm run dev:docs` (documentation) separately. For full cross-site navigation, use `npm run preview` on the unified build.

`npm run build:docs` compiles only the documentation and `npm run build:homepage` only the homepage. Releases use `npm run build`, whose output contains both. `VERSION` holds the release version this site build uses.

## Project layout (relative to website-docs)

- `homepage/`: the Next.js homepage source, brand assets and its own dependency lock file.
- `01-getting-started/` to `06-development/`: documentation pages; their paths are unchanged.
- `.vitepress/`: documentation site configuration and theme.
- `public/`: documentation screenshots and static files.
- `sample-data/`: demo documents for trying out a knowledge base, paired with the MCP demo server in `examples/mcp-demo/server.py`.
- `shared/`: brand variables, top bar styles and shared icons.
- `scripts/`: the unified build, checks, preview and packaging.
- `static-site/`: the only deploy directory, generated by the build.
- `deploy/`, `Dockerfile`: Nginx and Docker deployment configuration.
- `releases/`: generated release archives.

Brand asset sources are listed in [homepage/BRAND-ASSETS.md](homepage/BRAND-ASSETS.md). The product video plays from a GitHub attachment URL (`videoUrl` in `homepage/app/interactive.tsx`, currently an upstream upload that differs from the repository README's video), so playback needs access to GitHub. Pages, documentation and screenshots ship in the archive.
