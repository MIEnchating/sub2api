<script lang="ts">
import { defineComponent, h, vModelText, withDirectives, type PropType } from 'vue'

export default defineComponent({
  inheritAttrs: false,
  props: {
    modelValue: null,
    modelModifiers: Object as PropType<{ lazy?: boolean; trim?: boolean }>
  },
  setup(props, { attrs, slots }) {
    return () => {
      const textarea = h('textarea', { ...attrs, class: ['input', attrs.class] }, slots.default?.())
      return 'onUpdate:modelValue' in attrs
        ? withDirectives(textarea, [[vModelText, props.modelValue, undefined, props.modelModifiers ?? {}]])
        : textarea
    }
  }
})
</script>
