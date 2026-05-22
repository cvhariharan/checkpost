<script>
  import { createPolicy, updatePolicy } from '@/api.js'
  import ErrorMessage from '@/components/common/ErrorMessage.svelte'

  export let open = false
  export let policy = null
  export let onClose = () => {}
  export let onSaved = () => {}

  let dialog
  let preparedFor = null
  let title = ''
  let query = ''
  let description = ''
  let resolution = ''
  let platform = 'all'
  let enabled = true
  let error = ''
  let isSubmitting = false

  $: if (open && dialog) {
    const key = policy?.uuid || 'new'
    if (preparedFor !== key) {
      loadForm(policy)
      preparedFor = key
    }
    if (!dialog.open) dialog.showModal()
  }

  $: if (!open && dialog) {
    preparedFor = null
    if (dialog.open) dialog.close()
  }

  function loadForm(record) {
    title = record?.title || record?.name || ''
    query = record?.query || ''
    description = record?.description || ''
    resolution = record?.resolution || ''
    platform = record?.platform || 'all'
    enabled = record?.enabled ?? true
    error = ''
    isSubmitting = false
  }

  function handleClose() {
    onClose()
  }

  async function savePolicy(event) {
    event.preventDefault()
    isSubmitting = true
    error = ''
    try {
      const payload = { title, query, description, resolution, platform, enabled }
      if (policy?.uuid) {
        await updatePolicy(policy.uuid, payload)
      } else {
        await createPolicy(payload)
      }
      onSaved()
      dialog.close()
    } catch (err) {
      error = err.message || 'Failed to save policy'
    } finally {
      isSubmitting = false
    }
  }
</script>

<dialog bind:this={dialog} onclose={handleClose} closedby="any">
  <form onsubmit={savePolicy}>
    <header>
      <h2>{policy ? 'Edit Policy' : 'Create Policy'}</h2>
    </header>

    <ErrorMessage message={error} onClose={() => (error = '')} />

    <div class="vstack">
      <label>
        Name
        <input bind:value={title} required placeholder="Policy name" />
      </label>

      <label>
        Query
        <textarea bind:value={query} required rows="7" placeholder="SELECT 1 FROM disk_encryption WHERE encrypted = 1;"></textarea>
      </label>
      <p class="text-light">The query should return a value of <code>1</code> when the host passes the check and <code>0</code> (or no rows) when it fails.</p>

      <label>
        Description
        <textarea bind:value={description} rows="3" placeholder="What this policy checks"></textarea>
      </label>

      <label>
        Resolution
        <textarea bind:value={resolution} rows="3" placeholder="How to resolve failing machines"></textarea>
      </label>

      <label>
        Platform
        <select bind:value={platform}>
          <option value="all">All</option>
          <option value="any">Any</option>
          <option value="posix">POSIX</option>
          <option value="darwin">macOS</option>
          <option value="linux">Linux</option>
          <option value="windows">Windows</option>
        </select>
      </label>

      <label>
        <input type="checkbox" bind:checked={enabled} />
        Enabled
      </label>
    </div>

    <footer>
      <button type="button" class="outline" onclick={() => dialog.close()}>Cancel</button>
      <button type="submit" disabled={isSubmitting}>
        {isSubmitting ? (policy ? 'Updating...' : 'Creating...') : (policy ? 'Update' : 'Create')}
      </button>
    </footer>
  </form>
</dialog>
