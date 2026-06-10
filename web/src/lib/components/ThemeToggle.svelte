<script lang="ts">
  import { onMount } from 'svelte'
  import Moon from '@lucide/svelte/icons/moon'
  import Sun from '@lucide/svelte/icons/sun'

  type Theme = 'light' | 'dark'

  let theme: Theme = 'light'

  function systemTheme(): Theme {
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  }

  function toggle() {
    theme = theme === 'dark' ? 'light' : 'dark'
    document.documentElement.setAttribute('data-theme', theme)
    try {
      localStorage.setItem('theme', theme)
    } catch {}
  }

  onMount(() => {
    const current = document.documentElement.getAttribute('data-theme')
    if (current === 'light' || current === 'dark') {
      theme = current
      return
    }
    // Inline script didn't run (e.g. HMR): resolve and apply now.
    let stored: string | null = null
    try {
      stored = localStorage.getItem('theme')
    } catch {}
    theme = stored === 'light' || stored === 'dark' ? stored : systemTheme()
    document.documentElement.setAttribute('data-theme', theme)
  })
</script>

<button
  type="button"
  class="icon ghost small"
  onclick={toggle}
  aria-label="Switch to {theme === 'dark' ? 'light' : 'dark'} theme"
  data-tooltip="Switch to {theme === 'dark' ? 'light' : 'dark'} theme"
  data-tooltip-placement="right"
>
  {#if theme === 'dark'}
    <Moon size={16} aria-hidden="true" />
  {:else}
    <Sun size={16} aria-hidden="true" />
  {/if}
</button>
