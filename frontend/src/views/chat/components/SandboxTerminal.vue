<template>
    <div ref="containerRef" class="sandbox-terminal" :class="{ 'is-dark': isDarkTheme }"
        @mousedown="focusTerminal">
        <div v-if="status !== 'ready'" class="sandbox-terminal__overlay">
            <div class="sandbox-terminal__overlay-card">
                <t-icon v-if="status === 'connecting'" name="loading" size="24px"
                    class="sandbox-terminal__spinner" />
                <t-icon
                    v-else-if="status === 'paused' || status === 'needs_provision' || status === 'no_sandbox'"
                    name="terminal" size="28px" />
                <t-icon v-else-if="status === 'unsupported'" name="error-circle" size="28px" />
                <t-icon v-else-if="status === 'idle'" name="time" size="28px" />
                <t-icon v-else name="cloud" size="28px" />
                <p class="sandbox-terminal__overlay-text">{{ statusText }}</p>
                <t-button v-if="actionLabel" size="small"
                    :theme="status === 'needs_provision' ? 'primary' : 'default'" variant="outline"
                    @click.stop="start">
                    {{ actionLabel }}
                </t-button>
            </div>
        </div>
        <div ref="terminalHost" class="sandbox-terminal__host"></div>
    </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, toRef, watch } from 'vue';
import { useI18n } from 'vue-i18n';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';
import { useSandboxTerminal, type SandboxTerminalStatus } from '@/composables/useSandboxTerminal';
import { useTheme } from '@/composables/useTheme';
import { createPtyEchoPredictor } from '@/utils/ptyEchoPredictor';
import {
    PTY_PROMPT_NUDGE_DELAY_MS,
    xtermBufferLooksEmpty,
} from '@/utils/ptyPromptNudge';

const props = defineProps<{
    sessionId: string;
    /** The agent selected in the current session: on first connect the backend creates the sandbox from its config. */
    agentId?: string;
    /** Source workspace of a shared agent, matching agent_source_tenant_id on the chat request. */
    agentSourceTenantId?: string | number | null;
}>();

const { t } = useI18n();

const containerRef = ref<HTMLElement | null>(null);
const terminalHost = ref<HTMLElement | null>(null);

// The theme must derive from useTheme's shared ref, not the `theme-mode` DOM attribute: a DOM attribute
// is not a reactive source, so a computed written that way has no dependencies, evaluates once and is cached forever,
// the watch below never fires, and the terminal colors stay as they were at first mount.
const { currentTheme } = useTheme();
const systemPrefersDark = ref(prefersDarkQuery()?.matches === true);
const isDarkTheme = computed(() =>
    currentTheme.value === 'system' ? systemPrefersDark.value : currentTheme.value === 'dark',
);

function prefersDarkQuery(): MediaQueryList | null {
    if (typeof window === 'undefined' || !window.matchMedia) return null;
    return window.matchMedia('(prefers-color-scheme: dark)');
}

// In `system` mode it must also follow live OS switches; useTheme's global listener only writes the DOM
// attribute and does not expose the effective light/dark value, so this subscribes on its own.
const prefersDark = prefersDarkQuery();
const onSystemThemeChange = (event: MediaQueryListEvent) => {
    systemPrefersDark.value = event.matches;
};
prefersDark?.addEventListener('change', onSystemThemeChange);

