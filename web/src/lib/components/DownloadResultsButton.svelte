<script lang="ts">
  import Download from '@lucide/svelte/icons/download'

  let {
    href,
    label = 'Download CSV',
    disabled = false
  }: {
    href: string
    label?: string
    disabled?: boolean
  } = $props()

  function handleClick(event: MouseEvent) {
    if (disabled) event.preventDefault()
  }
</script>

<a
  class="button outline small gap-1"
  href={disabled ? undefined : href}
  download
  aria-disabled={disabled ? 'true' : undefined}
  tabindex={disabled ? -1 : 0}
  onclick={handleClick}
>
  <Download size={14} aria-hidden="true" />
  {label}
</a>
