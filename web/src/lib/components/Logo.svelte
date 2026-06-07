<script lang="ts">
  type LogoSize = 'sm' | 'md' | 'lg'

  let {
    size = 'md',
    class: className = ''
  }: {
    size?: LogoSize
    class?: string
  } = $props()

  const classes = $derived(
    ['checkpost-logo', `checkpost-logo-${size}`, className].filter(Boolean).join(' ')
  )
</script>

<span class={classes}>
  <svg
    class="checkpost-logo-mark"
    width="256"
    height="256"
    viewBox="0 0 256 256"
    fill="none"
    aria-hidden="true"
  >
    <rect x="32" y="32" width="192" height="192" rx="28" class="checkpost-logo-bg" />
    <circle cx="128" cy="128" r="54" class="checkpost-logo-ring" />
  </svg>
  <span class="checkpost-logo-title">checkpost</span>
</span>

<style>
  .checkpost-logo {
    --logo-gap: var(--space-2);
    --logo-mark-size: 2rem;
    --logo-title-size: 1.8rem;
    --logo-mark-offset: 0.06rem;
    --logo-bg: #1a2e2a;
    --logo-ring: #a0c461;

    display: inline-flex;
    align-items: center;
    gap: var(--logo-gap);
    min-width: 0;
    line-height: 1;
    color: var(--foreground);
  }
  :global(:root[data-theme='dark']) .checkpost-logo {
    --logo-bg: #a0c461;
    --logo-ring: #1a2e2a;
  }
  .checkpost-logo-sm {
    --logo-mark-size: 1.5rem;
    --logo-title-size: 1.45rem;
    --logo-mark-offset: 0.04rem;
  }
  .checkpost-logo-md {
    --logo-mark-size: 2rem;
    --logo-title-size: 1.8rem;
  }
  .checkpost-logo-lg {
    --logo-gap: var(--space-3);
    --logo-mark-size: 2.75rem;
    --logo-title-size: 2.25rem;
    --logo-mark-offset: 0.08rem;
  }
  .checkpost-logo-mark {
    display: block;
    width: var(--logo-mark-size);
    height: var(--logo-mark-size);
    flex: 0 0 auto;
    transform: translateY(var(--logo-mark-offset));
  }
  .checkpost-logo-bg {
    fill: var(--logo-bg);
  }
  .checkpost-logo-ring {
    fill: none;
    stroke: var(--logo-ring);
    stroke-width: 16;
  }
  .checkpost-logo-title {
    display: block;
    font-family: 'Libre Baskerville', Georgia, 'Times New Roman', serif;
    font-size: var(--logo-title-size);
    font-weight: 600;
    line-height: 0.9;
    white-space: nowrap;
  }
</style>
