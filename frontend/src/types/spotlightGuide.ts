export type GuidePlacement = 'right' | 'left' | 'bottom' | 'top'

export interface SpotlightGuideStep {
  key: string
  /** CSS selector of the highlighted target; omitted means a centered card */
  target?: string
  placement?: GuidePlacement
  before?: () => void | Promise<void>
  /** Whether to skip this step when the target does not exist */
  optional?: boolean
  /** When true, the guide has the user click the highlighted area directly and shows no "Next" */
  interact?: boolean
}
