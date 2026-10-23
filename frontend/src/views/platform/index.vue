<template>
    <div class="main" :class="{ 'main--overlay': uiStore.sidebarOverlay }" ref="dropzone" :style="{ '--sidebar-width': `${uiStore.sidebarDisplayWidth}px` }">
        <Menu></Menu>
        <button v-if="uiStore.sidebarOverlay && !uiStore.sidebarOverlayOpen" class="sidebar-open-btn" type="button"
            :aria-label="t('menu.openSidebar')" @click="uiStore.toggleSidebar">
            <svg viewBox="0 0 16 16" width="16" height="16" fill="none" xmlns="http://www.w3.org/2000/svg">
                <line x1="2" y1="4" x2="14" y2="4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
                <line x1="2" y1="8" x2="14" y2="8" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
                <line x1="2" y1="12" x2="14" y2="12" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
            </svg>
        </button>
        <div class="sidebar-backdrop" v-if="uiStore.sidebarOverlayOpen" @click="uiStore.sidebarOverlayOpen = false" />
        <div v-if="isRouterAlive" class="platform-route-outlet">
            <RouterView />
        </div>
        <div class="upload-mask" v-show="ismask">
            <UploadMask></UploadMask>
        </div>
        <!-- Global settings modal, shared by every platform child route -->
        <Settings />
        <!-- Global command palette (⌘K), lives as long as the platform route -->
        <GlobalCommandPalette />
        <!-- Global top-right "pending invitations" bell. Fixed position with a z-index below drawers, so a page's
             right-side drawer covers it naturally; rendered only when invitations are pending. -->
        <GlobalInvitationBell />
        <!-- Knowledge base upload progress overlay: the upload queue lives in the store, so switching pages does not interrupt it -->
        <UploadTasksPanel />
        <!-- Onboarding tour with a mask: opens automatically on first visit, and can be reopened from the help button next to the nickname at the top of the user menu -->
        <NewUserGuide />
    </div>
</template>
<script setup lang="ts">
import Menu from '@/components/menu.vue'
import { ref, onMounted, onUnmounted, nextTick, provide, watch } from 'vue';
import { useRoute, useRouter } from 'vue-router'
import UploadMask from '@/components/upload-mask.vue'
import Settings from '@/views/settings/Settings.vue'
import GlobalCommandPalette from '@/components/GlobalCommandPalette.vue'
import GlobalInvitationBell from '@/components/GlobalInvitationBell.vue'
import UploadTasksPanel from '@/components/upload-tasks/UploadTasksPanel.vue'
import NewUserGuide from '@/components/NewUserGuide.vue'
import { useCommandPaletteStore } from '@/stores/commandPalette'
import { useChatResourcesStore } from '@/stores/chatResources'
import { useUIStore } from '@/stores/ui'
import { getKnowledgeBaseById } from '@/api/knowledge-base/index'
import { MessagePlugin } from 'tdesign-vue-next'
import { useI18n } from 'vue-i18n'
import { collectDroppedFiles } from './collectDroppedFiles'

const route = useRoute();
const router = useRouter();
const commandPaletteStore = useCommandPaletteStore();
const uiStore = useUIStore();
let ismask = ref(false)
const { t } = useI18n();

let sidebarOverlayQuery: MediaQueryList
const handleSidebarOverlayChange = (e: MediaQueryListEvent) => uiStore.setSidebarOverlay(e.matches)
const handleSidebarOverlayEscape = (e: KeyboardEvent) => {
    if (e.key === 'Escape') uiStore.sidebarOverlayOpen = false
}

watch(() => route.fullPath, () => { uiStore.sidebarOverlayOpen = false })

const isRouterAlive = ref(true)
const reloadApp = () => {
    isRouterAlive.value = false
    nextTick(() => {
        isRouterAlive.value = true
    })
}
provide('app:reload', reloadApp)

// Intercept Cmd/Ctrl+R only in the Wails desktop runtime:
// the desktop app has no browser address bar and a full reload shows a blank screen, so a frontend soft refresh replaces it.
// In a browser (the web build and non-Lite deployments included) it is not intercepted, so the browser does a real full reload;
// otherwise the left menu, global settings, Pinia stores and so on would not reset along with the "refresh".
// @ts-ignore
const isWailsDesktop = typeof window !== 'undefined' && !!(window as any).runtime?.EventsOn

