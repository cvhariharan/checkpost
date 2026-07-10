export type Paginated<T, K extends string> = {
  page_count?: number
  total_count?: number
} & { [P in K]?: T[] }

export type Group = {
  uuid: string
  name?: string
  description?: string
  machine_count?: number
  policy_count?: number
}

export type DeviceOwner = {
  uuid: string
  id?: string
  display_name?: string
  email?: string
  external_id?: string
  department?: string
  title?: string
  phone?: string
  notes?: string
  machine_count?: number
  created_at?: string
  updated_at?: string
}

export type NodeInventory = {
  internal_tracking_id?: string
  notes?: string
  owner?: DeviceOwner
  created_at?: string
  updated_at?: string
}

export type Policy = {
  uuid: string
  name?: string
  title?: string
  description?: string
  query?: string
  resolution?: string
  platform?: string
  severity?: string
  enabled?: boolean
  groups?: Group[]
  target_all_machines?: boolean
  passing_count?: number
  failing_count?: number
  unknown_count?: number
  last_count_updated_at?: string
}

export type Schedule = {
  uuid: string
  title?: string
  sql?: string
  description?: string
  interval?: number
  platform?: string
  snapshot?: boolean
  groups?: Group[]
  target_all_machines?: boolean
  is_system?: boolean
}

export type Machine = {
  uuid: string
  hostname?: string
  Hostname?: string
  display_name?: string
  host_identifier?: string
  os_name?: string
  os_version?: string
  platform?: string
  last_seen_at?: string
  enrolled_at?: string
  osquery_version?: string
  hardware_serial?: string
  groups?: Group[]
  inventory?: NodeInventory | null
  compliance_score?: number | null
}

export type MachineQueryRecord = {
  id?: number | string
  query?: string
  status?: string
  row_count?: number
  error?: string
  timestamp?: string
}

export type QueryTargets = {
  host_ids?: string[]
  group_ids?: string[]
  platforms?: string[]
}

export type QueryRunHost = {
  query_id: string
  node_uuid: string
  hostname?: string
  platform?: string
  status?: string
  row_count?: number
  error?: string
  timestamp?: string
}

export type AdHocQueryResults = {
  columns: string[]
  rows: Record<string, string>[]
  total: number
  page: number
  count_per_page: number
  page_count: number
  export_supported?: boolean
  pending?: boolean
  browsing_disabled?: boolean
  error?: string
}

export type QueryRun = {
  id: string
  query?: string
  targets?: QueryTargets
  host_count?: number
  pending_count?: number
  complete_count?: number
  error_count?: number
  created_at?: string
  hosts?: QueryRunHost[]
}

export type YaraSignatureSource = {
  id: string
  uuid: string
  group_id?: string
  group_name?: string
  url: string
  label?: string
  enabled?: boolean
  created_at?: string
  updated_at?: string
}

export type YaraScan = {
  id: string
  uuid: string
  group_id?: string
  group_name?: string
  paths: string[]
  status: string
  target_count?: number
  completed_count?: number
  match_count?: number
  error?: string
  created_at?: string
  updated_at?: string
  completed_at?: string
}

export type YaraScanMatch = {
  machine_uuid?: string
  hostname?: string
  path?: string
  matches?: string
  count?: number
  created_at?: string
}

export type YaraScanTarget = {
  machine_uuid?: string
  hostname?: string
  status?: string
  dispatched_at?: string
  completed_at?: string
  error?: string
  created_at?: string
  updated_at?: string
}

export type MachinePolicyPosture = {
  uuid?: string
  name?: string
  title?: string
  description?: string
  severity?: string
  response?: 'passing' | 'failing' | 'unknown' | string
  resolution?: string
  stale?: boolean
  last_error?: string
  checked_at?: string
}

export type PolicyMachineRow = {
  uuid: string
  hostname?: string
  display_name?: string
  host_identifier?: string
  platform?: string
  response?: string
  stale?: boolean
  last_error?: string
  checked_at?: string
}

export type ScheduleResultRow = {
  hostname?: string
  node_uuid?: string
  last_seen?: string
  columns?: Record<string, string | number | null | undefined>
}

export type ScheduleResultsPayload = {
  columns: string[]
  rows: ScheduleResultRow[]
  total: number
  page: number
  page_count: number
  export_supported?: boolean
  browsing_disabled?: boolean
}

