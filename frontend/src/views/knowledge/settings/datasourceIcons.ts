import feishuIcon from '@/assets/img/datasource-feishu.ico'
import githubIcon from '@/assets/img/datasource-github.svg'
import gitlabIcon from '@/assets/img/datasource-gitlab.png'
import larkIcon from '@/assets/img/datasource-lark.svg'
import notionIcon from '@/assets/img/datasource-notion.ico'
import yuqueIcon from '@/assets/img/datasource-yuque.ico'
import rssIcon from '@/assets/img/datasource-rss.svg'
import confluenceIcon from '@/assets/img/datasource-confluence.svg'
import dingtalkIcon from '@/assets/img/im/dingtalk.svg'
import imaIcon from '@/assets/img/datasource-ima.png'
import seafileIcon from '@/assets/img/datasource-seafile.png'

export const datasourceIconMap: Record<string, string> = {
  feishu: feishuIcon,
  lark: larkIcon,
  // Drive (cloud drive) connectors reuse the wiki icons - same product, same brand.
  feishu_drive: feishuIcon,
  lark_drive: larkIcon,
  notion: notionIcon,
  yuque: yuqueIcon,
  dingtalk: dingtalkIcon,
  rss: rssIcon,
  confluence: confluenceIcon,
  gitlab: gitlabIcon,
  github: githubIcon,
  ima: imaIcon,
  seafile: seafileIcon,
}

export function getDatasourceIconUrl(type: string): string | undefined {
  return datasourceIconMap[type]
}
