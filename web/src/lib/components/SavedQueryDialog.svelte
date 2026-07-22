<script lang="ts">
  import { createSavedQuery, updateSavedQuery, type SavedQuery, type SavedQueryPayload } from '$lib/api'
  import ErrorMessage from './ErrorMessage.svelte'

  let {
    open = $bindable(false),
    existing = null,
    query = '',
    isPublic = false,
    hostIds = [],
    groupIds = [],
    platforms = [],
    onClose = () => {},
    onSaved = (_saved: SavedQuery) => {}
  }: {
    open?: boolean
    existing?: SavedQuery | null
    query?: string
    isPublic?: boolean
    hostIds?: string[]
    groupIds?: string[]
    platforms?: string[]
    onClose?: () => void
    onSaved?: (saved: SavedQuery) => void
  } = $props()

  let dialog = $state<HTMLDialogElement>()
  let preparedFor = $state<string | null>(null)
  let name = $state('')
  let description = $state('')
  let error = $state('')
  let isSubmitting = $state(false)

  $effect(() => {
    if (!open || !dialog) return
    const key = existing?.id || 'new'
    if (preparedFor !== key) {
      name = existing?.name || ''
      description = existing?.description || ''
      error = ''
      isSubmitting = false
      preparedFor = key
    }
    if (!dialog.open) dialog.showModal()
  })

  $effect(() => {
    if (open || !dialog) return
    preparedFor = null
    if (dialog.open) dialog.close()
  })

  function payload(): SavedQueryPayload {
    return {
      name,
      description,
      query,
      visibility: isPublic ? 'public' : 'private',
      host_ids: hostIds,
      group_ids: groupIds,
      platforms
    }
  }

  async function submit(asNew: boolean) {
    isSubmitting = true
    error = ''
    try {
      const saved =
        existing && !asNew ? await updateSavedQuery(existing.id, payload()) : await createSavedQuery(payload())
      onSaved(saved)
      dialog?.close()
    } catch (err) {
      error = (err as Error).message || 'Failed to save query'
    } finally {
      isSubmitting = false
    }
  }
</script>

<dialog bind:this={dialog} onclose={onClose} closedby="any">
  <form onsubmit={(e) => { e.preventDefault(); submit(false) }}>
    <header>
      <h2>{existing ? 'Update Saved Query' : 'Save Query'}</h2>
    </header>

    <ErrorMessage message={error} onClose={() => (error = '')} />

    <div class="vstack">
      <label data-field>
        Name <span class="req" aria-hidden="true">*</span>
        <input bind:value={name} required placeholder="Saved query name" />
      </label>

      <label data-field>
        Description
        <textarea bind:value={description} rows="3" placeholder="What this query is for"></textarea>
      </label>
    </div>

    <footer>
      <button type="button" class="outline" onclick={() => dialog?.close()}>Cancel</button>
      {#if existing}
        <button
          type="button"
          class="outline"
          disabled={isSubmitting || !name.trim()}
          onclick={() => submit(true)}
        >
          Save as new
        </button>
      {/if}
      <button
        type="submit"
        disabled={isSubmitting || !name.trim()}
        aria-busy={isSubmitting ? 'true' : undefined}
        data-spinner="small"
      >
        {existing ? 'Update' : 'Save'}
      </button>
    </footer>
  </form>
</dialog>