export type NodeMetric = {
  kind: string
  value: unknown
  collected_at?: string
  updated_at?: string
}

export type NodeMetrics = Record<string, NodeMetric | undefined>

export type MetricSchemas = {
  schemas: Record<string, unknown>
  kinds: string[]
}

export type OsqueryBootstrapPackage = {
  key: string
  label: string
  platform: string
  family: string
  architecture: string
  format: string
  url: string
  sha256: string
}

export type OsqueryBootstrapPlatform = {
  key: string
  label: string
  command: string
  generic_command: string
  script_url: string
  verify_command: string
  restart_command: string
  package?: OsqueryBootstrapPackage
  packages?: OsqueryBootstrapPackage[]
  install_steps?: string[]
  flagfile_path: string
  secret_path: string
  secret: string
  flagfile: string
  script: string
  architecture_notes?: string
  caveats?: string[]
}

export type OsqueryBootstrapOwner = {
  name: string
  email: string
}

export type OsqueryBootstrapProfile = {
  ready: boolean
  checkpost_url: string
  tls_hostname: string
  warnings?: string[]
  owner?: OsqueryBootstrapOwner
  platforms?: OsqueryBootstrapPlatform[]
}

type PageOpts = { page?: number; countPerPage?: number }
type MachinePageOpts = PageOpts & {
  query?: string
  platform?: string
  ownerID?: string
  assigned?: string
}

const BASE_URL = '/api/v1'

function apiUrl(path: string, params: Record<string, unknown> = {}): string {
  const query = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== '') {
      query.set(key, String(value))
    }
  }
  const qs = query.toString()
  return `${BASE_URL}${path}${qs ? `?${qs}` : ''}`
}

async function handleResponse<T>(response: Response): Promise<T> {
  let data: any
  try {
    data = await response.json()
  } catch {
    data = {}
  }
  if (!response.ok) {
    if (
      response.status === 401 &&
      typeof window !== 'undefined' &&
      window.location.pathname !== '/login'
    ) {
      const target = window.location.pathname + window.location.search
      window.location.assign(`/login?redirect_url=${encodeURIComponent(target)}`)
    }
    throw new Error(data?.error || response.statusText || 'API Error')
  }
  return data as T
}

function jsonRequest<T>(path: string, method: string, body: unknown): Promise<T> {
  return fetch(`${BASE_URL}${path}`, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body)
  }).then((res) => handleResponse<T>(res))
}

export function fetchOsqueryBootstrapProfile() {
  return fetch('/bootstrap').then((r) => handleResponse<OsqueryBootstrapProfile>(r))
}

export type BuildInfo = {
  name: string
  version: string
  commit: string
  date: string
}

export function fetchInfo() {
  return fetch(`${BASE_URL}/info`).then((r) => handleResponse<BuildInfo>(r))
}

// Dashboard overview
export type DashboardOverview = {
  generated_at: string
  heartbeat_threshold_seconds: number
  machines: {
    total: number
    online: number
    offline: number
    never_reported: number
    by_platform: { platform: string; total: number; online: number }[]
  }
  compliance: {
    score: number | null
    policy_rows: { passing: number; failing: number; unknown: number }
    top_failing_policies: { uuid: string; name: string; failing_count: number; platform: string }[]
    least_compliant: DashboardComplianceNode[]
    most_compliant: DashboardComplianceNode[]
  }
  security: {
    firing_alerts: {
      critical: number
      high: number
      medium: number
      low: number
      info: number
      total: number
    }
    firing_alert_list: {
      uuid: string
      name: string
      severity: string
      count: number
      last_seen_at: string
    }[]
    recent_yara_matches: {
      scan_uuid: string
      machine_uuid: string
      hostname: string
      path: string
      rules: string
      matched_at: string
    }[]
  }
  recently_enrolled: {
    uuid: string
    hostname: string
    display_name: string
    enrolled_at: string
  }[]
}

export type DashboardComplianceNode = {
  uuid: string
  hostname: string
  display_name: string
  score: number
  passing: number
  failing: number
  total: number
}

export function fetchDashboardOverview(opts: { top?: number } = {}) {
  const { top = 5 } = opts
  return fetch(apiUrl('/dashboard/overview', { top })).then((r) =>
    handleResponse<DashboardOverview>(r)
  )
}

