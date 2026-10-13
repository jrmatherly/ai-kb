import { applyStreamingTailFade } from './chatMarkdownRenderer.ts'

const cases = [
  '<p>Great, that narrows it down a lot. Two more quick</p>',
  '<p>Great, that narrows it down a lot. <strong>Two</strong> more quick</p>\n',
  '<ol>\n<li>First item</li>\n<li>Second item still being generated</li>\n</ol>',
  '<p>Ends with a citation <span class="citation">doc</span></p>',
  '<p></p>',
  '',
]
for (const c of cases) {
  console.log('IN :', JSON.stringify(c))
  console.log('OUT:', JSON.stringify(applyStreamingTailFade(c)))
  console.log('')
}