const handleGlobalKeyDown = (e: KeyboardEvent) => {
    if (!isWailsDesktop) return
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'r') {
        e.preventDefault()
        reloadApp()
    }
}

// Counter for drag enter/leave, so dragleave fired by child elements is handled
let dragCounter = 0;

// Get the current knowledge base ID
const getCurrentKbId = (): string | null => {
    return (route.params as any)?.kbId as string || null
}

const CHAT_DROP_ROUTE_NAMES = new Set(['chat', 'globalCreatChat', 'kbCreatChat']);

const isChatDropRoute = () => {
    return CHAT_DROP_ROUTE_NAMES.has(String(route.name || ''));
}

// Check the knowledge base initialization state
const checkKnowledgeBaseInitialization = async (): Promise<boolean> => {
    const currentKbId = getCurrentKbId();
    
    if (!currentKbId) {
        MessagePlugin.error(t('knowledgeBase.missingId'));
        return false;
    }
    
    try {
        const kbResponse = await getKnowledgeBaseById(currentKbId);
        const kb = kbResponse.data;
        
        if (!kb.summary_model_id) {
            MessagePlugin.warning(t('knowledgeBase.notInitialized'));
            return false;
        }
        const strategy = kb.indexing_strategy;
        const needsEmbedding = !strategy || strategy.vector_enabled || strategy.keyword_enabled;
        if (needsEmbedding && !kb.embedding_model_id) {
            MessagePlugin.warning(t('knowledgeBase.notInitialized'));
            return false;
        }
        return true;
    } catch (error) {
        MessagePlugin.error(t('knowledgeBase.getInfoFailed'));
        return false;
    }
}


// isFileDrag distinguishes an OS file drag (the only thing the global upload
// drop zone cares about) from an in-app element drag such as the wiki
// folder/page drag-and-drop. Element drags carry only "text/*" types, never
// "Files", so we bail out and let the originating component handle the drop.
const isFileDrag = (event: DragEvent): boolean => {
    const types = event.dataTransfer?.types
    if (!types) return false
    return Array.from(types).includes('Files')
}

const shouldHandleGlobalFileDrag = (event: DragEvent): boolean => {
    if (!isFileDrag(event)) return false;
    // Keep the browser from opening dropped files, even outside upload pages.
    event.preventDefault();
    // Settings and its teleported skill drawers own their uploads. This runs
    // in document capture, before a local drop handler can stop propagation.
    const enabled = !uiStore.showSettingsModal && (
        isChatDropRoute() || (route.name === 'knowledgeBaseDetail' && !!getCurrentKbId())
    );
    if (!enabled) {
        dragCounter = 0;
        ismask.value = false;
    }
    return enabled;
}

// Global drag event handling
const handleGlobalDragEnter = (event: DragEvent) => {
    if (!shouldHandleGlobalFileDrag(event)) return;
    dragCounter++;
    if (event.dataTransfer) {
        event.dataTransfer.effectAllowed = 'all';
    }
    ismask.value = true;
}

const handleGlobalDragOver = (event: DragEvent) => {
    if (!shouldHandleGlobalFileDrag(event)) return;
    if (event.dataTransfer) {
        event.dataTransfer.dropEffect = 'copy';
    }
}

const handleGlobalDragLeave = (event: DragEvent) => {
    if (!shouldHandleGlobalFileDrag(event)) return;
    dragCounter--;
    if (dragCounter === 0) {
        ismask.value = false;
    }
}

const handleGlobalDrop = async (event: DragEvent) => {
    if (!shouldHandleGlobalFileDrag(event)) return;
    dragCounter = 0;
    ismask.value = false;

    const droppedFiles = await collectDroppedFiles(event);
    if (droppedFiles.length === 0) {
        MessagePlugin.warning(t('knowledgeBase.dragFileNotText'));
        return;
    }

    if (isChatDropRoute()) {
        event.stopPropagation();
        window.dispatchEvent(new CustomEvent('aikb:chat-file-drop', {
            detail: { files: droppedFiles }
        }));
        return;
    }
    
    const isInitialized = await checkKnowledgeBaseInitialization();
    if (!isInitialized) {
        return;
    }

    window.dispatchEvent(new CustomEvent('aikb:knowledge-file-drop', {
        detail: { kbId: getCurrentKbId(), files: droppedFiles }
    }));
}

