const { getSettings, saveSettings } = require("../../utils/config");
const { createKnowledgeFromURL, listKnowledgeBases } = require("../../utils/request");
const { getMessages, t } = require("../../utils/i18n");

function normalizeKnowledgeBases(response) {
  if (!response) {
    return [];
  }
  if (Array.isArray(response.data)) {
    return response.data;
  }
  if (response.data && Array.isArray(response.data.list)) {
    return response.data.list;
  }
  if (Array.isArray(response.knowledge_bases)) {
    return response.knowledge_bases;
  }
  return [];
}

const messages = getMessages();

Page({
  data: {
    knowledgeTitle: messages.knowledgeTitle,
    knowledgeSubtitle: messages.knowledgeSubtitle,
    needsSettingsText: messages.needsSettings,
    openSettingsText: messages.openSettings,
    knowledgeBaseLabel: messages.knowledgeBase,
    tapToSelectText: messages.tapToSelect,
    refreshText: messages.refreshKnowledgeBases,
    urlLabel: messages.urlLabel,
    urlPlaceholder: messages.urlPlaceholder,
    importUrlText: messages.importUrl,
    displayKnowledgeBaseName: messages.tapToSelect,
    importing: false,
    knowledgeBases: [],
    knowledgeBaseNames: [],
    loading: false,
    needsSettings: false,
    selectedIndex: 0,
    selectedKnowledgeBaseId: "",
    selectedKnowledgeBaseName: "",
    statusMessage: "",
    url: ""
  },

  onShow() {
    try {
      const settings = getSettings();
      const needsSettings = !settings.baseUrl || !settings.apiKey;
      const patch = { needsSettings };
      if (settings.selectedKnowledgeBaseId) {
        patch.selectedKnowledgeBaseId = settings.selectedKnowledgeBaseId;
      }
      this.setData(patch);

      if (needsSettings) {
        return;
      }
      this.loadKnowledgeBases();
    } catch (error) {
      this.setData({
        needsSettings: true,
        statusMessage: (error && error.message) || "page init failed"
      });
    }
  },

  onUrlInput(event) {
    this.setData({ url: event.detail.value });
  },

  onKnowledgeBaseChange(event) {
    const selectedIndex = Number(event.detail.value);
    this.selectKnowledgeBase(selectedIndex);
  },

  onKnowledgeBaseTap(event) {
    this.selectKnowledgeBase(Number(event.currentTarget.dataset.index));
  },

  selectKnowledgeBase(selectedIndex) {
    const selected = this.data.knowledgeBases[selectedIndex];
    if (!selected) return;

    saveSettings({ selectedKnowledgeBaseId: selected.id });
    this.setData({
      selectedIndex,
      selectedKnowledgeBaseId: selected.id,
      selectedKnowledgeBaseName: selected.name,
      displayKnowledgeBaseName: selected.name || this.data.tapToSelectText
    });
  },

  openSettings() {
    wx.switchTab({ url: "/pages/settings/settings" });
  },

  async loadKnowledgeBases() {
    const settings = getSettings();
    if (!settings.baseUrl || !settings.apiKey) {
      this.setData({ needsSettings: true });
      return;
    }

    this.setData({ loading: true, statusMessage: "" });
    try {
      const response = await listKnowledgeBases();
      const knowledgeBases = normalizeKnowledgeBases(response);
      const knowledgeBaseNames = knowledgeBases.map((item) => item.name || item.id);
      const current = getSettings();
      let selectedIndex = knowledgeBases.findIndex(
        (item) => item.id === current.selectedKnowledgeBaseId
      );
      if (selectedIndex < 0) {
        selectedIndex = 0;
      }
      const selected = knowledgeBases[selectedIndex];
      const selectedName = (selected && selected.name) || "";
      this.setData({
        knowledgeBases,
        knowledgeBaseNames,
        selectedIndex,
        selectedKnowledgeBaseId: (selected && selected.id) || "",
        selectedKnowledgeBaseName: selectedName,
        displayKnowledgeBaseName: selectedName || this.data.tapToSelectText,
        statusMessage: knowledgeBases.length
          ? t("loadedKnowledgeBases", { count: knowledgeBases.length })
          : t("noKnowledgeBases")
      });
      if (selected && selected.id) {
        saveSettings({ selectedKnowledgeBaseId: selected.id });
      }
    } catch (error) {
      wx.showModal({
        title: t("loadFailed"),
        content: (error && error.message) || "",
        showCancel: false
      });
    } finally {
      this.setData({ loading: false });
    }
  },

  async importURL() {
    this.setData({ importing: true });
    try {
      await createKnowledgeFromURL(this.data.selectedKnowledgeBaseId, this.data.url.trim(), false);
      this.setData({ url: "" });
      wx.showToast({ title: t("imported"), icon: "success" });
    } catch (error) {
      wx.showModal({
        title: t("importFailed"),
        content: (error && error.message) || "",
        showCancel: false
      });
    } finally {
      this.setData({ importing: false });
    }
  }
});
