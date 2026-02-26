import type {
  Task, TaskReview, Thread, Crew, Execution, ExecutionEvent, AgentRun,
  StatusResponse, ServerInfoResponse, DiffResponse, ChatMessage,
  KnownCluster, CreateTaskInput, CreateCrewInput, CreateThreadInput,
  ClarificationQuestion,
} from './types'

// Re-export so consumers can import from one place
export type { ClarificationQuestion }

// Resolve base URL - Electron provides it via IPC, otherwise use localhost
let BASE_URL = 'http://localhost:8742'
let resolved = false

// Resolve base URL synchronously if possible, async as fallback.
function initBaseUrl(): void {
  if (typeof window !== 'undefined' && window.electronAPI?.getApiUrl) {
    // getApiUrl returns a promise, but we also check for a sync value.
    window.electronAPI.getApiUrl().then((url: string) => {
      BASE_URL = url
      resolved = true
    }).catch(() => {
      resolved = true
    })
  } else {
    resolved = true
  }
}

initBaseUrl()

export function getBaseUrl(): string {
  return BASE_URL
}

/** Wait for the base URL to be resolved. Use before first SSE connection. */
export async function waitForBaseUrl(): Promise<string> {
  if (resolved) return BASE_URL
  // Poll briefly — the Electron IPC resolves in <50ms typically.
  for (let i = 0; i < 20; i++) {
    await new Promise(r => setTimeout(r, 10))
    if (resolved) return BASE_URL
  }
  return BASE_URL
}

class ApiError extends Error {
  constructor(public status: number, public statusText: string, public body: string) {
    super(`API ${status} ${statusText}: ${body}`)
    this.name = 'ApiError'
  }
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const url = `${BASE_URL}${path}`
  const headers: Record<string, string> = {
    ...(options.headers as Record<string, string>),
  }
  if (options.body && typeof options.body === 'string') {
    headers['Content-Type'] = 'application/json'
  }
  const res = await fetch(url, { ...options, headers })
  if (!res.ok) {
    const body = await res.text().catch(() => '')
    throw new ApiError(res.status, res.statusText, body)
  }
  if (res.status === 204) return undefined as T
  return res.json()
}

const get = <T>(path: string) => request<T>(path)
const post = <T>(path: string, body?: unknown) =>
  request<T>(path, { method: 'POST', body: body !== undefined ? JSON.stringify(body) : undefined })
const put = <T>(path: string, body?: unknown) =>
  request<T>(path, { method: 'PUT', body: body !== undefined ? JSON.stringify(body) : undefined })
const del = <T>(path: string) => request<T>(path, { method: 'DELETE' })

export const api = {
  status: {
    get: () => get<StatusResponse>('/api/status'),
  },
  server: {
    info: () => get<ServerInfoResponse>('/api/server/info'),
  },
  clusters: {
    list: () => get<KnownCluster[]>('/api/clusters'),
    open: (path: string) => post<{ ok: boolean }>('/api/clusters/open', { path }),
    init: (path: string, name: string) =>
      post<{ ok: boolean }>('/api/clusters/init', { path, name }),
    delete: (path: string) => post<{ ok: boolean }>('/api/clusters/delete', { path }),
  },
  tasks: {
    list: () => get<Task[]>('/api/tasks'),
    get: (id: number) => get<Task>(`/api/tasks/${id}`),
    create: (input: CreateTaskInput) => post<Task>('/api/tasks', input),
    reviews: (id: number) => get<TaskReview[]>(`/api/tasks/${id}/reviews`),
    submitReview: (id: number, optionId: string) =>
      post<{ ok: boolean }>(`/api/tasks/${id}/review`, { option_id: optionId }),
    submitClarifications: (id: number, answers: Record<string, string>) =>
      post<{ ok: boolean }>(`/api/tasks/${id}/clarifications`, answers),
  },
  executions: {
    list: () => get<Execution[]>('/api/executions'),
    get: (id: number) => get<Execution>(`/api/executions/${id}`),
    events: (id: number) => get<ExecutionEvent[]>(`/api/executions/${id}/events`),
    runs: (id: number) => get<AgentRun[]>(`/api/executions/${id}/runs`),
  },
  diff: {
    get: (id: number) => get<DiffResponse>(`/api/diff/${id}`),
    commit: (id: number, message?: string) =>
      post<{ ok: boolean }>(`/api/diff/${id}/commit`, { message }),
    revert: (id: number) => post<{ ok: boolean }>(`/api/diff/${id}/revert`, {}),
    merge: (id: number) => post<{ ok: boolean; message: string }>(`/api/diff/${id}/merge`, {}),
  },
  crews: {
    list: () => get<Crew[]>('/api/crews'),
    create: (input: CreateCrewInput) => post<Crew>('/api/crews', input),
    update: (id: number, input: CreateCrewInput) => put<Crew>(`/api/crews/${id}`, input),
    delete: (id: number) => del<{ ok: boolean }>(`/api/crews/${id}`),
  },
  threads: {
    list: () => get<Thread[]>('/api/threads'),
    create: (input: CreateThreadInput) => post<Thread>('/api/threads', input),
  },
  branches: {
    list: () => get<{ branches: string[] }>('/api/branches'),
  },
  brain: {
    get: () => get<{ brain: string }>('/api/brain'),
    save: (brain: string) => put<{ ok: boolean }>('/api/brain', { brain }),
    scan: () => post<{ brain: string }>('/api/brain/scan', {}),
  },
  commander: {
    chat: (history: ChatMessage[], message: string) =>
      post<{ response: string }>('/api/commander/chat', { history, message }),
  },
}

export { ApiError }
