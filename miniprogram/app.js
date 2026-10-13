App({
  onLaunch() {
    const settings = wx.getStorageSync("aikb_settings");
    if (!settings) {
      wx.setStorageSync("aikb_settings", {
        baseUrl: "http://localhost:8080",
        apiKey: "",
        selectedKnowledgeBaseId: "",
        locale: "zh"
      });
    }
  }
});