// Schedules
export function fetchSchedules(opts: PageOpts & { query?: string } = {}) {
  const { page = 1, countPerPage = 10, query = '' } = opts
  return fetch(apiUrl('/schedules', { page, count_per_page: countPerPage, q: query })).then((r) =>
    handleResponse<Paginated<Schedule, 'schedules'>>(r)
  )
}

export function fetchSchedule(uuid: string) {
  return fetch(`${BASE_URL}/schedules/${encodeURIComponent(uuid)}`).then((r) =>
    handleResponse<Schedule>(r)
  )
}

export function createSchedule(payload: Record<string, unknown>) {
  return jsonRequest<Schedule>('/schedules', 'POST', payload)
}

export function updateSchedule(uuid: string, payload: Record<string, unknown>) {
  return jsonRequest<Schedule>(`/schedules/${encodeURIComponent(uuid)}`, 'PUT', payload)
}

export function deleteSchedule(uuid: string) {
  return fetch(`${BASE_URL}/schedules/${encodeURIComponent(uuid)}`, { method: 'DELETE' }).then((r) =>
    handleResponse<unknown>(r)
  )
}

export function fetchScheduleResults(
  uuid: string,
  opts: { page?: number; countPerPage?: number; query?: string } = {}
) {
  const { page = 1, countPerPage = 100, query = '' } = opts
  return fetch(
    apiUrl(`/schedules/${encodeURIComponent(uuid)}/results`, { page, count_per_page: countPerPage, q: query })
  ).then((r) => handleResponse<ScheduleResultsPayload>(r))
}

export function scheduleResultsExportUrl(uuid: string) {
  return apiUrl(`/schedules/${encodeURIComponent(uuid)}/results`, { format: 'csv' })
}

// Policies
export function fetchPolicies(opts: PageOpts & { query?: string } = {}) {
  const { page = 1, countPerPage = 10, query = '' } = opts
  return fetch(apiUrl('/policies', { page, count_per_page: countPerPage, q: query })).then((r) =>
    handleResponse<Paginated<Policy, 'policies'>>(r)
  )
}

export function fetchPolicy(uuid: string) {
  return fetch(apiUrl(`/policies/${encodeURIComponent(uuid)}`)).then((r) => handleResponse<Policy>(r))
}

export function createPolicy(payload: Record<string, unknown>) {
  return jsonRequest<Policy>('/policies', 'POST', payload)
}

export function updatePolicy(uuid: string, payload: Record<string, unknown>) {
  return jsonRequest<Policy>(`/policies/${encodeURIComponent(uuid)}`, 'PUT', payload)
}

export function deletePolicy(uuid: string) {
  return fetch(`${BASE_URL}/policies/${encodeURIComponent(uuid)}`, { method: 'DELETE' }).then((r) =>
    handleResponse<unknown>(r)
  )
}

export function fetchPolicyMachines(
  uuid: string,
  opts: { response?: string; page?: number; countPerPage?: number } = {}
) {
  const { response = '', page = 1, countPerPage = 10 } = opts
  return fetch(
    apiUrl(`/policies/${encodeURIComponent(uuid)}/machines`, { response, page, count_per_page: countPerPage })
  ).then((r) => handleResponse<Paginated<PolicyMachineRow, 'machines'>>(r))
}

// Groups
export function fetchGroups(opts: PageOpts & { query?: string } = {}) {
  const { page = 1, countPerPage = 10, query = '' } = opts
  return fetch(apiUrl('/groups', { page, count_per_page: countPerPage, q: query })).then((r) =>
    handleResponse<Paginated<Group, 'groups'>>(r)
  )
}

export function fetchGroup(uuid: string) {
  return fetch(apiUrl(`/groups/${encodeURIComponent(uuid)}`)).then((r) => handleResponse<Group>(r))
}

export function createGroup(payload: Record<string, unknown>) {
  return jsonRequest<Group>('/groups', 'POST', payload)
}

export function updateGroup(uuid: string, payload: Record<string, unknown>) {
  return jsonRequest<Group>(`/groups/${encodeURIComponent(uuid)}`, 'PUT', payload)
}

