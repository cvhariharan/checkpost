<script lang="ts">
  import { onMount } from 'svelte'
  import {
    fetchAPITokens,
    createAPIToken,
    revokeAPIToken,
    type APIToken,
    type IssuedAPIToken
  } from '$lib/api'
  import { formatTimestamp, toast } from '$lib/util'
  import ErrorMessage from '$lib/components/ErrorMessage.svelte'
  import Spinner from '$lib/components/Spinner.svelte'
  import ActionsMenu from '$lib/components/ActionsMenu.svelte'
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte'

  let tokens = $state<APIToken[]>([])
  let loading = $state(true)
  let error = $state('')

  // Create form.
  let createDialog = $state<HTMLDialogElement>()
  let name = $state('')
  let expiresInDays = $state(7)
  let saving = $state(false)
  let formError = $state('')

  // One-time secret reveal.
  let secretDialog = $state<HTMLDialogElement>()
  let issued = $state<IssuedAPIToken | null>(null)

  // Revoke confirmation.
  let revokeOpen = $state(false)
  let selected = $state<APIToken | null>(null)
  let revoking = $state(false)

  type TokenStatus = { label: string; variant: 'success' | 'danger' | 'warning' }

  function statusOf(token: APIToken): TokenStatus {
    if (token.revoked_at) return { label: 'Revoked', variant: 'danger' }
    if (token.expires_at && new Date(token.expires_at).getTime() < Date.now()) {
      return { label: 'Expired', variant: 'warning' }
    }
    return { label: 'Active', variant: 'success' }
  }

  onMount(() => {
    void load()
  })

  async function load() {
    loading = true
    error = ''
    try {
      tokens = (await fetchAPITokens()) || []
    } catch (err) {
      error = (err as Error).message || 'Failed to load tokens'
    } finally {
      loading = false
    }
  }

  function openCreate() {
    name = ''
    expiresInDays = 7
    saving = false
    formError = ''
    createDialog?.showModal()
  }

  async function save(event: SubmitEvent) {
    event.preventDefault()
    saving = true
    formError = ''
    try {
      const token = await createAPIToken({ name, expires_in_days: expiresInDays })
      createDialog?.close()
      issued = token
      secretDialog?.showModal()
      await load()
    } catch (err) {
      formError = (err as Error).message || 'Failed to create token'
    } finally {
      saving = false
    }
  }

  async function copySecret() {
    if (!issued) return
    try {
      await navigator.clipboard.writeText(issued.secret)
      toast('Token copied to clipboard', 'API Token', { variant: 'success' })
    } catch {
      toast('Could not copy token', 'Clipboard', { variant: 'danger' })
    }
  }

  function closeSecret() {
    secretDialog?.close()
    issued = null
  }

  function confirmRevoke(token: APIToken) {
    selected = token
    revokeOpen = true
  }

  async function doRevoke() {
    if (!selected) return
    revoking = true
    try {
      await revokeAPIToken(selected.uuid)
      revokeOpen = false
      selected = null
      await load()
    } catch (err) {
      error = (err as Error).message || 'Failed to revoke token'
    } finally {
      revoking = false
    }
  }
</script>

<section class="vstack gap-4">
  <header class="hstack justify-between mb-4">
    <div>
      <h1 class="mb-2">API Tokens</h1>
      <p class="text-light">
        Bearer tokens for the CLI and scripts. A token acts as you, with your permissions.
      </p>
    </div>
    <button type="button" onclick={openCreate}>Create token</button>
  </header>

  <ErrorMessage message={error} onClose={() => (error = '')} />

  {#if loading}
    <Spinner fill />
  {:else}
    <div class="table">
      <table>
        <thead>
          <tr>
            <th>Name</th>
            <th>Status</th>
            <th>Last used</th>
            <th>Expires</th>
            <th>Created</th>
            <th class="align-right"><span class="sr-only">Actions</span></th>
          </tr>
        </thead>
        <tbody>
          {#each tokens as token}
            {@const status = statusOf(token)}
            <tr>
              <td><strong>{token.name || 'Unnamed token'}</strong></td>
              <td><span class="badge" data-variant={status.variant}>{status.label}</span></td>
              <td>{token.last_used_at ? formatTimestamp(token.last_used_at) : 'Never'}</td>
              <td>{token.expires_at ? formatTimestamp(token.expires_at) : 'Never'}</td>
              <td>{formatTimestamp(token.created_at)}</td>
              <td class="align-right">
                {#if token.revoked_at}
                  <span class="text-light">—</span>
                {:else}
                  <ActionsMenu label={`Actions for ${token.name || 'token'}`}>
                    <button role="menuitem" type="button" onclick={() => confirmRevoke(token)}>
                      Revoke
                    </button>
                  </ActionsMenu>
                {/if}
              </td>
            </tr>
          {:else}
            <tr><td colspan={6} class="align-center text-light">No API tokens yet</td></tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</section>

<!-- Create token -->
<dialog bind:this={createDialog} closedby="any">
  <form onsubmit={save}>
    <header><h2>Create API token</h2></header>
    <ErrorMessage message={formError} onClose={() => (formError = '')} />
    <div class="vstack">
      <label data-field>
        Name
        <input bind:value={name} placeholder="cli@laptop" />
      </label>
      <label data-field>
        Expires in (days)
        <input type="number" min="1" bind:value={expiresInDays} required />
      </label>
    </div>
    <footer>
      <button type="button" class="outline" onclick={() => createDialog?.close()}>Cancel</button>
      <button type="submit" class="gap-1" disabled={saving} aria-busy={saving ? 'true' : undefined} data-spinner="small">
        {saving ? 'Creating...' : 'Create'}
      </button>
    </footer>
  </form>
</dialog>

<!-- One-time secret reveal -->
<dialog bind:this={secretDialog} closedby="any" onclose={closeSecret}>
  <form method="dialog">
    <header><h2>Copy your token</h2></header>
    <div class="vstack gap-3">
      <div role="alert" data-variant="warning">
        <p>This is the only time the token is shown. Copy it now and store it securely.</p>
      </div>
      <div class="hstack gap-2 items-center">
        <code class="token-secret">{issued?.secret}</code>
        <button type="button" class="small" onclick={copySecret}>Copy</button>
      </div>
    </div>
    <footer>
      <button type="button" onclick={closeSecret}>Done</button>
    </footer>
  </form>
</dialog>

<ConfirmDialog
  bind:open={revokeOpen}
  title="Revoke token"
  message="Revoke this token? Any CLI or script using it will immediately lose access."
  confirmLabel="Revoke"
  confirmingLabel="Revoking..."
  confirming={revoking}
  onConfirm={doRevoke}
  onCancel={() => (selected = null)}
/>

<style>
  .token-secret {
    flex: 1 1 auto;
    overflow-wrap: anywhere;
    user-select: all;
  }
</style>
