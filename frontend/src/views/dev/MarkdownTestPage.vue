<template>
  <div class="markdown-test-page">
    <h1 class="page-title">Markdown Rendering Test</h1>
    <p class="page-desc">
      Dev-only page for visual regression testing of chat answer markdown
      (same typography as botmsg / AgentStreamDisplay / embed).
      Add new test cases or paste arbitrary markdown in the editor below.
    </p>

    <!-- Basic Text Styles (GPT markdown test doc alignment) -->
    <section class="test-section">
      <h2>Basic Text Styles</h2>
      <div class="test-case">
        <div class="test-rendered markdown-content" v-html="basicTextHtml"></div>
      </div>
    </section>

    <!-- LaTeX Formulas -->
    <section class="test-section">
      <h2>LaTeX Formulas</h2>
      <div v-for="(tc, i) in latexCases" :key="'latex-' + i" class="test-case">
        <div class="test-raw"><code>{{ tc.raw }}</code></div>
        <div class="test-rendered markdown-content" v-html="tc.html"></div>
      </div>
    </section>

    <!-- Code Blocks -->
    <section class="test-section">
      <h2>Code Blocks</h2>
      <div class="test-case">
        <div class="test-rendered markdown-content" v-html="codeBlockHtml"></div>
      </div>
    </section>

    <!-- Tables -->
    <section class="test-section">
      <h2>Tables</h2>
      <div class="test-case">
        <div class="test-rendered markdown-content" v-html="tableHtml"></div>
      </div>
    </section>

    <!-- Lists & Blockquotes -->
    <section class="test-section">
      <h2>Lists &amp; Blockquotes</h2>
      <div class="test-case">
        <div class="test-rendered markdown-content" v-html="listsHtml"></div>
      </div>
    </section>

    <!-- Mixed Content (LaTeX + code + text) -->
    <section class="test-section">
      <h2>Mixed Content</h2>
      <div class="test-case">
        <div class="test-rendered markdown-content" v-html="mixedHtml"></div>
      </div>
    </section>

    <!-- Mermaid -->
    <section class="test-section">
      <h2>Mermaid Diagram</h2>
      <div class="test-case">
        <div ref="mermaidContainer" class="test-rendered markdown-content" v-html="mermaidHtml"></div>
      </div>
    </section>

    <!-- Streaming Simulation -->
    <section class="test-section">
      <h2>Streaming Simulation</h2>
      <p class="test-hint">Simulates character-by-character streaming, like during a chat response.</p>
      <div class="stream-controls">
        <button @click="startStream" :disabled="isStreaming" class="btn">Start</button>
        <button @click="resetStream" class="btn">Reset</button>
        <label class="speed-label">
          Speed:
          <input type="range" min="10" max="200" v-model.number="streamSpeed" />
          {{ streamSpeed }}ms
        </label>
      </div>
      <div class="test-case">
        <div ref="streamContainer" class="test-rendered markdown-content" v-html="streamHtml"></div>
      </div>
    </section>

    <!-- Streaming Shimmer (in-progress step titles) -->
    <section class="test-section">
      <h2>Streaming Shimmer</h2>
      <p class="test-hint">
        The "light sweep" applied to in-progress step titles in
        AgentStreamDisplay / RagPipelineProgress. Running steps shimmer; finished ones are static.
      </p>
      <div class="test-case shimmer-demo">
        <div class="action-card action-pending">
          <div class="action-title">
            <span class="action-name">Searching the knowledge base…</span>
          </div>
        </div>
        <div class="action-card action-pending">
          <div class="action-title">
            <span class="action-name">Generating the answer…</span>
          </div>
        </div>
        <div class="action-card">
          <div class="action-title">
            <span class="action-name is-done">Search complete (static reference)</span>
          </div>
        </div>
      </div>
    </section>

    <!-- Custom Editor -->
    <section class="test-section">
      <h2>Custom Input</h2>
      <p class="test-hint">Paste any markdown here to test rendering.</p>
      <textarea v-model="customInput" class="custom-textarea" rows="8"
        placeholder="Type or paste markdown here..."></textarea>
      <div v-if="customInput.trim()" class="test-case">
        <div ref="customContainer" class="test-rendered markdown-content" v-html="customHtml"></div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick, watch } from 'vue';
