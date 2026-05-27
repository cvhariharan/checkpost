<script lang="ts">
  import { onMount } from 'svelte'

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
    <svg
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="1.75"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      <path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9Z" />
    </svg>
  {:else}
    <svg
      width="16"
      height="16"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="1.75"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2" />
      <path d="M12 20v2" />
      <path d="m4.93 4.93 1.41 1.41" />
      <path d="m17.66 17.66 1.41 1.41" />
      <path d="M2 12h2" />
      <path d="M20 12h2" />
      <path d="m6.34 17.66-1.41 1.41" />
      <path d="m19.07 4.93-1.41 1.41" />
    </svg>
  {/if}
</button>