// ANSI palette: ls --color uses the usual dircolors mapping (dir=blue,
// exec=green, link=cyan). Only the green slots stay AI KB brand so
// user@host (01;32) matches the product color; path (01;34) stays blue
// like directories.
function xtermTheme(dark: boolean) {
    return dark
        ? {
            background: '#1a1a1a',
            foreground: '#e6e6e6',
            cursor: '#e6e6e6',
            cursorAccent: '#1a1a1a',
            selectionBackground: '#3a3a3a',
            red: '#c64751',
            brightRed: '#de6670',
            green: '#06b04d',
            brightGreen: '#07c05f',
            yellow: '#c4a000',
            brightYellow: '#fce94f',
            blue: '#3465a4',
            brightBlue: '#729fcf',
            magenta: '#75507b',
            brightMagenta: '#ad7fa8',
            cyan: '#06989a',
            brightCyan: '#34e2e2',
        }
        : {
            background: '#ffffff',
            foreground: '#242424',
            cursor: '#242424',
            cursorAccent: '#ffffff',
            selectionBackground: '#d0d7de',
            red: '#e34d59',
            brightRed: '#f36d78',
            green: '#06b04d',
            brightGreen: '#07c05f',
            yellow: '#c4a000',
            brightYellow: '#c4a000',
            blue: '#3465a4',
            brightBlue: '#729fcf',
            magenta: '#75507b',
            brightMagenta: '#ad7fa8',
            cyan: '#06989a',
            brightCyan: '#34e2e2',
        };
}

// toRef rather than ref(props.x): the latter is a snapshot, so reconnecting after switching agents would still create
// the sandbox with the old agent's sandbox config. sessionId is covered by the parent's :key rebuild; agentId is not.
const terminal = useSandboxTerminal(
    toRef(props, 'sessionId'),
    toRef(props, 'agentId'),
    toRef(props, 'agentSourceTenantId'),
);
const { status, sandboxMayBeReclaimed, reattachable } = terminal;

const statusText = computed(() => {
    switch (status.value as SandboxTerminalStatus) {
        case 'paused':
            return sandboxMayBeReclaimed.value
                ? t('chat.sandbox.stopped')
                : t('chat.sandbox.paused');
        case 'connecting':
            return t('chat.sandbox.connecting');
        case 'needs_provision':
            return t('chat.sandbox.needsProvision');
        case 'no_sandbox':
            return t('chat.sandbox.noSandbox');
        case 'unsupported':
            return t('chat.sandbox.unsupported');
        case 'exited':
            return t('chat.sandbox.sessionEnded');
        case 'idle':
            return reattachable.value
                ? t('chat.sandbox.idleDisconnected')
                : t('chat.sandbox.idleDisconnectedNewShell');
        case 'unauthorized':
            return t('chat.sandbox.authRevoked');
        default:
            return reattachable.value
                ? t('chat.sandbox.disconnected')
                : t('chat.sandbox.disconnectedNewShell');
    }
});

// Overlay button: paused uses "Start terminal", not yet created uses "Create and start". Only a click may
// create or wake the sandbox; the lookup when the panel opens does neither.
const actionLabel = computed(() => {
    switch (status.value as SandboxTerminalStatus) {
        case 'paused':
            return t('chat.sandbox.start');
        case 'needs_provision':
            return t('chat.sandbox.createAndStart');
        case 'error':
        case 'exited':
        case 'idle':
        case 'unauthorized':
        // no_sandbox means "the agent has no sandbox backend configured, so there is nowhere to create one". Once the user configures it,
        // they should be able to retry directly rather than only close and reopen the panel, a dead end with no way forward.
        case 'no_sandbox':
            return t('chat.sandbox.retry');
        default:
            return '';
    }
});

// The xterm instance mounts once after ready; the overlay sits on top to show status.
let xterm: Terminal | null = null;
let fitAddon: FitAddon | null = null;
let echo: ReturnType<typeof createPtyEchoPredictor> | null = null;
let resizeObserver: ResizeObserver | null = null;
let resizeDebounce: ReturnType<typeof setTimeout> | null = null;
let promptNudgeTimer: ReturnType<typeof setTimeout> | null = null;
let unmounted = false;

function writeToXterm(chunk: string | Uint8Array) {
    xterm?.write(chunk);
}

function focusTerminal() {
    if (status.value !== 'ready') return;
    xterm?.focus();
}