import 'katex/dist/katex.min.css';
import { sanitizeMarkdownHTML } from '@/utils/security';
import {
  createChatMarkdownRenderer,
  renderChatMarkdown,
} from '@/utils/chatMarkdownRenderer';
import {
  appendMermaidSvgCache,
  ensureMermaidInitialized,
  enhanceMarkdownContainer,
  createMermaidCodeRenderer,
} from '@/utils/mermaidShared';
import {
  replaceIncompleteMermaidWithPlaceholder,
  prepareStreamingMermaidMarkdown,
  extractMermaidCodes,
  injectCachedMermaidSvg,
} from '@/utils/chatMessageShared';

ensureMermaidInitialized();

const mermaidRenderer = createChatMarkdownRenderer({
  codeRenderer: createMermaidCodeRenderer('mermaid-test'),
});

const mermaidContainer = ref<HTMLElement | null>(null);
const streamContainer = ref<HTMLElement | null>(null);
const customContainer = ref<HTMLElement | null>(null);

const render = (raw: string): string => {
  if (!raw) return '';
  return renderChatMarkdown(raw, {
    renderer: mermaidRenderer,
    escapeMarkdown: (markdown) => markdown,
    sanitizeHtml: sanitizeMarkdownHTML,
    prepareMarkdown: (markdown) => replaceIncompleteMermaidWithPlaceholder(markdown),
  });
};

const renderStreamMarkdown = (raw: string): string => {
  if (!raw) return '';
  return renderChatMarkdown(raw, {
    renderer: mermaidRenderer,
    escapeMarkdown: (markdown) => markdown,
    sanitizeHtml: sanitizeMarkdownHTML,
    // Match production: the live chat marks unfinished answers as streaming so
    // mid-stream guards (dangling emphasis, trailing rules) are exercised here.
    streaming: isStreaming.value,
    cachedMermaidSvgHtml: streamMermaidSvgHtml.value,
    prepareMarkdown: prepareStreamingMermaidMarkdown,
    injectCachedMermaidSvg,
  });
};

// --- Test Data ---

const basicTextSample = `This is plain text with **bold**, *italic*, ***bold italic***, ~~strikethrough~~ and inline \`code\`.

It can also include keyboard shortcut styles: <kbd>⌘</kbd> + <kbd>K</kbd>.

Here is a link: [GitHub](https://github.com).

> First-level quote
>
> > Nested quote, for comparing GPT's quote levels and font weight.

- [ ] Open task
- [x] Completed task
`;

const latexCases = [
  { raw: 'Inline math: $E = mc^2$ in the middle of text.' },
  { raw: 'Block math:\n$$\\int_0^\\infty e^{-x}\\,dx = 1$$' },
  { raw: 'Chemical formula: $\\mathrm{Mg}^{2+} + 2\\mathrm{OH}^{-} = \\mathrm{Mg(OH)}_{2}\\downarrow$' },
  { raw: 'Chemical block:\n$$\\mathrm{Cu}^{2+} + 2\\mathrm{OH}^{-} \\rightarrow \\mathrm{Cu(OH)_2}\\downarrow$$' },
  { raw: 'Summation: $\\sum_{i=1}^{n} i = \\frac{n(n+1)}{2}$' },
  { raw: 'Matrix:\n$$\\begin{pmatrix} a & b \\\\ c & d \\end{pmatrix}$$' },
  { raw: 'Escaped delimiters: \\(\\alpha + \\beta = \\gamma\\) and \\[\\int_a^b f(x)\\,dx\\]' },
].map((tc) => ({ ...tc, html: '' }));

const codeBlockSample = `Here is some Python:

\`\`\`python
def fibonacci(n: int) -> int:
    """Calculate the nth Fibonacci number."""
    if n <= 1:
        return n
    return fibonacci(n - 1) + fibonacci(n - 2)

print(fibonacci(10))  # 55
\`\`\`

And inline code: \`const x = 42;\`
`;

