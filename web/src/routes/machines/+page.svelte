<script lang="ts">
  import { onMount } from 'svelte'
  import { fetchMachines, type Machine } from '$lib/api'
  import { formatTimestamp, isOnline, machineHostname, machineOS } from '$lib/util'
  import SearchInput from '$lib/components/SearchInput.svelte'
  import SelectFilter from '$lib/components/SelectFilter.svelte'
  import ErrorMessage from '$lib/components/ErrorMessage.svelte'
  import Spinner from '$lib/components/Spinner.svelte'
  import Truncate from '$lib/components/Truncate.svelte'
  import BadgeList from '$lib/components/BadgeList.svelte'

  let machines: Machine[] = []
  let allPlatforms: string[] = []
  let searchTerm = ''
  let selectedPlatform = ''
  let error = ''
  let loading = true

  onMount(loadMachines)

  async function loadMachines() {
    loading = true
    error = ''
    try {
      const data = await fetchMachines()
      machines = data.machines || []
      allPlatforms = Array.from(
        new Set(machines.map((m) => m.platform).filter((p): p is string => Boolean(p)))
      ).sort()
    } catch (err) {
      machines = []
      allPlatforms = []
      error = (err as Error).message || 'Failed to fetch machines'
    } finally {
      loading = false
    }
  }

  function statusLabel(machine: Machine): string {
    const ts = machine.last_seen_at || machine.enrolled_at
    if (!ts) return 'unknown'
    return isOnline(machine) ? 'active' : 'offline'
  }

  function statusVariant(machine: Machine): 'success' | 'danger' | 'warning' {
    const label = statusLabel(machine)
    if (label === 'active') return 'success'
    if (label === 'offline') return 'danger'
    return 'warning'
  }

  $: filteredMachines = machines.filter((machine) => {
    const search = searchTerm.trim().toLowerCase()
    const matchesSearch =
      !search ||
      machineHostname(machine).toLowerCase().includes(search) ||
      machineOS(machine).toLowerCase().includes(search) ||
      statusLabel(machine).includes(search)
    const matchesPlatform = !selectedPlatform || machine.platform === selectedPlatform
    return matchesSearch && matchesPlatform
  })
</script>

<section class="vstack gap-4">
  <header class="hstack justify-between mb-4">
    <div>
      <h1 class="mb-2">Connected Machines</h1>
      <p class="text-light">View and manage all connected osquery endpoints</p>
    </div>
  </header>

  <div class="row">
    <div class="col-8">
      <SearchInput bind:value={searchTerm} placeholder="Search machines..." />
    </div>
    <div class="col-4">
      <SelectFilter
        options={allPlatforms}
        bind:value={selectedPlatform}
        label="Platform"
        allLabel="All platforms"
      />
    </div>
  </div>

  <ErrorMessage message={error} onClose={() => (error = '')} />

  {#if loading}
    <Spinner />
  {:else}
    <div class="table">
      <table class="machines-table">
        <thead>
          <tr>
            <th class="col-status">Status</th>
            <th class="col-hostname">Hostname</th>
            <th>Operating System</th>
            <th class="col-platform">Platform</th>
            <th class="col-last-seen">Last Seen</th>
            <th class="col-groups">Groups</th>
            <th class="col-version">Version</th>
          </tr>
        </thead>
        <tbody>
          {#each filteredMachines as machine}
            <tr>
              <td>
                <span class="badge" data-variant={statusVariant(machine)}>{statusLabel(machine)}</span>
              </td>
              <td>
                <a href="/machines/{machine.uuid}">
                  <Truncate text={machineHostname(machine)} />
                </a>
              </td>
              <td>
                <Truncate text={machineOS(machine)} />
              </td>
              <td>
                {#if machine.platform}
                  <span class="badge outline">{machine.platform}</span>
                {/if}
              </td>
              <td>
                <Truncate text={formatTimestamp(machine.last_seen_at || machine.enrolled_at)} />
              </td>
              <td>
                <BadgeList items={machine.groups || []} max={2} />
              </td>
              <td>
                <Truncate text={machine.osquery_version || 'Unknown'} />
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
  {/if}
</section>

<style>
  .machines-table {
    table-layout: fixed;
    width: 100%;
  }
  .machines-table .col-status {
    width: 6rem;
  }
  .machines-table .col-hostname {
    width: 14rem;
  }
  .machines-table .col-platform {
    width: 6rem;
  }
  .machines-table .col-last-seen {
    width: 12rem;
  }
  .machines-table .col-groups {
    width: 16rem;
  }
  .machines-table .col-version {
    width: 7rem;
  }
</style>