export function deleteGroup(uuid: string) {
  return fetch(`${BASE_URL}/groups/${encodeURIComponent(uuid)}`, { method: 'DELETE' }).then((r) =>
    handleResponse<unknown>(r)
  )
}

export function fetchGroupMachines(uuid: string, opts: PageOpts = {}) {
  const { page = 1, countPerPage = 100 } = opts
  return fetch(
    apiUrl(`/groups/${encodeURIComponent(uuid)}/machines`, { page, count_per_page: countPerPage })
  ).then((r) => handleResponse<Paginated<Machine, 'machines'>>(r))
}

export function patchGroupMachines(
  uuid: string,
  changes: { add?: string[]; remove?: string[] }
) {
  return jsonRequest<Paginated<Machine, 'machines'>>(
    `/groups/${encodeURIComponent(uuid)}/machines`,
    'PATCH',
    { add_node_ids: changes.add ?? [], remove_node_ids: changes.remove ?? [] }
  )
}

// YARA
export function fetchYaraSignatureSources(opts: PageOpts = {}) {
  const { page = 1, countPerPage = 100 } = opts
  return fetch(apiUrl('/yara/signature-sources', { page, count_per_page: countPerPage })).then((r) =>
    handleResponse<Paginated<YaraSignatureSource, 'sources'>>(r)
  )
}

export function createYaraSignatureSource(payload: Record<string, unknown>) {
  return jsonRequest<YaraSignatureSource>('/yara/signature-sources', 'POST', payload)
}

export function updateYaraSignatureSource(uuid: string, payload: Record<string, unknown>) {
  return jsonRequest<YaraSignatureSource>(
    `/yara/signature-sources/${encodeURIComponent(uuid)}`,
    'PUT',
    payload
  )
}

export function deleteYaraSignatureSource(uuid: string) {
  return fetch(`${BASE_URL}/yara/signature-sources/${encodeURIComponent(uuid)}`, { method: 'DELETE' }).then((r) =>
    handleResponse<unknown>(r)
  )
}

export function createYaraScan(payload: { paths: string[]; group_id?: string; rule_urls: string[] }) {
  return jsonRequest<YaraScan>('/yara/scans', 'POST', payload)
}

export function fetchYaraScans(opts: PageOpts = {}) {
  const { page = 1, countPerPage = 10 } = opts
  return fetch(apiUrl('/yara/scans', { page, count_per_page: countPerPage })).then((r) =>
    handleResponse<Paginated<YaraScan, 'scans'>>(r)
  )
}

export function fetchYaraScanMatches(uuid: string, opts: PageOpts = {}) {
  const { page = 1, countPerPage = 100 } = opts
  return fetch(
    apiUrl(`/yara/scans/${encodeURIComponent(uuid)}/matches`, { page, count_per_page: countPerPage })
  ).then((r) => handleResponse<Paginated<YaraScanMatch, 'matches'>>(r))
}

export function fetchYaraScanTargets(uuid: string, opts: PageOpts = {}) {
  const { page = 1, countPerPage = 1000 } = opts
  return fetch(
    apiUrl(`/yara/scans/${encodeURIComponent(uuid)}/targets`, { page, count_per_page: countPerPage })
  ).then((r) => handleResponse<Paginated<YaraScanTarget, 'targets'>>(r))
}

// Owners and inventory
export function fetchOwners(opts: PageOpts & { query?: string } = {}) {
  const { page = 1, countPerPage = 100, query = '' } = opts
  return fetch(apiUrl('/owners', { page, count_per_page: countPerPage, q: query })).then((r) =>
    handleResponse<Paginated<DeviceOwner, 'owners'>>(r)
  )
}

export function createOwner(payload: Record<string, unknown>) {
  return jsonRequest<{ id: string }>('/owners', 'POST', payload)
}

export function updateOwner(uuid: string, payload: Record<string, unknown>) {
  return jsonRequest<DeviceOwner>(`/owners/${encodeURIComponent(uuid)}`, 'PUT', payload)
}

export function deleteOwner(uuid: string) {
  return fetch(`${BASE_URL}/owners/${encodeURIComponent(uuid)}`, { method: 'DELETE' }).then((r) =>
    handleResponse<unknown>(r)
  )
}

