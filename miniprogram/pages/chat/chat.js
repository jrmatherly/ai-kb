const { getSettings } = require("../../utils/config");
const { createSession, knowledgeChat } = require("../../utils/request");
const { collectAnswerFromSSE } = require("../../utils/sse");
const { getMessages, t } = require("../../utils/i18n");

const messages = getMessages();

Page({
  data: {
    chatTitle: messages.chatTitle,
    chatSubtitle: messages.chatSubtitle,
    questionLabel: messages.question,
    questionPlaceholder: messages.questionPlaceholder,
    askText: messages.askAIKB,
    answerLabel: messages.answer,
    answer: "",
    displayAnswer: "",
    loading: false,
    query: "",
    rawResponse: "",
    sessionId: ""
  },

  onQueryInput(event) {
    this.setData({ query: event.detail.value });
  },

  async ensureSession() {
    if (this.data.sessionId) {
      return this.data.sessionId;
    }

    const settings = getSettings();
    const response = await createSession(settings.selectedKnowledgeBaseId);
    const sessionId = response && response.data ? response.data.id : "";
    if (!sessionId) {
      throw new Error(t("sessionIdMissing"));
    }
    this.setData({ sessionId: sessionId });
    return sessionId;
  },

  async ask() {
    this.setData({ answer: "", rawResponse: "", displayAnswer: "", loading: true });
    try {
      const sessionId = await this.ensureSession();
      const settings = getSettings();
      const response = await knowledgeChat(
        sessionId,
        this.data.query.trim(),
        settings.selectedKnowledgeBaseId
      );
      const rawResponse = typeof response === "string" ? response : JSON.stringify(response);
      const answer = collectAnswerFromSSE(rawResponse);
      const displayAnswer = answer || rawResponse;
      this.setData({
        answer: answer,
        rawResponse: answer ? "" : rawResponse,
        displayAnswer: displayAnswer
      });
    } catch (error) {
      wx.showModal({
        title: t("chatFailed"),
        content: (error && error.message) || "",
        showCancel: false
      });
    } finally {
      this.setData({ loading: false });
    }
  }
});
