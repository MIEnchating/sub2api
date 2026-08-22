import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../VersionBadge.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('VersionBadge upstream layout', () => {
  it('uses a wider viewport-constrained dropdown', () => {
    expect(componentSource).toContain('w-[27rem]')
    expect(componentSource).toContain('max-w-[calc(100vw-5rem)]')
  })

  it('keeps upstream labels readable and stacks rows on narrow screens', () => {
    expect(componentSource).toContain('grid-cols-[7rem_minmax(0,1fr)]')
    expect(componentSource).toContain('max-[420px]:grid-cols-1')
    expect(componentSource).toContain('whitespace-nowrap text-xs font-medium')
  })
})
