import { describe, expect, it } from 'vitest'
import { detectLanguage, resolvedLanguage } from './language'

describe('单文件语言识别', () => {
  it('识别课程常见 C、C++ 和 Python 源码', () => {
    expect(detectLanguage('#include <stdio.h>\nint main(void) { return 0; }')).toBe('c')
    expect(detectLanguage('#include <iostream>\nint main() { std::cout << 1; }')).toBe('cpp')
    expect(detectLanguage('print("你好")')).toBe('python')
    expect(detectLanguage('n = input("数字：")\nfor i in range(3):\n    print(i)')).toBe('python')
  })
  it('C/C++ 通用语法和空白不自动执行', () => {
    expect(detectLanguage('int main() { return 0; }')).toBeNull()
    expect(detectLanguage('')).toBeNull()
  })
  it('字符串中的其他语言片段不会改变识别结果', () => {
    expect(detectLanguage('print("#include <iostream> std::cout")')).toBe('python')
  })
  it('手动选择优先于自动识别', () => {
    expect(resolvedLanguage('cpp', '#include <stdio.h>\nint main(void) { return 0; }')).toBe('cpp')
  })
})
