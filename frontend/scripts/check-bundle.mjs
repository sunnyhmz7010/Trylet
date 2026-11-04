import { readFile, stat } from 'node:fs/promises'

const dist = new URL('../dist/', import.meta.url)
const manifest = JSON.parse(await readFile(new URL('.vite/manifest.json', dist), 'utf8'))
const entry = Object.keys(manifest).find((key) => manifest[key].isEntry && manifest[key].src === 'index.html')
const editor = Object.keys(manifest).find((key) => manifest[key].isDynamicEntry && manifest[key].name === 'Editor')
if (!entry || !editor) throw new Error('构建清单缺少主入口或编辑器模块，无法检查启动体积')

// 合计全部静态依赖，防止拆分文件后遗漏启动仍需加载的代码。
const size = async (roots) => {
  const files = new Set()
  const visit = (key) => {
    const chunk = manifest[key]
    if (!chunk) throw new Error(`构建清单缺少依赖：${key}`)
    if (files.has(chunk.file)) return
    files.add(chunk.file)
    for (const dependency of chunk.imports ?? []) visit(dependency)
  }
  roots.forEach(visit)
  const sizes = await Promise.all([...files].map((file) => stat(new URL(file, dist))))
  return sizes.reduce((total, file) => total + file.size, 0)
}

for (const { name, roots, limit } of [
  { name: '启动 JavaScript', roots: [entry], limit: 500_000 },
  { name: '编辑器核心及启动依赖', roots: [entry, editor], limit: 900_000 },
]) {
  const bytes = await size(roots)
  console.log(`${name}：${(bytes / 1000).toFixed(2)} kB / ${(limit / 1000).toFixed(0)} kB`)
  if (bytes > limit) throw new Error(`${name}超过体积限制，请检查是否将延迟模块引入启动依赖`)
}