const tableSample = `| Element | Symbol | Atomic Number |
|---------|--------|:-------------:|
| Hydrogen | H | 1 |
| Helium | He | 2 |
| Lithium | Li | 3 |
| Carbon | C | 6 |
`;

const listsSample = `### Ordered List
1. First item
2. Second item
   1. Nested item A
   2. Nested item B
3. Third item

### Unordered List
- Alpha
- Beta
  - Sub-item
  - Another sub-item
- Gamma

### Blockquote
> This is a blockquote with **bold** and *italic* text.
>
> It can span multiple paragraphs.
`;

const mixedSample = `## Quadratic Formula

The solutions to $ax^2 + bx + c = 0$ are given by:

$$x = \\frac{-b \\pm \\sqrt{b^2 - 4ac}}{2a}$$

### Example in Python

\`\`\`python
import math

def solve_quadratic(a, b, c):
    discriminant = b**2 - 4*a*c
    if discriminant < 0:
        return None
    x1 = (-b + math.sqrt(discriminant)) / (2*a)
    x2 = (-b - math.sqrt(discriminant)) / (2*a)
    return x1, x2
\`\`\`

| a | b | c | Solutions |
|---|---|---|-----------|
| 1 | -3 | 2 | $x = 1, 2$ |
| 1 | 0 | -4 | $x = \\pm 2$ |
| 1 | 2 | 5 | No real solutions |

> **Note:** The discriminant $\\Delta = b^2 - 4ac$ determines the nature of the roots.
`;

const mermaidSample = `\`\`\`mermaid
graph TD
    A[Start] --> B{Decision}
    B -->|Yes| C[Process A]
    B -->|No| D[Process B]
    C --> E[End]
    D --> E
\`\`\`
`;

// --- Streaming Simulation ---
const fullStreamText = `
Sure. Here is an introduction to XXX, compiled from the **"xxx" course handbook** in the knowledge base:

**XBRL (eXtensible Business Reporting Language)** is an XML-based standardized markup language for preparing, exchanging and analyzing electronic business and financial reports

The dataset contains 90 text and equation pairs that challenge a model's ability to extract, interpret and reason about interrelated financial terms and formulas, for example:
\`\`\`
APR = ((Fees + Interest) / Principal) × (365 / Days in Loan Term)
\`\`\`
<kb doc="2502.08127v1.pdf" chunk_id="1ecdce8a-f922-4d0c-b124-257ab4634da2" />

### Importance


1. **AAAAA**
   - **BBBBB**
   - **CCCCC**

1. **AAA**
2. **BBB**
3. **CCC**
4. **DDD**
5. **EEE**
6. **FFF**
7. **GGG**
8. **HHH**
9. **III**

**Title: JJJ**

**Title: KKK**


The energy-mass equivalence is $E = mc^2$.

In chemistry, the neutralization reaction:

$$\\mathrm{Mg}^{2+} + 2\\mathrm{OH}^{-} = \\mathrm{Mg(OH)}_2\\downarrow$$

Here is a code example:

\`\`\`python
def greet(name):
    print(f"Hello, {name}!")
\`\`\`

And the derivative rule: $\\frac{d}{dx}\\sin x = \\cos x$.

\`\`\`mermaid
graph TD
    A[Start] --> B{Decision}
    B -->|Yes| C[Process A]
    B -->|No| D[Process B]
    C --> E[End]
    D --> E
\`\`\`

### Ordered List
1. First item
2. Second item
   1. Nested item A
   2. Nested item B
3. Third item

### Unordered List
- Alpha
- Beta
  - Sub-item
  - Another sub-item
- Gamma

### Blockquote
> This is a blockquote with **bold** and *italic* text.
>
> It can span multiple paragraphs.

\`\`\`mermaid
sequenceDiagram
    participant Driver as Driver
    participant HMI as Human-machine interface (HMI/Cluster)
    participant ADAS as Combined driver assistance system (ADAS ECU)
    participant Sensor as Sensor/positioning module
    participant Cloud as Cloud service platform (optional)

    Note over ADAS, Sensor: Phase 1: normal operation and monitoring
    Driver->>Sensor: Vehicle driving normally
    Sensor->>ADAS: Status data (environment, position, vehicle state)
    ADAS-->>Driver: Keep combined driver assistance active (icon shown as normal)

    Note over ADAS, Sensor: Phase 2: takeover condition triggered
    alt System detects a takeover scenario
        Sensor->>ADAS: Condition met (e.g. missing map data/speed limit change/system fault/driver distraction)
        ADAS->>HMI: Send takeover request signal (HOR Signal)
        
        Note right of HMI: Phase 3: tiered alert strategy (meets national standard requirements)
        HMI-->>Driver: Visual cue (cluster icon flashing/color change)
        HMI-->>Driver: Audible cue (soft beep)
        
        ADAS->>HMI: Escalated alert (if the driver does not respond)
        HMI-->>Driver: Strong visual warning (red border/text)
        HMI-->>Driver: Strong audible warning (continuous rapid beeping)
        HMI-->>Driver: Haptic cue (steering wheel/seat vibration)
    end

    Note over Driver, ADAS: Phase 4: driver response handling
    alt Driver takes over in time
        Driver->>HMI: Hands on the steering wheel (Torque/Grip Detection)
        HMI->>ADAS: Confirm driver intervention signal
        ADAS-->>Driver: Exit automated driving, switch to manual driving mode
        ADAS->>HMI: Clear the warning prompts
    else Driver does not respond
        alt Final takeover deadline reached (e.g., T+5s)
            ADAS->>ADAS: Start the minimal risk maneuver (MRM/MLR)
            ADAS->>HMI: Trigger the emergency deceleration/stop prompt
            HMI-->>Driver: Emergency warning (highest level)
            ADAS->>Sensor: Perform a safe stop (pull over, brake, hazard lights)
        end
    end

    Note over Cloud, Driver: Phase 5: data logging and reporting
    ADAS->>Cloud: Upload takeover event data (time, cause, driver response)
    Note right of Cloud: Used for accident liability and algorithm tuning
\`\`\`

Done.`;

