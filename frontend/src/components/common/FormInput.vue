<script lang="ts">
import { defineComponent, h, vModelDynamic, withDirectives, type PropType } from 'vue'

export default defineComponent({
  inheritAttrs: false,
  props: {
    modelValue: null,
    modelModifiers: Object as PropType<{ lazy?: boolean; number?: boolean; trim?: boolean }>
  },
  setup(props, { attrs }) {
    return () => {
      const choice = attrs.type === 'checkbox' || attrs.type === 'radio'
      const appearance = choice
        ? attrs.role === 'switch' ? 'form-switch' : 'form-choice'
        : 'input'
      const input = h('input', { ...attrs, class: [appearance, attrs.class] })
      return 'onUpdate:modelValue' in attrs
        ? withDirectives(input, [[vModelDynamic, props.modelValue, undefined, props.modelModifiers ?? {}]])
        : input
    }
  }
})
</script>
