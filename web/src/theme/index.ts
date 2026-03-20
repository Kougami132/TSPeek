import {
  type BrandVariants,
  createLightTheme,
  type Theme,
} from '@fluentui/react-components'

// 基于原 MVP 暖色调 (#f5efe6 / #0f766e) 定制的品牌色板
const tsPeekBrand: BrandVariants = {
  10: '#021211',
  20: '#042723',
  30: '#063B34',
  40: '#085045',
  50: '#0A6456',
  60: '#0D7968',
  70: '#0F8D79',
  80: '#0F766E', // 主色
  90: '#2B9E8E',
  100: '#47B09E',
  110: '#63C2AE',
  120: '#7FD4BE',
  130: '#9BE5CE',
  140: '#B7F0DE',
  150: '#D3F7EE',
  160: '#EFFCF8',
}

const baseLightTheme = createLightTheme(tsPeekBrand)

export const tsPeekTheme: Theme = {
  ...baseLightTheme,
  // 保留暖色调背景
  colorNeutralBackground1: '#FFFBF5',
  colorNeutralBackground2: '#F5EFE6',
  colorNeutralBackground3: '#EADFCF',
  colorNeutralForeground1: '#2B2118',
  colorNeutralForeground2: '#6D5A47',
}
