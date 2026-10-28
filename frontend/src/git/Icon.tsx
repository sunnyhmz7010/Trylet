import { GitBranch, Folder, RefreshCw, X, ArrowLeft, FileDiff, Check, Copy, Layers,
  ChevronDown, ChevronUp, ChevronRight, Search, ListFilter, Crosshair, type LucideIcon } from 'lucide-react'
const icons: Record<string, LucideIcon> = { branch:GitBranch, folder:Folder, refresh:RefreshCw,
  close:X, arrow:ArrowLeft, changes:FileDiff, check:Check, copy:Copy, stack:Layers,
  chevron:ChevronDown, up:ChevronUp, down:ChevronDown, right:ChevronRight, search:Search,
  filter:ListFilter, target:Crosshair }
export function Icon({ name, size = 16 }: { name:string; size?:number }) {
  const Glyph = icons[name] ?? GitBranch
  return <Glyph size={size} aria-hidden="true" />
}
