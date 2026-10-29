import { describe, expect, it } from 'vitest'
import { createI18n } from 'vue-i18n'

// Exercise the same runtime bundle as Vite, including its patched shared dependency.
function translate(message: string, values: Record<string, string> = {}) {
  const i18n = createI18n({
    legacy: false,
    locale: 'en',
    escapeParameter: true,
    warnHtmlMessage: false,
    messages: { en: { message } }
  })
  return i18n.global.t('message', values)
}

describe('translation HTML security (GHSA-83rm-6c8r-g728)', () => {
  it.each([
    '<iframe srcdoc="<b>untrusted</b>"></iframe>',
    "<iframe srcdoc='<b>untrusted</b>'></iframe>",
    '<iframe SrCdOc = "&lt;script&gt;alert(1)&lt;/script&gt;"></iframe>',
    '<iframe/srcdoc="<img src=x onerror=alert(1)>"></iframe>',
    '<iframe\nsrcdoc=&lt;script&gt;alert(1)&lt;/script&gt;></iframe>',
    '<iframe srcdoc="{content}"></iframe>'
  ])('neutralizes the nested document attribute: %s', (message) => {
    const container = document.createElement('div')
    container.innerHTML = translate(message, { content: '<img src=x onerror=alert(1)>' })
    // Entity escaping the value alone fails: the browser decodes it for srcdoc.
    expect(container.querySelector('iframe')).not.toBeNull()
    expect(container.querySelector('[srcdoc]')).toBeNull()
  })

  it('preserves ordinary rich translations and escapes interpolation', () => {
    const container = document.createElement('div')
    container.innerHTML = translate('<strong>{name}</strong><br>Next', { name: '<img src=x>' })
    expect(container.querySelector('strong')?.textContent).toBe('<img src=x>')
    expect(container.querySelector('img')).toBeNull()
    expect(container.querySelector('br')).not.toBeNull()
    expect(translate('Hello {name}', { name: '世界' })).toBe('Hello 世界')
  })
})
