import { describe, expect, it } from 'vitest'
import { parse } from 'vue/compiler-sfc'
import source from '@/views/admin/SettingsView.vue?raw'
import smartOpsSource from '@/components/settings/SmartOpsSidebarSettings.vue?raw'
import bpsImageSource from '@/components/admin/settings/ExcelBPSImageSettings.vue?raw'
import { SETTINGS_SECTIONS, settingsLocation } from '../settingsLocation'

describe('settings navigation locations', () => {
  it('keeps every settings heading in its actual tab with a unique anchor', () => {
    const root = parse(source).descriptor.template!.ast!
    type Node = typeof root.children[number]
    const anchors = new Map<string, string>()
    const sectionSources: Record<string, string> = {
      SmartOpsSidebarSettings: smartOpsSource,
      ExcelBPSImageSettings: bpsImageSource,
    }
    function visit(node: Node, tab = '') {
      if (node.type !== 1) return
      const show = node.props.find(prop => prop.type === 7 && (prop.name === 'show' || prop.name === 'if'))
      if (show?.type === 7 && show.exp?.type === 4) tab = show.exp.content.match(/activeTab === '([^']+)'/)?.[1] ?? tab
      const id = node.props.find(prop => prop.type === 6 && prop.name === 'id')
      if (id?.type === 6 && id.value?.content.startsWith('settings-section-')) {
        expect(anchors.has(id.value.content)).toBe(false)
        anchors.set(id.value.content, tab)
      }
      const sectionSource = sectionSources[node.tag]
      if (sectionSource) {
        const childRoot = parse(sectionSource).descriptor.template!.ast!
        for (const child of childRoot.children) visit(child, tab)
      }
      for (const child of node.children) visit(child, tab)
    }
    for (const child of root.children) visit(child)
    expect(anchors.size).toBe(SETTINGS_SECTIONS.length)
    for (const section of SETTINGS_SECTIONS) expect(anchors.get(`settings-section-${section.id}`), section.id).toBe(section.tab)
  })

  it('validates locations and prefers the target card tab over a conflicting query', () => {
    expect(settingsLocation('security', '#settings-section-features-risk-control').tab).toBe('features')
    expect(settingsLocation(['features'], '#unknown')).toEqual({ tab: 'general', anchor: undefined, linked: false })
    expect(settingsLocation('email', '')).toEqual({ tab: 'email', anchor: undefined, linked: true })
  })
})
