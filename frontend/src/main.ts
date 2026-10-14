import { createApp } from "vue";
import { createPinia } from "pinia";
import App from "./App.vue";
import router from "./router";
import TDesign from "tdesign-vue-next";
// A few global style variables from the component library
import "tdesign-vue-next/dist/tdesign.css";
import "@/assets/theme/theme.css";
import "@/assets/theme/tdesign-overrides.less";
import "@/assets/dropdown-menu.less";
import "@/components/css/chat-hljs-dark.less";
// vue-virtual-scroller ships its own tiny stylesheet - required for
// RecycleScroller/DynamicScroller to size their viewport correctly.
// Without it the scroller computes 0 height and renders no items.
import "vue-virtual-scroller/dist/vue-virtual-scroller.css";
import i18n from "./i18n";
import { initTheme } from "@/composables/useTheme";
import { initFont } from "@/composables/useFont";
import { installTDesignIconOfflineGuard } from "@/utils/tdesign-icon-offline";
import { installAutofillGuard } from "@/utils/disable-autofill";
import { useAuthStore } from "@/stores/auth";
import { useDeploymentCapabilitiesStore } from "@/stores/deploymentCapabilities";

// Must run before Vue components mount, so tdesign-icons does not request tdesign.gtimg.com at run time
installTDesignIconOfflineGuard();

initTheme();
initFont();

async function bootstrap() {
  const app = createApp(App);

  // Global error handling: catch unhandled component errors to prevent a blank screen
  app.config.errorHandler = (err, instance, info) => {
    console.error("[AI KB] Unhandled Vue error:", err, "\nComponent:", instance, "\nInfo:", info);
  };

  app.use(TDesign);
  const pinia = createPinia();
  app.use(pinia);

  // Capabilities (can_create_tenant, auto_accept_invitation) are not cached
  // in localStorage - reconcile once before first paint when a session exists.
  const authStore = useAuthStore();
  if (localStorage.getItem("aikb_token")) {
    // best-effort; capabilities stay at defaults until the next refresh
    const authMe = authStore.refreshFromAuthMe().catch(() => {});
    // The router guard sends this same request next (auth/me does not change its headers); starting it now overlaps the two.
    void useDeploymentCapabilitiesStore().ensureLoaded();
    // Fetch the landing route's lazy chunks meanwhile instead of after the router guard.
    for (const record of router.resolve(location.pathname + location.search).matched) {
      for (const component of Object.values(record.components ?? {})) {
        if (typeof component === "function") (component as () => Promise<unknown>)().catch(() => {});
      }
    }
    await authMe;
  }

  app.use(router);
  app.use(i18n);

  // Mount after the first route (navigation guards and Lite auto-login included) resolves, so the default page does not flash before the redirect
  await router.isReady();
  app.mount("#app");
  installAutofillGuard();
}

bootstrap();
