<script lang="ts">
  import { onMount } from 'svelte'
  import {
    fetchEnrollmentSecrets,
    createBulkEnrollmentSecret,
    fetchEnrollmentSecretBootstrap,
    revokeEnrollmentSecret,
    fetchEnrollmentSecretMachines,
    type EnrollmentSecret,
    type Machine
  } from '$lib/api'
  import { me, canFrom } from '$lib/auth'
  import { formatTimestamp, isOnline, machineHostname, toast } from '$lib/util'
  import ErrorMessage from '$lib/components/ErrorMessage.svelte'
  import Spinner from '$lib/components/Spinner.svelte'
  import ActionsMenu from '$lib/components/ActionsMenu.svelte'
  import ConfirmDialog from '$lib/components/ConfirmDialog.svelte'
  import CopyableId from '$lib/components/CopyableId.svelte'
  import Plus from '@lucide/svelte/icons/plus'
  import Copy from '@lucide/svelte/icons/copy'

  let secrets = $state<EnrollmentSecret[]>([])
  let loading = $state(true)
  let error = $state('')

  let revokeOpen = $state(false)
  let selected = $state<EnrollmentSecret | null>(null)
  let revoking = $state(false)

  let machinesDialog = $state<HTMLDialogElement>()
  let machinesFor = $state<EnrollmentSecret | null>(null)
  let machines = $state<Machine[]>([])
  let machinesLoading = $state(false)

  let createDialog = $state<HTMLDialogElement>()
  let newName = $state('')
  let creating = $state(false)

  const PLATFORMS = [
    { key: 'linux', label: 'Linux' },
    { key: 'macos', label: 'macOS' },
    { key: 'windows', label: 'Windows' }
  ]
  let commandsDialog = $state<HTMLDialogElement>()
  let commandsFor = $state<EnrollmentSecret | null>(null)
  let commands = $state<Record<string, string>>({})
  let commandsLoading = $state(false)
  let activePlatform = $state('linux')
  let groupsInput = $state('')

  const canManage = $derived(canFrom($me, 'setting', 'update'))

  type Status = { label: string; variant: 'success' | 'danger' }

  function statusOf(secret: EnrollmentSecret): Status {
    if (secret.revoked_at) return { label: 'Revoked', variant: 'danger' }
    return { label: 'Active', variant: 'success' }
  }

  function machineStatusLabel(machine: Machine): string {
    if (!machine.last_seen_at && !machine.enrolled_at) return 'unknown'
    return isOnline(machine) ? 'active' : 'offline'
  }

  function machineStatusVariant(machine: Machine): 'success' | 'danger' | 'warning' {
    const label = machineStatusLabel(machine)
    if (label === 'active') return 'success'
    if (label === 'offline') return 'danger'
    return 'warning'
  }

  onMount(() => {
    void load()
  })

  async function load() {
    loading = true
    error = ''
    try {
      secrets = (await fetchEnrollmentSecrets()).enrollment_secrets || []
    } catch (err) {
      error = (err as Error).message || 'Failed to load enrollment keys'
    } finally {
      loading = false
    }
  }

  function openCreate() {
    newName = ''
    error = ''
    createDialog?.showModal()
  }

  async function doCreate(event: Event) {
    event.preventDefault()
    if (!newName.trim()) return
    creating = true
    error = ''
    try {
      const res = await createBulkEnrollmentSecret(newName.trim())
      createDialog?.close()
      await load()
      openCommands(res.enrollment_secret, res.commands)
    } catch (err) {
      error = (err as Error).message || 'Failed to create enrollment key'
    } finally {
      creating = false
    }
  }

  function openCommands(secret: EnrollmentSecret, initial?: Record<string, string>) {
    commandsFor = secret
    activePlatform = 'linux'
    groupsInput = ''
    commands = initial || {}
    commandsDialog?.showModal()
    if (!initial) void refreshCommands()
  }

  async function refreshCommands() {
    if (!commandsFor) return
    commandsLoading = true
    try {
      commands = (await fetchEnrollmentSecretBootstrap(commandsFor.uuid, groupsInput.trim())).commands
    } catch (err) {
      error = (err as Error).message || 'Failed to load install command'
    } finally {
      commandsLoading = false
    }
  }

  async function copyCommand() {
    const value = commands[activePlatform] || ''
    try {
      await navigator.clipboard.writeText(value)
      toast('Install command copied', 'Clipboard', { variant: 'success' })
    } catch {
      toast('Could not copy command', 'Clipboard', { variant: 'danger' })
    }
  }

  function confirmRevoke(secret: EnrollmentSecret) {
    selected = secret
    revokeOpen = true
  }

  async function doRevoke() {
    if (!selected) return
    revoking = true
    try {
      await revokeEnrollmentSecret(selected.uuid)
      revokeOpen = false
      selected = null
      await load()
    } catch (err) {
      error = (err as Error).message || 'Failed to revoke enrollment key'
    } finally {
      revoking = false
    }
  }

  async function openMachines(secret: EnrollmentSecret) {
    machinesFor = secret
    machines = []
    machinesLoading = true
    machinesDialog?.showModal()
    try {
      machines = (await fetchEnrollmentSecretMachines(secret.uuid)).machines || []
    } catch (err) {
      error = (err as Error).message || 'Failed to load machines'
      machinesDialog?.close()
    } finally {
      machinesLoading = false
    }
  }
