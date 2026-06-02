<script lang="ts">
  import { createAlertTarget, updateAlertTarget, type AlertTarget } from '$lib/api'
  import ErrorMessage from './ErrorMessage.svelte'

  let {
    open = false,
    target = null,
    onClose = () => {},
    onSaved = () => {}
  }: {
    open?: boolean
    target?: AlertTarget | null
    onClose?: () => void
    onSaved?: () => void
  } = $props()

  let dialog = $state<HTMLDialogElement>()
  let preparedFor = $state<string | null>(null)
  let name = $state('')
  let targetType = $state<'smtp' | 'webhook'>('smtp')
  let enabled = $state(true)
  let recipients = $state('')
  let webhookUrl = $state('')
  let error = $state('')
  let isSubmitting = $state(false)

  const isEdit = $derived(!!target?.uuid)

  $effect(() => {
    if (!open || !dialog) return
    const key = target?.uuid || 'new'
    if (preparedFor !== key) {
      loadForm(target)
      preparedFor = key
    }
    if (!dialog.open) dialog.showModal()
  })

  $effect(() => {
    if (open || !dialog) return
    preparedFor = null
    if (dialog.open) dialog.close()
  })

  function loadForm(record: AlertTarget | null) {
    name = record?.name || ''
    targetType = record?.type === 'webhook' ? 'webhook' : 'smtp'
    enabled = record?.enabled ?? true
    const cfg = (record?.config || {}) as Record<string, unknown>
    recipients = Array.isArray(cfg.recipients) ? (cfg.recipients as string[]).join('\n') : ''
    webhookUrl = typeof cfg.url === 'string' ? cfg.url : ''
    error = ''
    isSubmitting = false
  }

  function buildConfig(): Record<string, unknown> {
    if (targetType === 'webhook') {
      return { url: webhookUrl.trim() }
    }
    return {
      recipients: recipients
        .split('\n')
        .map((r) => r.trim())
        .filter(Boolean)
    }
  }

  async function save(event: SubmitEvent) {
    event.preventDefault()
    isSubmitting = true
    error = ''
    try {
      const payload: Record<string, unknown> = { name, config: buildConfig(), enabled }
      if (isEdit && target) {
        await updateAlertTarget(target.uuid, payload)
      } else {
        payload.type = targetType
        await createAlertTarget(payload)
      }
      onSaved()
      dialog?.close()
    } catch (err) {
      error = (err as Error).message || 'Failed to save target'
    } finally {
      isSubmitting = false
    }
  }
</script>

<dialog bind:this={dialog} onclose={onClose} closedby="any">
  <form onsubmit={save}>
    <header>
      <h2>{isEdit ? 'Edit Target' : 'Create Target'}</h2>
    </header>

    <ErrorMessage message={error} onClose={() => (error = '')} />

    <div class="vstack">
      <label data-field>
        Name
        <input bind:value={name} required placeholder="Target name" />
      </label>

      {#if isEdit}
        <label data-field>
          Type
          <input value={targetType === 'webhook' ? 'Webhook' : 'Email'} disabled />
        </label>
      {:else}
        <label data-field>
          Type
          <select bind:value={targetType}>
            <option value="smtp">Email</option>
            <option value="webhook">Webhook</option>
          </select>
        </label>
      {/if}

      {#if targetType === 'webhook'}
        <label data-field>
          Webhook URL
          <input bind:value={webhookUrl} required type="url" placeholder="https://example.com/checkpost-alerts" />
        </label>
      {:else}
        <label data-field>
          Recipients
          <textarea
            bind:value={recipients}
            rows="4"
            placeholder="One per line: user@example.com, owner, user-group:&lt;name&gt;"
          ></textarea>
        </label>
      {/if}

      <label data-field class="hstack gap-2">
        <input type="checkbox" bind:checked={enabled} />
        Enabled
      </label>
    </div>

    <footer>
      <button type="button" class="outline" onclick={() => dialog?.close()}>Cancel</button>
      <button type="submit" disabled={isSubmitting} aria-busy={isSubmitting ? 'true' : undefined}>
        {isSubmitting ? 'Saving...' : isEdit ? 'Update' : 'Create'}
      </button>
    </footer>
  </form>
</dialog>
