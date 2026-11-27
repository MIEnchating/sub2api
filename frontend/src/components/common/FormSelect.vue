<script lang="ts">
import { defineComponent, h, vModelSelect, withDirectives, type PropType } from 'vue'

export default defineComponent({
  inheritAttrs: false,
  props: {
    modelValue: null,
    modelModifiers: Object as PropType<{ number?: boolean }>
  },
  setup(props, { attrs, slots }) {
    return () => {
      const select = h('select', { ...attrs, class: ['input select-control form-select', attrs.class] }, slots.default?.())

      // Delegate to Vue's native select binding, including typed option values and .number.
      if ('onUpdate:modelValue' in attrs) {
        return withDirectives(select, [[vModelSelect, props.modelValue, undefined, props.modelModifiers ?? {}]])
      }
      return select
    }
  }
})
</script>
