<script>
  import { onMount } from 'svelte'
  import { executeMachineQuery, fetchMachine, fetchMachinePolicies, fetchMachineQueries } from '@/api.js'
  import ErrorMessage from '@/components/common/ErrorMessage.svelte'

  export let params = {}

  let machine = null
  let queryText = ''
  let queryHistory = []
  let policyPosture = []
  let loading = true
  let executing = false
  let error = ''

  $: machineId = params.id

  onMount(loadMachine)

  async function loadMachine() {
    loading = true
    error = ''
    try {
      const [machineData, historyData, policyData] = await Promise.all([
        fetchMachine(machineId),
        fetchMachineQueries(machineId),
        fetchMachinePolicies(machineId)
      ])
      machine = machineData
      queryHistory = Array.isArray(historyData) ? historyData : historyData.queries || []
      policyPosture = Array.isArray(policyData) ? policyData : policyData.policies || []
    } catch (err) {
      error = err.message || 'Failed to load machine data'
    } finally {
      loading = false
    }
  }

  async function runQuery() {
    if (!queryText.trim()) return
    executing = true
    error = ''
    try {
      const result = await executeMachineQuery(machineId, queryText)
      queryHistory = [result, ...queryHistory]
      queryText = ''
      if (result.status === 'pending') {
        setTimeout(loadMachine, 6000)
      }
    } catch (err) {
      error = err.message || 'Query execution failed'
    } finally {
      executing = false
    }
  }

  function formatTimestamp(ts) {
    if (!ts) return ''
    try {
      return new Date(ts).toLocaleString()
    } catch {
      return ts
    }
  }

  function formatResults(results) {
    if (!results) return 'Awaiting results...'
    try {
      return typeof results === 'string' ? results : JSON.stringify(results, null, 2)
    } catch {
      return String(results)
    }
  }

  function isOnline(machine) {
    const timestamp = machine?.last_seen_at || machine?.enrolled_at
    if (!timestamp) return false

    const seenAt = new Date(timestamp)
    if (Number.isNaN(seenAt.getTime())) return false

    return Date.now() - seenAt.getTime() < 5 * 60 * 1000
  }

  function machineOS(machine) {
    return [machine?.os_name, machine?.os_version].filter(Boolean).join(' ') || machine?.platform || ''
  }
</script>

<section class="vstack gap-4">
  {#if loading}
    <article class="card">
      <p>Loading machine...</p>
    </article>
  {:else}
    <header class="hstack justify-between">
      <div>
        <h1>{machine?.hostname || machine?.Hostname || 'Machine'}</h1>
        <p class="text-light">{machineOS(machine)}</p>
      </div>
      <span class="badge" data-variant={isOnline(machine) ? 'success' : 'danger'}>
        {isOnline(machine) ? 'Online' : 'Offline'}
      </span>
    </header>

    <ErrorMessage message={error} onClose={() => (error = '')} />

    <section class="vstack gap-3">
      <h2>Policy Posture</h2>
      <div class="table">
        <table>
          <thead>
            <tr>
              <th>Policy</th>
              <th>Response</th>
              <th>Checked</th>
              <th>Error</th>
              <th>Resolution</th>
            </tr>
          </thead>
          <tbody>
            {#each policyPosture as policy}
              <tr>
                <td>
                  <strong>{policy.name || policy.title}</strong>
                  {#if policy.description}
                    <p class="text-light">{policy.description}</p>
                  {/if}
                </td>
                <td>
                  <span class="badge" data-variant={policy.response === 'passing' ? 'success' : policy.response === 'failing' ? 'danger' : 'warning'}>
                    {policy.stale ? `${policy.response} stale` : policy.response}
                  </span>
                </td>
                <td>{formatTimestamp(policy.checked_at)}</td>
                <td>{policy.last_error || ''}</td>
                <td>{policy.response === 'failing' ? policy.resolution || '' : ''}</td>
              </tr>
            {:else}
              <tr>
                <td colspan="5" class="align-center text-light">No policies target this machine</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </section>

    <article class="card">
      <header>
        <h2>Execute Query</h2>
      </header>
      <form onsubmit={(event) => { event.preventDefault(); runQuery() }}>
        <label>
          SQL Query
          <textarea bind:value={queryText} rows="6" placeholder="SELECT * FROM processes LIMIT 10;"></textarea>
        </label>
        <footer class="hstack justify-end mt-4">
          <button type="submit" disabled={executing || !queryText.trim()}>
            {executing ? 'Executing...' : 'Run Query'}
          </button>
        </footer>
      </form>
    </article>

    <section class="vstack gap-4">
      <h2>Query History</h2>
      {#each queryHistory as query}
        <article class="card">
          <div class="hstack justify-between">
            <code>{query.query}</code>
            <div class="hstack gap-2">
              {#if query.status}
                <span class="badge" data-variant={query.status === 'complete' ? 'success' : query.status === 'error' ? 'danger' : 'warning'}>{query.status}</span>
              {/if}
              <small class="text-light">{formatTimestamp(query.timestamp)}</small>
            </div>
          </div>
          <pre><code>{query.error || formatResults(query.results)}</code></pre>
        </article>
      {:else}
        <article class="card align-center text-light">No queries executed yet</article>
      {/each}
    </section>
  {/if}
</section>
