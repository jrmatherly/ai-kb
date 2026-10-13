const { getSettings, saveSettings } = require("../../utils/config");
const { getMessages, t } = require("../../utils/i18n");

const messages = getMessages();

Page({
  data: {
    settingsTitle: messages.settingsTitle,
    settingsSubtitle: messages.settingsSubtitle,
    apiBaseUrlLabel: messages.apiBaseUrl,
    apiBaseUrlPlaceholder: messages.apiBaseUrlPlaceholder,
    apiKeyLabel: messages.apiKey,
    apiKeyPlaceholder: messages.apiKeyPlaceholder,
    saveSettingsText: messages.saveSettings,
    baseUrl: "",
    apiKey: ""
  },

  onShow() {
    try {
      const settings = getSettings();
      this.setData({
        baseUrl: settings.baseUrl,
        apiKey: settings.apiKey
      });
    } catch (error) {
      // storage unavailable: keep the empty form
    }
  },

  onBaseUrlInput(event) {
    this.setData({ baseUrl: event.detail.value });
  },

  onApiKeyInput(event) {
    this.setData({ apiKey: event.detail.value });
  },

  save() {
    saveSettings({
      baseUrl: this.data.baseUrl,
      apiKey: this.data.apiKey
    });
    wx.showToast({ title: t("saved"), icon: "success" });
  }
});