const streamBuffer = ref('');
const isStreaming = ref(false);
const streamSpeed = ref(30);
const customInput = ref('');
const streamMermaidSvgHtml = ref<string[]>([]);
let streamTimer: ReturnType<typeof setInterval> | null = null;
let streamMermaidRenderTask: Promise<void> | null = null;

// Pre-render static fixtures once so streaming ticks do not reset other sections.
latexCases.forEach((tc) => {
  tc.html = render(tc.raw);
});
const codeBlockHtml = render(codeBlockSample);
const basicTextHtml = render(basicTextSample);
const tableHtml = render(tableSample);
const listsHtml = render(listsSample);
const mixedHtml = render(mixedSample);
const mermaidHtml = render(mermaidSample);

const streamHtml = computed(() => renderStreamMarkdown(streamBuffer.value));
const customHtml = computed(() => render(customInput.value));

const cacheStreamMermaidSvg = async () => {
  const codes = extractMermaidCodes(streamBuffer.value);
  if (codes.length <= streamMermaidSvgHtml.value.length) return;

  if (!streamMermaidRenderTask) {
    streamMermaidRenderTask = (async () => {
      streamMermaidSvgHtml.value = await appendMermaidSvgCache(
        extractMermaidCodes(streamBuffer.value),
        streamMermaidSvgHtml.value,
        'mermaid-stream',
      );
    })().finally(() => {
      streamMermaidRenderTask = null;
    });
  }

  await streamMermaidRenderTask;

  if (extractMermaidCodes(streamBuffer.value).length > streamMermaidSvgHtml.value.length) {
    await cacheStreamMermaidSvg();
  }
};

const startStream = () => {
  resetStream();
  isStreaming.value = true;
  let idx = 0;
  const tick = () => {
    if (idx >= fullStreamText.length) {
      if (streamTimer) clearInterval(streamTimer);
      isStreaming.value = false;
      return;
    }
    streamBuffer.value += fullStreamText[idx++];
  };
  streamTimer = setInterval(tick, streamSpeed.value);
};

