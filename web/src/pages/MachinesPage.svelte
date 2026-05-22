<script>
  import { onMount } from 'svelte'
  import { fetchMachines } from '@/api.js'
  import SearchInput from '@/components/common/SearchInput.svelte'
  import TagSelector from '@/components/common/TagSelector.svelte'
  import ErrorMessage from '@/components/common/ErrorMessage.svelte'
  import { link } from '@/routes.js'

  let machines = []
  let allTags = []
  let searchTerm = ''
  let selectedTag = ''
  let error = ''

  $: filteredMachines = machines.filter((machine) => {
    const search = searchTerm.trim().toLowerCase()
    const tags = machineTags(machine)
    const matchesSearch =
      !search ||
      hostname(machine).toLowerCase().includes(search) ||
      osLabel(machine).toLowerCase().includes(search) ||
      machineStatus(machine).toLowerCase().includes(search)
    const matchesTag = !selectedTag || tags.includes(selectedTag)
    return matchesSearch && matchesTag
  })

  onMount(loadMachines)

  async function loadMachines() {
    error = ''
    try {
      const data = await fetchMachines()
      machines = data.machines || []
      allTags = Array.from(new Set(machines.flatMap((machine) => machineTags(machine))))
    } catch (err) {
      machines = []
      allTags = []
      error = err.message || 'Failed to fetch machines'
    }
  }

  function statusVariant(status) {
    return status === 'active' ? 'success' : 'danger'
  }

  function hostname(machine) {
    return machine.hostname || machine.Hostname || machine.host_identifier || 'Unknown'
  }

  function osLabel(machine) {
    return [machine.os_name, machine.os_version].filter(Boolean).join(' ') || machine.platform || 'Unknown'
  }

  function machineTags(machine) {
    return [machine.platform, machine.os_name].filter(Boolean)
  }

  function machineStatus(machine) {
    const timestamp = machine.last_seen_at || machine.enrolled_at
    if (!timestamp) return 'unknown'

    const seenAt = new Date(timestamp)
    if (Number.isNaN(seenAt.getTime())) return 'unknown'

    return Date.now() - seenAt.getTime() < 5 * 60 * 1000 ? 'active' : 'offline'
  }

  function formatTimestamp(timestamp) {
    if (!timestamp) return ''
    try {
      return new Date(timestamp).toLocaleString()
    } catch {
      return timestamp
    }
  }
</script>

<section class="vstack gap-4">
  <header class="hstack justify-between">
    <div>
      <h1>Connected Machines</h1>
      <p class="text-light">View and manage all connected osquery endpoints</p>
    </div>
  </header>

  <div class="row">
    <div class="col-8">
      <SearchInput bind:value={searchTerm} placeholder="Search machines..." />
    </div>
    <div class="col-4">
      <TagSelector tags={allTags} bind:value={selectedTag} />
    </div>
  </div>

  <ErrorMessage message={error} onClose={() => (error = '')} />

  <div class="table">
    <table>
      <thead>
        <tr>
          <th>Status</th>
          <th>Hostname</th>
          <th>Operating System</th>
          <th>Last Seen</th>
          <th>Tags</th>
          <th>Version</th>
          <th class="align-right">Actions</th>
        </tr>
      </thead>
      <tbody>
        {#each filteredMachines as machine}
          <tr>
            <td>
              <span class="badge" data-variant={statusVariant(machineStatus(machine))}>
                {machineStatus(machine)}
              </span>
            </td>
            <td>
              <a href={`/machines/${machine.uuid}`} use:link={`/machines/${machine.uuid}`}>
                {hostname(machine)}
              </a>
            </td>
            <td>{osLabel(machine)}</td>
            <td>{formatTimestamp(machine.last_seen_at || machine.enrolled_at)}</td>
            <td>
              <div class="hstack gap-2">
                {#each machineTags(machine) as tag}
                  <span class="badge outline">{tag}</span>
                {/each}
              </div>
            </td>
            <td>{machine.osquery_version || 'Unknown'}</td>
            <td class="align-right">
              <a
                href={`/machines/${machine.uuid}`}
                use:link={`/machines/${machine.uuid}`}
                class="button small outline"
                aria-label={`Open ${hostname(machine)}`}
              >
                Open
              </a>
            </td>
          </tr>
        {:else}
          <tr>
            <td colspan="7" class="align-center text-light">No machines found</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
</section>
