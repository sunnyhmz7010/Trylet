export type Language = 'c' | 'cpp' | 'python'
export const languageNames: Record<string, string> = {
  auto: '自动识别',
  c: 'C',
  cpp: 'C++',
  python: 'Python',
}
export const extensions: Record<Language, string> = { c: '.c', cpp: '.cpp', python: '.py' }
export const templates: Record<Language, string> = {
  c: '#include <stdio.h>\n\nint main(void) {\n    printf("Hello, world!\\n");\n    return 0;\n}\n',
  cpp: '#include <iostream>\n\nint main() {\n    std::cout << "Hello, world!" << std::endl;\n    return 0;\n}\n',
  python: 'print("Hello, world!")\n',
}

export function detectLanguage(code: string): Language | null {
  const text = code
    .replace(/"""[\s\S]*?"""|'''[\s\S]*?'''|"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'/g, '""')
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/\/\/[^\n]*/g, '')
  if (
    /\bstd::|#\s*include\s*<(?:iostream|vector|bits\/stdc\+\+\.h|string)>|\busing\s+namespace\s|\btemplate\s*</.test(
      text,
    )
  )
    return 'cpp'
  if (
    /^\s*(?:def\s+\w+|from\s+\w+\s+import|import\s+\w+|(?:if|for|while|class)\b[^\n]*:)\s*/m.test(
      text,
    )
  )
    return 'python'
  if (/^\s*print\s*\(/m.test(text) && !/#\s*include|\bint\s+main\s*\(/.test(text)) return 'python'
  if (
    /#\s*include\s*<(?:stdio|stdlib|stdint|stdbool|string)\.h>|\bint\s+main\s*\(\s*void\s*\)/.test(
      text,
    )
  )
    return 'c'
  return null
}

export function resolvedLanguage(language: string, code: string): Language | null {
  return language === 'auto'
    ? detectLanguage(code)
    : language in extensions
      ? (language as Language)
      : null
}

export function sessionID() {
  return Array.from(crypto.getRandomValues(new Uint8Array(12)), (n) =>
    n.toString(16).padStart(2, '0'),
  ).join('')
}
export function localDate() {
  const d = new Date()
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}
export function timeSuffix() {
  return new Date().toTimeString().slice(0, 8).replaceAll(':', '')
}
export function errorText(error: unknown) {
  return error instanceof Error ? error.message : String(error)
}
