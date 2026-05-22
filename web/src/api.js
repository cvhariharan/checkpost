// watcher/web/src/api.js
// Centralized API module for all backend calls

const BASE_URL = '/api/v1';

function apiUrl(path, params = {}) {
  const query = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== '') {
      query.set(key, value);
    }
  }
  const qs = query.toString();
  return `${BASE_URL}${path}${qs ? `?${qs}` : ''}`;
}

async function handleResponse(response) {
  let data;
  try {
    data = await response.json();
  } catch {
    data = {};
  }
  if (!response.ok) {
    const error = data.error || response.statusText || 'API Error';
    throw new Error(error);
  }
  return data;
}

// Queries
export async function fetchQueries({ page = 1, countPerPage = 10 } = {}) {
  const res = await fetch(apiUrl('/queries', { page, count_per_page: countPerPage }));
  return handleResponse(res);
}

export async function fetchAllQueries() {
  const res = await fetch(apiUrl('/queries', { page: 1, count_per_page: 1000 }));
  return handleResponse(res);
}

export async function createQuery({ title, query, description }) {
  const res = await fetch(`${BASE_URL}/query`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title, query, description }),
  });
  return handleResponse(res);
}

export async function updateQuery(uuid, { title, query, description }) {
  const res = await fetch(`${BASE_URL}/query/${encodeURIComponent(uuid)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ title, query, description }),
  });
  return handleResponse(res);
}

export async function deleteQuery(uuid) {
  const res = await fetch(`${BASE_URL}/query/${encodeURIComponent(uuid)}`, { method: 'DELETE' });
  return handleResponse(res);
}

// Schedules
export async function fetchSchedules({ page = 1, countPerPage = 10 } = {}) {
  const res = await fetch(apiUrl('/schedules', { page, count_per_page: countPerPage }));
  return handleResponse(res);
}

export async function createSchedule(payload) {
  const res = await fetch(`${BASE_URL}/schedule`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  return handleResponse(res);
}

export async function updateSchedule(uuid, payload) {
  const res = await fetch(`${BASE_URL}/schedule/${encodeURIComponent(uuid)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  return handleResponse(res);
}

export async function deleteSchedule(uuid) {
  const res = await fetch(`${BASE_URL}/schedule/${encodeURIComponent(uuid)}`, { method: 'DELETE' });
  return handleResponse(res);
}

export async function fetchPolicies({ page = 1, countPerPage = 10 } = {}) {
  const res = await fetch(apiUrl('/policies', { page, count_per_page: countPerPage }));
  return handleResponse(res);
}

export async function fetchGroups({ page = 1, countPerPage = 10 } = {}) {
  const res = await fetch(apiUrl('/groups', { page, count_per_page: countPerPage }));
  return handleResponse(res);
}

export async function createGroup(payload) {
  const res = await fetch(`${BASE_URL}/group`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  return handleResponse(res);
}

export async function updateGroup(uuid, payload) {
  const res = await fetch(`${BASE_URL}/group/${encodeURIComponent(uuid)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  return handleResponse(res);
}

export async function deleteGroup(uuid) {
  const res = await fetch(`${BASE_URL}/group/${encodeURIComponent(uuid)}`, { method: 'DELETE' });
  return handleResponse(res);
}

export async function createPolicy(payload) {
  const res = await fetch(`${BASE_URL}/policy`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  return handleResponse(res);
}

export async function updatePolicy(uuid, payload) {
  const res = await fetch(`${BASE_URL}/policy/${encodeURIComponent(uuid)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  });
  return handleResponse(res);
}

export async function deletePolicy(uuid) {
  const res = await fetch(`${BASE_URL}/policy/${encodeURIComponent(uuid)}`, { method: 'DELETE' });
  return handleResponse(res);
}

export async function fetchPolicyMachines(uuid, { response = '', page = 1, countPerPage = 10 } = {}) {
  const res = await fetch(apiUrl(`/policy/${encodeURIComponent(uuid)}/machines`, { response, page, count_per_page: countPerPage }));
  return handleResponse(res);
}

// Machines
export async function fetchMachines({ page = 1, countPerPage = 100 } = {}) {
  const res = await fetch(apiUrl('/machines', { page, count_per_page: countPerPage }));
  return handleResponse(res);
}

export async function fetchMachine(id) {
  const res = await fetch(`${BASE_URL}/machines/${encodeURIComponent(id)}`);
  return handleResponse(res);
}

export async function fetchMachineQueries(id, { page = 1, countPerPage = 10 } = {}) {
  const res = await fetch(apiUrl(`/machines/${encodeURIComponent(id)}/queries`, { page, count_per_page: countPerPage }));
  return handleResponse(res);
}

export async function deleteMachineQuery(machineId, queryId) {
  const res = await fetch(`${BASE_URL}/machines/${encodeURIComponent(machineId)}/queries/${encodeURIComponent(queryId)}`, {
    method: 'DELETE',
  });
  return handleResponse(res);
}

export async function fetchMachinePolicies(id) {
  const res = await fetch(`${BASE_URL}/machines/${encodeURIComponent(id)}/policies`);
  return handleResponse(res);
}

export async function fetchMachineGroups(id) {
  const res = await fetch(`${BASE_URL}/machines/${encodeURIComponent(id)}/groups`);
  return handleResponse(res);
}

export async function updateMachineGroups(id, group_ids) {
  const res = await fetch(`${BASE_URL}/machines/${encodeURIComponent(id)}/groups`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ group_ids }),
  });
  return handleResponse(res);
}

export async function executeMachineQuery(id, query) {
  const res = await fetch(`${BASE_URL}/machines/${encodeURIComponent(id)}/query`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ query }),
  });
  return handleResponse(res);
}

// Packs
export async function fetchPacks() {
  const res = await fetch(`${BASE_URL}/packs`);
  return handleResponse(res);
}