export function fetchOwnerMachines(uuid: string, opts: PageOpts = {}) {
  const { page = 1, countPerPage = 100 } = opts
  return fetch(
    apiUrl(`/owners/${encodeURIComponent(uuid)}/machines`, { page, count_per_page: countPerPage })
  ).then((r) => handleResponse<Paginated<Machine, 'machines'>>(r))
}

export function fetchMachineInventory(id: string) {
  return fetch(`${BASE_URL}/machines/${encodeURIComponent(id)}/inventory`).then((r) =>
    handleResponse<{ inventory?: NodeInventory }>(r)
  )
}

export function updateMachineInventory(
  id: string,
  payload: { owner_id?: string | null; internal_tracking_id?: string; notes?: string }
) {
  return jsonRequest<{ inventory?: NodeInventory }>(
    `/machines/${encodeURIComponent(id)}/inventory`,
    'PUT',
    payload
  )
}

export function deleteMachineInventory(id: string) {
  return fetch(`${BASE_URL}/machines/${encodeURIComponent(id)}/inventory`, { method: 'DELETE' }).then((r) =>
    handleResponse<unknown>(r)
  )
}

// Machines
export function fetchMachines(opts: MachinePageOpts = {}) {
  const { page = 1, countPerPage = 100, query = '', platform = '', ownerID = '', assigned = '' } = opts
  return fetch(
    apiUrl('/machines', {
      page,
      count_per_page: countPerPage,
      q: query,
      platform,
      owner_id: ownerID,
      assigned
    })
  ).then((r) =>
    handleResponse<Paginated<Machine, 'machines'>>(r)
  )
}

export function fetchMachine(id: string) {
  return fetch(`${BASE_URL}/machines/${encodeURIComponent(id)}`).then((r) => handleResponse<Machine>(r))
}

export function updateMachine(id: string, payload: { display_name?: string }) {
  return jsonRequest<Machine>(`/machines/${encodeURIComponent(id)}`, 'PUT', payload)
}

export function deleteMachine(id: string) {
  return fetch(apiUrl(`/machines/${encodeURIComponent(id)}`), { method: 'DELETE' }).then((r) =>
    handleResponse<unknown>(r)
  )
}

export function fetchMachineQueries(id: string, opts: PageOpts = {}) {
  const { page = 1, countPerPage = 10 } = opts
  return fetch(
    apiUrl(`/machines/${encodeURIComponent(id)}/queries`, { page, count_per_page: countPerPage })
  ).then((r) => handleResponse<Paginated<MachineQueryRecord, 'queries'> | MachineQueryRecord[]>(r))
}

export function deleteMachineQuery(machineId: string, queryId: string | number) {
  return fetch(
    `${BASE_URL}/machines/${encodeURIComponent(machineId)}/queries/${encodeURIComponent(String(queryId))}`,
    { method: 'DELETE' }
  ).then((r) => handleResponse<unknown>(r))
}

export function fetchMachineQueryResults(
  machineId: string,
  queryId: string,
  opts: PageOpts = {}
) {
  const { page = 1, countPerPage = 100 } = opts
  return fetch(
    apiUrl(`/machines/${encodeURIComponent(machineId)}/results/${encodeURIComponent(queryId)}`, {
      page,
      count_per_page: countPerPage
    })
  ).then((r) => handleResponse<AdHocQueryResults>(r))
}

export function fetchQueryRunHostResults(runId: string, queryId: string, opts: PageOpts = {}) {
  const { page = 1, countPerPage = 100 } = opts
  return fetch(
    apiUrl(`/query-runs/${encodeURIComponent(runId)}/results/${encodeURIComponent(queryId)}`, {
      page,
      count_per_page: countPerPage
    })
  ).then((r) => handleResponse<AdHocQueryResults>(r))
}

export function machineQueryExportUrl(machineId: string, queryId: string) {
  return apiUrl(`/machines/${encodeURIComponent(machineId)}/results/${encodeURIComponent(queryId)}`, { format: 'csv' })
}

export function queryRunHostExportUrl(runId: string, queryId: string) {
  return apiUrl(`/query-runs/${encodeURIComponent(runId)}/results/${encodeURIComponent(queryId)}`, { format: 'csv' })
}

export function queryRunExportUrl(runId: string) {
  return apiUrl(`/query-runs/${encodeURIComponent(runId)}/results`, { format: 'csv' })
}