const resetStream = () => {
  if (streamTimer) clearInterval(streamTimer);
  streamBuffer.value = '';
  streamMermaidSvgHtml.value = [];
  streamMermaidRenderTask = null;
  isStreaming.value = false;
};

const refreshMermaid = async (root?: HTMLElement | null) => {
  await nextTick();
  await enhanceMarkdownContainer(root ?? mermaidContainer.value);
};

onMounted(() => refreshMermaid());

watch(streamBuffer, () => {
  void cacheStreamMermaidSvg();
});

watch(streamHtml, () => {
  nextTick(() => refreshMermaid(streamContainer.value));
});

watch(isStreaming, (streaming) => {
  if (!streaming) {
    void cacheStreamMermaidSvg().then(() => refreshMermaid(streamContainer.value));
  }
});

let customMermaidTimer: ReturnType<typeof setTimeout> | null = null;
const COMPLETE_MERMAID_RE = /```mermaid[\s\S]*?```/;

watch(customInput, () => {
  if (!COMPLETE_MERMAID_RE.test(customInput.value)) return;
  if (customMermaidTimer) clearTimeout(customMermaidTimer);
  customMermaidTimer = setTimeout(() => {
    customMermaidTimer = null;
    void refreshMermaid(customContainer.value);
  }, 200);
});
</script>

<style lang="less" scoped>
@import '../../components/css/chat-markdown.less';
@import '../../components/css/chat-citations.less';
@import '../../components/css/chat-message-shared.less';
@import '../../components/css/chat-timeline-loading.less';

.markdown-test-page {
  max-width: 860px;
  margin: 0 auto;
  padding: 32px 24px;
  font-family: var(--app-font-family);
}

.page-title {
  font-size: var(--app-text-4xl);
  font-weight: 700;
  margin-bottom: 4px;
}

.page-desc {
  color: var(--td-text-color-secondary);
  font-size: var(--app-text-base);
  margin-bottom: 32px;
}

.test-section {
  margin-bottom: 36px;
  border-bottom: 1px solid var(--td-component-stroke);
  padding-bottom: 24px;

  h2 {
    font-size: var(--app-text-2xl);
    font-weight: 600;
    margin-bottom: 12px;
  }
}

.test-hint {
  font-size: var(--app-text-md);
  color: var(--td-text-color-secondary);
  margin-bottom: 8px;
}

.test-case {
  margin: 12px 0;
}

.test-raw {
  background: var(--td-bg-color-secondarycontainer);
  padding: 6px 10px;
  border-radius: var(--app-radius-xs);
  margin-bottom: 6px;
  font-size: var(--app-text-md);
  overflow-x: auto;

  code {
    white-space: pre-wrap;
    word-break: break-all;
  }
}

.test-rendered {
  padding: 8px 12px;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-sm);
  background: var(--td-bg-color-container);
}

.stream-controls {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.btn {
  padding: 4px 16px;
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-xs);
  background: var(--td-bg-color-container);
  cursor: pointer;
  font-size: var(--app-text-md);

  &:hover {
    background: var(--td-bg-color-container-hover);
  }

  &:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
}

.speed-label {
  font-size: var(--app-text-md);
  display: flex;
  align-items: center;
  gap: 6px;

  input[type="range"] {
    width: 120px;
  }
}

.custom-textarea {
  width: 100%;
  padding: 10px;
  font-family: var(--app-font-family-mono);
  font-size: var(--app-text-md);
  border: 1px solid var(--td-component-stroke);
  border-radius: var(--app-radius-sm);
  resize: vertical;
  box-sizing: border-box;
  margin-bottom: 12px;
}

.shimmer-demo {
  display: flex;
  flex-direction: column;
  gap: 14px;

  .action-card {
    background: transparent;
  }

  .action-name {
    font-size: var(--app-text-base);
    line-height: 1.55;
    color: var(--td-text-color-secondary);
  }
}

// Chat answer markdown - shared with botmsg / AgentStreamDisplay / embed
.markdown-content {
  // Dev page intentionally uses the same chat Markdown mixin as runtime chat.
  // Keep visual changes in chat-markdown.less so this page remains a regression target.
  .chat-markdown-typography();
  .chat-citation-pills();
}
</style>
