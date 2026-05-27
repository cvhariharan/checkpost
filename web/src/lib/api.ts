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

export type Policy = {
  uuid: string
  name?: string
  title?: string
  description?: string
  query?: string
  resolution?: string
  platform?: string
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
}

export type Machine = {
  uuid: string
  hostname?: string
  Hostname?: string
  host_identifier?: string
  os_name?: string
  os_version?: string
  platform?: string
  last_seen_at?: string
  enrolled_at?: string
  osquery_version?: string
  groups?: Group[]
}

export type MachineQueryRecord = {
  id?: number | string
  query?: string
  status?: string
  results?: unknown
  error?: string
  timestamp?: string
}

export type MachinePolicyPosture = {
  uuid?: string
  name?: string
  title?: string
  description?: string
  response?: 'passing' | 'failing' | 'unknown' | string
  resolution?: string
  stale?: boolean
  last_error?: string
  checked_at?: string
}

export type PolicyMachineRow = {
  uuid: string
  hostname?: string
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

type PageOpts = { page?: number; countPerPage?: number }

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

// Schedules
export function fetchSchedules(opts: PageOpts = {}) {
  const { page = 1, countPerPage = 10 } = opts
  return fetch(apiUrl('/schedules', { page, count_per_page: countPerPage })).then((r) =>
    handleResponse<Paginated<Schedule, 'schedules'>>(r)
  )
}

export function fetchSchedule(uuid: string) {
  return fetch(`${BASE_URL}/schedule/${encodeURIComponent(uuid)}`).then((r) =>
    handleResponse<Schedule>(r)
  )
}

export function createSchedule(payload: Record<string, unknown>) {
  return jsonRequest<Schedule>('/schedule', 'POST', payload)
}

export function updateSchedule(uuid: string, payload: Record<string, unknown>) {
  return jsonRequest<Schedule>(`/schedule/${encodeURIComponent(uuid)}`, 'PUT', payload)
}

export function deleteSchedule(uuid: string) {
  return fetch(`${BASE_URL}/schedule/${encodeURIComponent(uuid)}`, { method: 'DELETE' }).then((r) =>
    handleResponse<unknown>(r)
  )
}

export function fetchScheduleResults(
  uuid: string,
  opts: { page?: number; countPerPage?: number; query?: string } = {}
) {
  const { page = 1, countPerPage = 100, query = '' } = opts
  return fetch(
    apiUrl(`/schedule/${encodeURIComponent(uuid)}/results`, { page, count_per_page: countPerPage, q: query })
  ).then((r) => handleResponse<ScheduleResultsPayload>(r))
}

// Policies
export function fetchPolicies(opts: PageOpts = {}) {
  const { page = 1, countPerPage = 10 } = opts
  return fetch(apiUrl('/policies', { page, count_per_page: countPerPage })).then((r) =>
    handleResponse<Paginated<Policy, 'policies'>>(r)
  )
}

export function createPolicy(payload: Record<string, unknown>) {
  return jsonRequest<Policy>('/policy', 'POST', payload)
}

export function updatePolicy(uuid: string, payload: Record<string, unknown>) {
  return jsonRequest<Policy>(`/policy/${encodeURIComponent(uuid)}`, 'PUT', payload)
}

export function deletePolicy(uuid: string) {
  return fetch(`${BASE_URL}/policy/${encodeURIComponent(uuid)}`, { method: 'DELETE' }).then((r) =>
    handleResponse<unknown>(r)
  )
}

export function fetchPolicyMachines(
  uuid: string,
  opts: { response?: string; page?: number; countPerPage?: number } = {}
) {
  const { response = '', page = 1, countPerPage = 10 } = opts
  return fetch(
    apiUrl(`/policy/${encodeURIComponent(uuid)}/machines`, { response, page, count_per_page: countPerPage })
  ).then((r) => handleResponse<Paginated<PolicyMachineRow, 'machines'>>(r))
}

// Groups
export function fetchGroups(opts: PageOpts = {}) {
  const { page = 1, countPerPage = 10 } = opts
  return fetch(apiUrl('/groups', { page, count_per_page: countPerPage })).then((r) =>
    handleResponse<Paginated<Group, 'groups'>>(r)
  )
}

export function createGroup(payload: Record<string, unknown>) {
  return jsonRequest<Group>('/group', 'POST', payload)
}

export function updateGroup(uuid: string, payload: Record<string, unknown>) {
  return jsonRequest<Group>(`/group/${encodeURIComponent(uuid)}`, 'PUT', payload)
}

export function deleteGroup(uuid: string) {
  return fetch(`${BASE_URL}/group/${encodeURIComponent(uuid)}`, { method: 'DELETE' }).then((r) =>
    handleResponse<unknown>(r)
  )
}

// Machines
export function fetchMachines(opts: PageOpts = {}) {
  const { page = 1, countPerPage = 100 } = opts
  return fetch(apiUrl('/machines', { page, count_per_page: countPerPage })).then((r) =>
    handleResponse<Paginated<Machine, 'machines'>>(r)
  )
}

export function fetchMachine(id: string) {
  return fetch(`${BASE_URL}/machines/${encodeURIComponent(id)}`).then((r) => handleResponse<Machine>(r))
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
  return jsonRequest<MachineQueryRecord>(`/machines/${encodeURIComponent(id)}/query`, 'POST', { query })
}

export function fetchMachineMetrics(id: string) {
  return fetch(`${BASE_URL}/machines/${encodeURIComponent(id)}/metrics`).then((r) =>
    handleResponse<{ metrics: NodeMetrics }>(r)
  )
}

export function fetchMetricSchemas() {
  return fetch(`${BASE_URL}/metrics/schemas`).then((r) => handleResponse<MetricSchemas>(r))
}