export function fetchMachinePolicies(id: string) {
  return fetch(`${BASE_URL}/machines/${encodeURIComponent(id)}/policies`).then((r) =>
    handleResponse<MachinePolicyPosture[] | { policies?: MachinePolicyPosture[] }>(r)
  )
}

export function fetchMachineGroups(id: string) {
  return fetch(`${BASE_URL}/machines/${encodeURIComponent(id)}/groups`).then((r) =>
    handleResponse<Group[] | { groups?: Group[] }>(r)
  )
}

export function updateMachineGroups(id: string, group_ids: string[]) {
  return jsonRequest<Machine>(`/machines/${encodeURIComponent(id)}/groups`, 'PUT', { group_ids })
}

export function executeMachineQuery(id: string, query: string) {
  return jsonRequest<MachineQueryRecord>(`/machines/${encodeURIComponent(id)}/queries`, 'POST', { query })
}

// Multi-host query runs
export type QueryRunPayload = {
  query: string
  host_ids?: string[]
  group_ids?: string[]
  platforms?: string[]
}

export function fetchQueryRuns(opts: PageOpts = {}) {
  const { page = 1, countPerPage = 10 } = opts
  return fetch(apiUrl('/query-runs', { page, count_per_page: countPerPage })).then((r) =>
    handleResponse<Paginated<QueryRun, 'runs'>>(r)
  )
}

export function fetchQueryRun(id: string) {
  return fetch(`${BASE_URL}/query-runs/${encodeURIComponent(id)}`).then((r) => handleResponse<QueryRun>(r))
}

export function createQueryRun(payload: QueryRunPayload) {
  return jsonRequest<QueryRun>('/query-runs', 'POST', payload)
}

export function deleteQueryRun(id: string) {
  return fetch(`${BASE_URL}/query-runs/${encodeURIComponent(id)}`, { method: 'DELETE' }).then((r) =>
    handleResponse<unknown>(r)
  )
}

export function previewQueryTargets(payload: Omit<QueryRunPayload, 'query'>) {
  return jsonRequest<{ host_count: number }>('/query-runs/preview', 'POST', payload)
}

export function fetchMachineMetrics(id: string) {
  return fetch(`${BASE_URL}/machines/${encodeURIComponent(id)}/metrics`).then((r) =>
    handleResponse<{ metrics: NodeMetrics }>(r)
  )
}

export function fetchMetricSchemas() {
  return fetch(`${BASE_URL}/metrics/schemas`).then((r) => handleResponse<MetricSchemas>(r))
}

// osquery schema — static asset under web/static/, not a /api/v1 endpoint.

export type OsqueryColumn = { name: string; type: string; description: string }
export type OsqueryTable = {
  name: string
  description: string
  platforms: string[]
  columns: OsqueryColumn[]
}
export type OsquerySchema = { version: string; tables: OsqueryTable[] }

let osquerySchemaPromise: Promise<OsquerySchema> | null = null

// Memoized so every SqlEditor shares one request per page load.
export function fetchOsquerySchema(): Promise<OsquerySchema> {
  if (!osquerySchemaPromise) {
    osquerySchemaPromise = fetch('/osquery-schema.json')
      .then((r) => {
        if (!r.ok) throw new Error('failed to load osquery schema')
        return r.json() as Promise<OsquerySchema>
      })
      .catch((err) => {
        osquerySchemaPromise = null // retry on next mount
        throw err
      })
  }
  return osquerySchemaPromise
}

// Authentication & authorization ------------------------------------------

export type SessionUser = {
  uuid: string
  username?: string
  name?: string
  email?: string
  login_type?: string
}

export type Me = {
  user: SessionUser
  roles: string[]
  permissions: Record<string, string[]>
  owner_only_access?: boolean
}

export type Providers = {
  password: boolean
  sso: { enabled: boolean; label: string }
}

export type User = {
  uuid: string
  username: string
  name?: string
  email?: string
  login_type?: string
  disabled?: boolean
  last_login_at?: string
  created_at?: string
  updated_at?: string
}

export type UserGroup = {
  uuid: string
  name: string
  description?: string
  oidc_claim_value?: string
  member_count?: number
  created_at?: string
  updated_at?: string
}

export type UserGroupMember = {
  user_uuid: string
  username?: string
  name?: string
  email?: string
  login_type?: string
  disabled?: boolean
  source?: string
}

