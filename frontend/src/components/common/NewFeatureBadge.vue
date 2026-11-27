<script lang="ts">
import { defineComponent, type PropType } from 'vue'

// Upstream templates can still reference the retired badge.
export default defineComponent({
  props: {
    noticeId: String,
    userId: Number as PropType<number | null>,
    compact: Boolean,
  },
  render: () => null,
})
</script>
