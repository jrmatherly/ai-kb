// Mirrors types.Channel* in internal/types/knowledge.go plus the connector
// types documents carry as a channel; channelLabel.test.ts fails when they drift.
export const UPLOAD_CHANNELS: ReadonlySet<string> = new Set(['web', 'api', 'browser_extension']);

export const CHANNEL_LABEL_KEYS: Record<string, string> = {
  web: 'knowledgeBase.channelWeb',
  api: 'knowledgeBase.channelApi',
  browser_extension: 'knowledgeBase.channelBrowserExtension',
  feishu: 'knowledgeBase.channelFeishu',
  feishu_drive: 'knowledgeBase.channelFeishuDrive',
  lark_drive: 'knowledgeBase.channelLarkDrive',
  notion: 'knowledgeBase.channelNotion',
  yuque: 'knowledgeBase.channelYuque',
  confluence: 'knowledgeBase.channelConfluence',
  gitlab: 'knowledgeBase.channelGitLab',
  github: 'knowledgeBase.channelGitHub',
  bitbucket: 'knowledgeBase.channelBitbucket',
  seafile: 'knowledgeBase.channelSeafile',
  ima: 'knowledgeBase.channelIma',
  rss: 'knowledgeBase.channelRss',
  wechat: 'knowledgeBase.channelWechat',
  wecom: 'knowledgeBase.channelWecom',
  dingtalk: 'knowledgeBase.channelDingtalk',
  slack: 'knowledgeBase.channelSlack',
  im: 'knowledgeBase.channelIm',
};

export function channelLabelKey(channel: string | undefined): string {
  return channel && Object.hasOwn(CHANNEL_LABEL_KEYS, channel)
    ? CHANNEL_LABEL_KEYS[channel]
    : 'knowledgeBase.channelUnknown';
}

export function sourceFilterChannels(): string[] {
  return Object.keys(CHANNEL_LABEL_KEYS).filter((channel) => !UPLOAD_CHANNELS.has(channel));
}