function mountTerminal() {
    if (!terminalHost.value || xterm) return;
    xterm = new Terminal({
        cursorBlink: true,
        cursorStyle: 'bar',
        cursorInactiveStyle: 'outline',
        fontSize: 13,
        fontFamily: "'SFMono-Regular', Consolas, 'Liberation Mono', Menlo, monospace",
        theme: xtermTheme(isDarkTheme.value),
        scrollback: 5000,
    });
    echo = createPtyEchoPredictor(writeToXterm);
    fitAddon = new FitAddon();
    xterm.loadAddon(fitAddon);
    xterm.open(terminalHost.value);
    xterm.onData((data) => {
        echo?.onLocal(data);
        terminal.sendInput(data);
    });

    resizeObserver = new ResizeObserver(() => {
        if (resizeDebounce) clearTimeout(resizeDebounce);
        resizeDebounce = setTimeout(() => applyFit(), 100);
    });
    resizeObserver.observe(containerRef.value || terminalHost.value);
    // Fit BEFORE flushing buffered PTY bytes. FitAddon.fit() calls
    // _renderService.clear() when the default 80x24 becomes the panel size,
    // which would wipe a prompt painted a moment earlier and leave only the
    // cursor until the next keystroke.
    applyFit();
    terminal.onOutput((data) => echo?.onRemote(data));
    schedulePromptNudge();
    void nextTick(() => {
        requestAnimationFrame(() => fitAndFocus());
    });
}

function unmountTerminal() {
    terminal.onOutput(null);
    echo = null;
    resizeObserver?.disconnect();
    resizeObserver = null;
    if (resizeDebounce) clearTimeout(resizeDebounce);
    resizeDebounce = null;
    if (promptNudgeTimer) clearTimeout(promptNudgeTimer);
    promptNudgeTimer = null;
    xterm?.dispose();
    xterm = null;
    fitAddon = null;
}

// The only entry on the overlay that creates or wakes a sandbox. provision: true marks the user's explicit
// confirmation. Mounting only does a lookup: a running sandbox connects directly, while a paused or not yet created one waits
// on the overlay for a click.
function start() {
    unmountTerminal();
    const { cols, rows } = estimatePtySize();
    terminal.connect({ provision: true, cols, rows });
}

function connectLookup() {
    const { cols, rows } = estimatePtySize();
    terminal.connect({ provision: false, cols, rows });
}

onMounted(() => {
    connectLookup();
});

watch(isDarkTheme, (dark) => {
    if (xterm) xterm.options.theme = xtermTheme(dark);
});

// xterm 6 no longer paints its viewport with the theme background, so the
// stylesheet's #000 showed as a strip below the last row.
const terminalBackground = computed(() => xtermTheme(isDarkTheme.value).background);

// Mount xterm after ready; leaving the ready state resets the instance (reconnect = new PTY).
watch(status, (next, prev) => {
    if (next === 'ready' && prev !== 'ready') {
        requestAnimationFrame(() => {
            // The component may unmount between ready and the next frame (panel closed / session switched). Without this check
            // a Terminal would be created on a destroyed component, observing a detached node, and that
            // instance could not be rescued even by a watch.
            if (unmounted) return;
            mountTerminal();
        });
    } else if (prev === 'ready' && next !== 'ready') {
        unmountTerminal();
    }
});

// watch(status) only covers "the status leaves ready", not the component itself being destroyed: closing the panel goes through
// SandboxSidePanel's v-if and switching sessions through the :key rebuild, and on both paths status never changes,
// so the watch does not fire. Without this hook every panel open/close would leak one xterm instance
// (renderer + 5000 lines of scrollback) and one ResizeObserver.
onBeforeUnmount(() => {
    unmounted = true;
    prefersDark?.removeEventListener('change', onSystemThemeChange);
    unmountTerminal();
});

