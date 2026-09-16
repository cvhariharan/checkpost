<script lang="ts">
  import type { Snippet } from 'svelte'

  let {
    lines = 1,
    text = undefined,
    tooltip = true,
    children
  }: {
    lines?: number
    text?: string | null
    tooltip?: boolean
    children?: Snippet
  } = $props()

  const hasText = $derived(text !== undefined && text !== null)
  const displayText = $derived(hasText ? String(text) : '')
  const tooltipId = $props.id()
  let trigger: HTMLSpanElement
  let popup: HTMLSpanElement | undefined

  function showTooltip() {
    if (!popup) return
    popup.showPopover()
    const anchor = trigger.getBoundingClientRect()
    const rect = popup.getBoundingClientRect()
    const left = Math.max(8, Math.min(anchor.left, window.innerWidth - rect.width - 8))
    const top = anchor.top >= rect.height + 8
      ? anchor.top - rect.height - 8
      : Math.min(anchor.bottom + 8, window.innerHeight - rect.height - 8)
    popup.style.left = `${left}px`
    popup.style.top = `${Math.max(8, top)}px`
  }

  function hideTooltip() {
    popup?.hidePopover()
  }
</script>

<svelte:window onscrollcapture={hideTooltip} onresize={hideTooltip} onkeydown={(event) => { if (event.key === 'Escape') hideTooltip() }} />

<span
  bind:this={trigger}
  class="truncate"
  style:--truncate-lines={lines}
  aria-describedby={tooltip && hasText ? tooltipId : undefined}
  onmouseenter={showTooltip}
  onmouseleave={hideTooltip}
  onfocusin={showTooltip}
  onfocusout={hideTooltip}
>
  {#if hasText}{displayText}{:else if children}{@render children()}{/if}
</span>
{#if tooltip && hasText}
  <span bind:this={popup} id={tooltipId} role="tooltip" popover="manual" class="truncate-tooltip">{displayText}</span>
{/if}

<style>
  .truncate {
    display: -webkit-box;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: var(--truncate-lines);
    line-clamp: var(--truncate-lines);
    overflow: hidden;
    overflow-wrap: anywhere;
    word-break: break-word;
  }

  .truncate-tooltip {
    position: fixed;
    inset: auto;
    margin: 0;
    max-width: min(60ch, calc(100vw - 16px));
    max-height: calc(100vh - 16px);
    padding: var(--space-2) var(--space-3);
    border: 0;
    border-radius: var(--radius-medium);
    background: var(--foreground);
    color: var(--background);
    font-size: var(--text-7);
    white-space: normal;
    overflow-wrap: anywhere;
    pointer-events: none;
  }
</style>