export type RoleBinding = {
  uuid: string
  role: string
  scope_group_uuid?: string
  scope_group_name?: string
  created_at?: string
}

export type RoleDefinition = {
  name: string
  description?: string
  permissions: Record<string, string[]>
}

export type PermissionCatalogEntry = {
  resource: string
  actions: string[]
  scopable: boolean
}

export type RolesResponse = {
  roles: RoleDefinition[]
  catalog: { resources: PermissionCatalogEntry[] }
}

export type APIToken = {
  uuid: string
  name?: string
  source?: string
  expires_at?: string
  last_used_at?: string
  revoked_at?: string
  created_at?: string
}

export type IssuedAPIToken = APIToken & {
  secret: string
}

export function fetchMe() {
  return fetch(`${BASE_URL}/me`).then((r) => handleResponse<Me>(r))
}

export function fetchProviders() {
  return fetch('/auth/providers').then((r) => handleResponse<Providers>(r))
}

export function login(username: string, password: string) {
  return fetch('/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password })
  }).then((r) => handleResponse<unknown>(r))
}

export function logout() {
  return fetch('/logout', { method: 'POST' }).then((r) => handleResponse<unknown>(r))
}

// API tokens (self-service).
export function fetchAPITokens() {
  return fetch(`${BASE_URL}/auth/tokens`).then((r) => handleResponse<APIToken[]>(r))
}

export function createAPIToken(payload: { name: string; expires_in_days: number }) {
  return jsonRequest<IssuedAPIToken>('/auth/tokens', 'POST', payload)
}

export function revokeAPIToken(uuid: string) {
  return fetch(`${BASE_URL}/auth/tokens/${encodeURIComponent(uuid)}`, { method: 'DELETE' }).then(
    (r) => handleResponse<unknown>(r)
  )
}

// Users
export function fetchUsers(opts: PageOpts = {}) {
  const { page = 1, countPerPage = 50 } = opts
  return fetch(apiUrl('/users', { page, count_per_page: countPerPage })).then((r) =>
    handleResponse<Paginated<User, 'users'>>(r)
  )
}

export function createUser(payload: {
  name?: string
  email: string
}) {
  return jsonRequest<User>('/users', 'POST', payload)
}

export function updateUser(
  uuid: string,
  payload: { name?: string; email?: string; disabled?: boolean }
) {
  return jsonRequest<User>(`/users/${encodeURIComponent(uuid)}`, 'PUT', payload)
}

export function deleteUser(uuid: string) {
  return fetch(`${BASE_URL}/users/${encodeURIComponent(uuid)}`, { method: 'DELETE' }).then((r) =>
    handleResponse<unknown>(r)
  )
}

// User groups
export function fetchUserGroups(opts: PageOpts = {}) {
  const { page = 1, countPerPage = 50 } = opts
  return fetch(apiUrl('/user-groups', { page, count_per_page: countPerPage })).then((r) =>
    handleResponse<Paginated<UserGroup, 'user_groups'>>(r)
  )
}

export function createUserGroup(payload: {
  name: string
  description?: string
  oidc_claim_value?: string
}) {
  return jsonRequest<UserGroup>('/user-groups', 'POST', payload)
}

export function updateUserGroup(
  uuid: string,
  payload: { name: string; description?: string; oidc_claim_value?: string }
) {
  return jsonRequest<UserGroup>(`/user-groups/${encodeURIComponent(uuid)}`, 'PUT', payload)
}

export function deleteUserGroup(uuid: string) {
  return fetch(`${BASE_URL}/user-groups/${encodeURIComponent(uuid)}`, { method: 'DELETE' }).then(
    (r) => handleResponse<unknown>(r)
  )
}

export function fetchUserGroupMembers(uuid: string) {
  return fetch(`${BASE_URL}/user-groups/${encodeURIComponent(uuid)}/members`).then((r) =>
    handleResponse<{ members: UserGroupMember[] }>(r)
  )
}

export function addUserGroupMember(uuid: string, userId: string) {
  return jsonRequest<unknown>(`/user-groups/${encodeURIComponent(uuid)}/members`, 'POST', {
    user_id: userId
  })
}

