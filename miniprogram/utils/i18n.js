const MESSAGES = {
  tabKnowledge: "Knowledge",
  tabChat: "Chat",
  tabSettings: "Settings",
  navKnowledge: "Knowledge",
  navChat: "Chat",
  navSettings: "Settings",
  knowledgeTitle: "AI KB Knowledge",
  knowledgeSubtitle: "Import a web page into a selected knowledge base from WeChat.",
  needsSettings: "Configure the AI KB API base URL and API key before loading knowledge bases.",
  openSettings: "Open Settings",
  knowledgeBase: "Knowledge Base",
  tapToSelect: "Tap to select",
  refreshKnowledgeBases: "Refresh knowledge bases",
  urlLabel: "URL",
  urlPlaceholder: "https://github.com/jrmatherly/ai-kb",
  importUrl: "Import URL",
  loadedKnowledgeBases: "Loaded {count} knowledge bases.",
  noKnowledgeBases: "No knowledge bases found.",
  loadFailed: "Load failed",
  imported: "Imported",
  importFailed: "Import failed",
  chatTitle: "Knowledge Chat",
  chatSubtitle: "Ask the selected AI KB knowledge base.",
  question: "Question",
  questionPlaceholder: "Ask something...",
  askAIKB: "Ask AI KB",
  answer: "Answer",
  chatFailed: "Chat failed",
  sessionIdMissing: "The session API did not return a session id.",
  settingsTitle: "Connect AI KB",
  settingsSubtitle: "Use your AI KB API endpoint and tenant API key.",
  apiBaseUrl: "API Base URL",
  apiBaseUrlPlaceholder: "https://your-ai-kb.example.com",
  apiKey: "API Key",
  apiKeyPlaceholder: "sk-...",
  saveSettings: "Save settings",
  saved: "Saved",
  missingBaseUrl: "Please configure the AI KB API base URL first.",
  missingApiKey: "Please configure the AI KB API key first."
};

function t(key, vars = {}) {
  let text = MESSAGES[key] || key;
  Object.keys(vars).forEach((name) => {
    text = text.replace(new RegExp(`\\{${name}\\}`, "g"), String(vars[name]));
  });
  return text;
}

function getMessages() {
  return MESSAGES;
}

module.exports = {
  MESSAGES,
  getMessages,
  t
};
