import { createContext, useContext } from 'react'
import type { Settings } from '../bindings/trylet/internal/trylet/models'

// 只提供已保存设置；设置页的未提交表单不会改变其他页面。
export const SettingsContext = createContext<Settings | null>(null)
export const useSettings = () => useContext(SettingsContext)
