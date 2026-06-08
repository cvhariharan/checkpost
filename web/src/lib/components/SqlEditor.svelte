<script lang="ts">
  import { onMount } from 'svelte'
  import { fetchOsquerySchema } from '$lib/api'
  import type { SqlEditorHandle } from '$lib/sqlEditor'

  let {
    value = $bindable(''),
    placeholder = 'SELECT * FROM ...',
    minLines = 4,
    maxLines = 20,
    disabled = false,
    lineNumbers = false,
    id = undefined,
    ariaLabel = 'SQL query editor',
    onsubmit = undefined
  }: {
    value?: string
    placeholder?: string
    minLines?: number
    maxLines?: number
    disabled?: boolean
    lineNumbers?: boolean
    id?: string
    ariaLabel?: string
    onsubmit?: () => void
  } = $props()

  let host = $state<HTMLDivElement>()
  let handle = $state<SqlEditorHandle | null>(null)

  onMount(() => {
    let destroyed = false
    // Lazy-load CodeMirror (browser-only, code-split); the textarea shows until ready.
    import('$lib/sqlEditor')
      .then(({ createSqlEditor }) => {
        if (destroyed || !host) return
        handle = createSqlEditor({
          parent: host,
          doc: value,
          placeholder,
          lineNumbers,
          disabled,
          minHeight: `${minLines * 1.5}em`,
          maxHeight: `${maxLines * 1.5}em`,
          onChange: (v) => (value = v),
          onSubmit: onsubmit
        })
        // Memoized fetch shared across editors; completion upgrades once it loads.
        fetchOsquerySchema()
          .then((schema) => !destroyed && handle?.setSchema(schema))
          .catch(() => {})
      })
      .catch(() => {})

    return () => {
      destroyed = true
      handle?.destroy()
      handle = null
    }
  })

  // Push external value changes (loading a record for edit) into the editor,
  // guarding against the loop from our own onChange.
  $effect(() => {
    const v = value
    if (handle && v !== handle.getValue()) handle.setValue(v)
  })

  $effect(() => {
    handle?.setDisabled(disabled)
  })
</script>

<div class="sqleditor">
  <div bind:this={host} class="sqleditor-cm" class:sqleditor-hidden={!handle} {id}></div>
  {#if !handle}
    <textarea
      class="sqleditor-fallback"
      rows={minLines}
      {placeholder}
      {disabled}
      aria-label={ariaLabel}
      bind:value
    ></textarea>
  {/if}
</div>

<style>
  .sqleditor,
  .sqleditor-cm {
    width: 100%;
  }
  .sqleditor-hidden {
    display: none;
  }
  .sqleditor-fallback {
    width: 100%;
    font-family: var(--font-mono, ui-monospace, 'JetBrains Mono', monospace);
  }
</style>