export function removeUserGroupMember(uuid: string, userId: string) {
  return fetch(
    `${BASE_URL}/user-groups/${encodeURIComponent(uuid)}/members/${encodeURIComponent(userId)}`,
    { method: 'DELETE' }
  ).then((r) => handleResponse<unknown>(r))
}

// Roles & bindings
export function fetchRoles() {
  return fetch(`${BASE_URL}/roles`).then((r) => handleResponse<RolesResponse>(r))
}

export function fetchRoleBindings(subjectType: 'user' | 'usergroup', subjectId: string) {
  return fetch(apiUrl('/role-bindings', { subject_type: subjectType, subject_id: subjectId })).then(
    (r) => handleResponse<{ bindings: RoleBinding[] }>(r)
  )
}

export function createRoleBinding(payload: {
  subject_type: 'user' | 'usergroup'
  subject_id: string
  role: string
  scope_group_uuid?: string | null
}) {
  return jsonRequest<RoleBinding>('/role-bindings', 'POST', payload)
}

export function deleteRoleBinding(uuid: string) {
  return fetch(`${BASE_URL}/role-bindings/${encodeURIComponent(uuid)}`, { method: 'DELETE' }).then(
    (r) => handleResponse<unknown>(r)
  )
}

// Alerting
export type AlertTarget = {
  uuid: string
  name?: string
  type?: string
  config?: Record<string, unknown>
  enabled?: boolean
  created_at?: string
  updated_at?: string
}

export type AlertRule = {
  uuid: string
  name?: string
  description?: string
  source?: string
  params?: Record<string, unknown>
  severity?: string
  enabled?: boolean
  evaluation_interval_seconds?: number
  for_seconds?: number
  repeat_interval_seconds?: number
  target_ids?: string[]
  last_evaluated_at?: string
  created_at?: string
  updated_at?: string
}

export type AlertSource = {
  type: string
  schema: Record<string, unknown>
}

export function fetchAlertTargets(opts: PageOpts = {}) {
  const { page = 1, countPerPage = 10 } = opts
  return fetch(apiUrl('/alert-targets', { page, count_per_page: countPerPage })).then((r) =>
    handleResponse<Paginated<AlertTarget, 'targets'>>(r)
  )
}

export function createAlertTarget(payload: Record<string, unknown>) {
  return jsonRequest<AlertTarget>('/alert-targets', 'POST', payload)
}

export function updateAlertTarget(uuid: string, payload: Record<string, unknown>) {
  return jsonRequest<AlertTarget>(`/alert-targets/${encodeURIComponent(uuid)}`, 'PUT', payload)
}

export function deleteAlertTarget(uuid: string) {
  return fetch(`${BASE_URL}/alert-targets/${encodeURIComponent(uuid)}`, { method: 'DELETE' }).then(
    (r) => handleResponse<unknown>(r)
  )
}

export function testAlertTarget(uuid: string) {
  return jsonRequest<unknown>(`/alert-targets/${encodeURIComponent(uuid)}/test`, 'POST', {})
}

export function fetchAlertRules(opts: PageOpts = {}) {
  const { page = 1, countPerPage = 10 } = opts
  return fetch(apiUrl('/alert-rules', { page, count_per_page: countPerPage })).then((r) =>
    handleResponse<Paginated<AlertRule, 'rules'>>(r)
  )
}

export function fetchAlertRule(uuid: string) {
  return fetch(apiUrl(`/alert-rules/${encodeURIComponent(uuid)}`)).then((r) =>
    handleResponse<AlertRule>(r)
  )
}

export function createAlertRule(payload: Record<string, unknown>) {
  return jsonRequest<AlertRule>('/alert-rules', 'POST', payload)
}

export function updateAlertRule(uuid: string, payload: Record<string, unknown>) {
  return jsonRequest<AlertRule>(`/alert-rules/${encodeURIComponent(uuid)}`, 'PUT', payload)
}

export function deleteAlertRule(uuid: string) {
  return fetch(`${BASE_URL}/alert-rules/${encodeURIComponent(uuid)}`, { method: 'DELETE' }).then(
    (r) => handleResponse<unknown>(r)
  )
}

export function fetchAlertSources() {
  return fetch(`${BASE_URL}/alert-sources`).then((r) =>
    handleResponse<{ sources: AlertSource[] }>(r)
  )
}