// Add global event listeners when the component mounts
onMounted(() => {
    sidebarOverlayQuery = window.matchMedia('(max-width: 767px)')
    uiStore.setSidebarOverlay(sidebarOverlayQuery.matches)
    sidebarOverlayQuery.addEventListener('change', handleSidebarOverlayChange)
    window.addEventListener('keydown', handleSidebarOverlayEscape)
    document.addEventListener('dragenter', handleGlobalDragEnter, true);
    document.addEventListener('dragover', handleGlobalDragOver, true);
    document.addEventListener('dragleave', handleGlobalDragLeave, true);
    document.addEventListener('drop', handleGlobalDrop, true);
    if (isWailsDesktop) {
        window.addEventListener('keydown', handleGlobalKeyDown);
        // @ts-ignore
        window.runtime.EventsOn('app:reload', () => {
            reloadApp()
        })
    }
    // Support opening the global command palette through a URL query parameter, for example the old path
    // /platform/knowledge-search?q=foo redirects with ?cmdk=foo
    maybeOpenCmdkFromRoute()
    // Prefetch the chat input bar resources in the background, reused from cache on entering creatChat / chat
    void useChatResourcesStore().prefetchChatInput()
});

// Watch route changes, handling the ?cmdk= parameter on in-app SPA navigation
watch(() => route.query.cmdk, () => {
    maybeOpenCmdkFromRoute()
})

function maybeOpenCmdkFromRoute() {
    if (!('cmdk' in route.query)) return
    const q = String(route.query.cmdk ?? '')
    commandPaletteStore.openPalette(q)
    // Clear the query so going back or refreshing does not trigger it again
    const newQuery = { ...route.query }
    delete (newQuery as any).cmdk
    router.replace({ path: route.path, query: newQuery, hash: route.hash })
}

// Remove global event listeners when the component unmounts
onUnmounted(() => {
    sidebarOverlayQuery.removeEventListener('change', handleSidebarOverlayChange)
    window.removeEventListener('keydown', handleSidebarOverlayEscape)
    document.removeEventListener('dragenter', handleGlobalDragEnter, true);
    document.removeEventListener('dragover', handleGlobalDragOver, true);
    document.removeEventListener('dragleave', handleGlobalDragLeave, true);
    document.removeEventListener('drop', handleGlobalDrop, true);
    if (isWailsDesktop) {
        window.removeEventListener('keydown', handleGlobalKeyDown);
        // @ts-ignore
        if (window.runtime?.EventsOff) {
            // @ts-ignore
            window.runtime.EventsOff('app:reload')
        }
    }
    dragCounter = 0;
});
</script>
<style lang="less">
.main {
    display: flex;
    align-items: stretch;
    width: 100%;
    height: 100%;
    min-height: 0;
    /* One page-wide background, so the left menu and the right content area read as one surface */
    background: var(--td-bg-color-container);
}

/* Below TDesign drawers (default z-index 1500), so an open drawer covers it */
.sidebar-open-btn {
    position: fixed;
    top: 8px;
    left: 8px;
    z-index: 100;
    width: 36px;
    height: 36px;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0;
    color: var(--td-text-color-primary);
    background: var(--td-bg-color-container);
    border: 1px solid var(--td-component-stroke);
    border-radius: 8px;
    cursor: pointer;
}

.sidebar-backdrop {
    position: fixed;
    inset: 0;
    z-index: 998;
    background: rgba(0, 0, 0, 0.35);
}

/* Right-hand route area: fills the remaining width and the full column height, and passes min-height:0 to child pages for inner flex scrolling */
.platform-route-outlet {
    flex: 1;
    min-width: 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
    overflow: hidden;
}

/* Keeps page headers clear of the fixed open button */
.main--overlay .platform-route-outlet {
    padding-top: 44px;
}

.upload-mask {
    background-color: rgba(255, 255, 255, 0.8);
    position: fixed;
    width: 100%;
    height: 100%;
    z-index: 999;
    display: flex;
    justify-content: center;
    align-items: center;
}

img {
    -webkit-user-drag: none;
    -khtml-user-drag: none;
    -moz-user-drag: none;
    -o-user-drag: none;
    user-drag: none;
}
</style>
