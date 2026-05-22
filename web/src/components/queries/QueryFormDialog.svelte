<script>
  import { createQuery, updateQuery } from '@/api.js'
  import ErrorMessage from '@/components/common/ErrorMessage.svelte'

  export let open = false
  export let queryRecord = null
  export let onClose = () => {}
  export let onSaved = () => {}

  let dialog
  let preparedFor = null
  let title = ''
  let sql = ''
  let description = ''
  let error = ''
  let isSubmitting = false

  $: if (open && dialog) {
    const key = queryRecord?.uuid || 'new'
    if (preparedFor !== key) {
      loadForm(queryRecord)
      preparedFor = key
    }
    if (!dialog.open) dialog.showModal()
  }

  $: if (!open && dialog) {
    preparedFor = null
    if (dialog.open) dialog.close()
  }

  function loadForm(record) {
    title = record?.title || ''
    sql = record?.query || ''
    description = record?.description || ''
    error = ''
    isSubmitting = false
  }

  function handleClose() {
    onClose()
  }

  async function saveQuery(event) {
    event.preventDefault()
    isSubmitting = true
    error = ''
    try {
      const payload = { title, query: sql, description }
      if (queryRecord?.uuid) {
        await updateQuery(queryRecord.uuid, payload)
      } else {
        await createQuery(payload)
      }
      onSaved()
      dialog.close()
    } catch (err) {
      error = err.message || 'Failed to save query'
    } finally {
      isSubmitting = false
    }
  }
</script>

<dialog bind:this={dialog} onclose={handleClose} closedby="any">
  <form onsubmit={saveQuery}>
    <header>
      <h2>{queryRecord ? 'Edit Query' : 'Create Query'}</h2>
    </header>

    <ErrorMessage message={error} onClose={() => (error = '')} />

    <div class="vstack">
      <label>
        Title
        <input bind:value={title} required placeholder="Enter a title for this query" />
      </label>

      <label>
        SQL Query
        <textarea bind:value={sql} required rows="6" placeholder="SELECT * FROM processes;"></textarea>
      </label>

      <label>
        Description
        <textarea bind:value={description} rows="3" placeholder="Enter a description for this query"></textarea>
      </label>
    </div>

    <footer>
      <button type="button" class="outline" onclick={() => dialog.close()}>
        Cancel
      </button>
      <button type="submit" disabled={isSubmitting}>
        {isSubmitting ? (queryRecord ? 'Updating...' : 'Creating...') : (queryRecord ? 'Update' : 'Create')}
      </button>
    </footer>
  </form>
</dialog>