</script>

<section class="vstack gap-4">
  <header class="hstack justify-between mb-4">
    <div>
      <h1 class="mb-2">Enrollment keys</h1>
    </div>
    {#if canManage}
      <button type="button" class="gap-1" onclick={openCreate}>
        <Plus size={16} aria-hidden="true" />
        Create enrollment key
      </button>
    {/if}
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
            <th>Secret ID</th>
            <th>Type</th>
            <th>Generated by</th>
            <th class="align-right">Machines</th>
            <th>Status</th>
            <th>Created</th>
            <th class="align-right"><span class="sr-only">Actions</span></th>
          </tr>
        </thead>
        <tbody>
          {#each secrets as secret}
            {@const status = statusOf(secret)}
            <tr>
              <td>{secret.name || '—'}</td>
              <td><CopyableId value={secret.secret_id} label="Secret ID" /></td>
              <td><span class="badge outline">{secret.type}</span></td>
              <td>{secret.generated_by || '—'}</td>
              <td class="align-right">
                {#if secret.machine_count > 0}
                  <button type="button" class="cell-link" onclick={() => openMachines(secret)}>
                    {secret.machine_count}
                  </button>
                {:else}
                  <span class="text-light">0</span>
                {/if}
              </td>
              <td><span class="badge" data-variant={status.variant}>{status.label}</span></td>
              <td>{formatTimestamp(secret.created_at)}</td>
              <td class="align-right">
                {#if secret.revoked_at}
                  <span class="text-light">—</span>
                {:else}
                  <ActionsMenu label={`Actions for ${secret.name || secret.secret_id}`}>
                    {#if secret.type === 'bulk'}
                      <button role="menuitem" type="button" onclick={() => openCommands(secret)}>
                        Show install command
                      </button>
                    {/if}
                    {#if canManage}
                      <button role="menuitem" type="button" onclick={() => confirmRevoke(secret)}>
                        Revoke
                      </button>
                    {/if}
                  </ActionsMenu>
                {/if}
              </td>
            </tr>
          {:else}
            <tr><td colspan={8} class="align-center text-light">No enrollment keys yet</td></tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</section>

<dialog bind:this={createDialog} onclose={() => (error = '')} closedby="any">
  <form onsubmit={doCreate}>
    <header>
      <h2>Create enrollment key</h2>
    </header>

    <ErrorMessage message={error} onClose={() => (error = '')} />

    <div class="vstack">
      <label data-field>
        Name <span class="req" aria-hidden="true">*</span>
        <input bind:value={newName} required maxlength="255" placeholder="prod-linux-servers" />
      </label>
    </div>

    <footer>
      <button type="button" class="outline" onclick={() => createDialog?.close()}>Cancel</button>
      <button
        type="submit"
        class="gap-1"
        disabled={creating || !newName.trim()}
        aria-busy={creating ? 'true' : undefined}
        data-spinner="small"
      >
        {creating ? 'Creating...' : 'Create'}
      </button>
    </footer>
  </form>
</dialog>

<dialog bind:this={commandsDialog} class="commands-dialog" closedby="any">
  <header>
    <h2>Install command</h2>
    {#if commandsFor}
      <p class="text-light">
        Run this on each host to enroll with <strong>{commandsFor.name || 'this key'}</strong>.
      </p>
    {/if}
  </header>

  <div class="vstack gap-3">
    <div role="tablist" class="hstack gap-1">
      {#each PLATFORMS as p}
        <button
          type="button"
          class={activePlatform === p.key ? '' : 'outline'}
          onclick={() => (activePlatform = p.key)}
        >
          {p.label}
        </button>
      {/each}
    </div>

    <label data-field>
      Groups (optional, comma-separated)
      <input bind:value={groupsInput} placeholder="prod,linux" onchange={refreshCommands} />
      <small class="text-light">Groups must already exist. Hosts auto-join them at enrollment.</small>
    </label>

    {#if commandsLoading}
      <Spinner />
    {:else}
      <div class="hstack justify-end">
        <button type="button" class="small outline gap-1" onclick={copyCommand}>
          <Copy size={14} aria-hidden="true" />
          Copy command
        </button>
      </div>
      <pre><code>{commands[activePlatform] || ''}</code></pre>
    {/if}
  </div>

  <footer class="hstack justify-end">
    <button type="button" class="outline" onclick={() => commandsDialog?.close()}>Close</button>
  </footer>
</dialog>

<ConfirmDialog
  bind:open={revokeOpen}
  title="Revoke enrollment key"
  message="Revoke this key? New machines will no longer be able to enroll with it. Machines already enrolled keep working."
  confirmLabel="Revoke"
  confirmingLabel="Revoking..."
  confirming={revoking}
  onConfirm={doRevoke}
  onCancel={() => (selected = null)}
/>

<dialog bind:this={machinesDialog} class="machines-dialog" closedby="any">
  <header>
    <h2>Machines using this key</h2>
    {#if machinesFor}
      <p class="text-light">{machinesFor.name || machinesFor.secret_id}</p>
    {/if}
  </header>
  {#if machinesLoading}
    <Spinner fill />
  {:else}
    <div class="table">
      <table>
        <thead>
          <tr>
            <th>Host</th>
            <th>Platform</th>
            <th>Status</th>
            <th>Last seen</th>
          </tr>
        </thead>
        <tbody>
          {#each machines as machine}
            <tr>
              <td><a href={`/machines/${machine.uuid}`}>{machineHostname(machine)}</a></td>
              <td>{machine.platform || '—'}</td>
              <td>
                <span class="badge" data-variant={machineStatusVariant(machine)}>
                  {machineStatusLabel(machine)}
                </span>
              </td>
              <td>{machine.last_seen_at ? formatTimestamp(machine.last_seen_at) : 'Never'}</td>
            </tr>
          {:else}
            <tr><td colspan={4} class="align-center text-light">No machines</td></tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
  <footer>
    <button type="button" class="outline" onclick={() => machinesDialog?.close()}>Close</button>
  </footer>
</dialog>

<style>
  .machines-dialog,
  .commands-dialog {
    width: min(48rem, 94vw);
  }

  pre {
    overflow: auto;
    max-width: 100%;
    padding: var(--space-4);
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--faint);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
</style>