function estimatePtySize() {
    const el = containerRef.value;
    const width = Math.max(0, (el?.clientWidth ?? 0) - 16);
    const height = Math.max(0, (el?.clientHeight ?? 0) - 16);
    return {
        cols: Math.max(20, Math.floor(width / 8) || 80),
        rows: Math.max(8, Math.floor(height / 17) || 24),
    };
}

function xtermVisibleBufferEmpty(): boolean {
    if (!xterm) return true;
    const buf = xterm.buffer.active;
    const origin = buf.viewportY;
    return xtermBufferLooksEmpty(
        (row) => buf.getLine(origin + row)?.translateToString(true),
        xterm.rows,
    );
}

// Pty.Connect does not replay a prompt bash already printed. A same-size
// resize is a no-op; flipping rows by 1 sends SIGWINCH so readline (and
// TUIs) redraw. Do not inject Ctrl-L or Enter: that would go to whatever
// is running in a reattached PTY.
function schedulePromptNudge() {
    if (promptNudgeTimer) clearTimeout(promptNudgeTimer);
    promptNudgeTimer = setTimeout(() => {
        promptNudgeTimer = null;
        if (unmounted || !xterm || status.value !== 'ready') return;
        if (!xtermVisibleBufferEmpty()) return;
        const cols = Math.max(2, xterm.cols);
        const rows = Math.max(2, xterm.rows);
        terminal.resize(cols, rows - 1);
        terminal.resize(cols, rows);
    }, PTY_PROMPT_NUDGE_DELAY_MS);
}

// When v-show hides the terminal the container is 0x0. FitAddon may still compute 2x1 and send it to the PTY,
// and on switching back to the terminal tab bash is still at that size, which looks disconnected. Do nothing when the size is too small.
function containerHasPtySize() {
    const el = containerRef.value;
    return !!el && el.clientWidth >= 20 && el.clientHeight >= 20;
}

function applyFit() {
    if (!xterm || !containerHasPtySize()) return;
    try {
        fitAddon?.fit();
    } catch {
        // fit throws when the container size is 0; ignore it.
        return;
    }
    if (xterm.cols < 2 || xterm.rows < 2) return;
    terminal.resize(xterm.cols, xterm.rows);
    xterm.refresh(0, xterm.rows - 1);
}

function fitAndFocus() {
    // Right after v-show opens, layout may not be done in nextTick, so wait two more frames before fit.
    requestAnimationFrame(() => {
        requestAnimationFrame(() => {
            if (unmounted) return;
            applyFit();
            xterm?.focus();
        });
    });
}

defineExpose({
    focus: fitAndFocus,
});
</script>

<style scoped lang="less">
.sandbox-terminal {
    position: relative;
    height: 100%;
    min-height: 0;
    display: flex;
    flex-direction: column;
    background: var(--td-bg-color-container);
    border-radius: var(--app-radius-md);
    overflow: hidden;
    cursor: text;
}

.sandbox-terminal__host {
    flex: 1;
    min-height: 0;
    padding: 8px;
    cursor: text;

    :deep(.xterm) {
        height: 100%;
        cursor: text;
    }

    :deep(.xterm-viewport) {
        overflow-y: auto;
        background-color: v-bind(terminalBackground);
    }

    :deep(.xterm-helper-textarea) {
        // xterm receives keys through a hidden textarea; it must be focusable for the cursor to blink.
        pointer-events: auto;
    }
}

.sandbox-terminal__overlay {
    position: absolute;
    inset: 0;
    z-index: 2;
    display: flex;
    align-items: center;
    justify-content: center;
    background: var(--td-bg-color-container);
    cursor: default;
}

.sandbox-terminal__overlay-card {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 10px;
    max-width: 280px;
    padding: 20px;
    text-align: center;
    color: var(--td-text-color-placeholder);
}

.sandbox-terminal__overlay-text {
    margin: 0;
    font-size: var(--app-text-md);
    line-height: 1.6;
    white-space: pre-line;
}

.sandbox-terminal__spinner {
    animation: wk-spin 0.9s linear infinite;
}

</style>
