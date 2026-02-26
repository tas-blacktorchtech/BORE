import { useEffect, useRef, useCallback } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { waitForBaseUrl } from '@/lib/api'

export function useSSE() {
  const queryClient = useQueryClient()
  const esRef = useRef<EventSource | null>(null)
  const reconnectRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  const connect = useCallback(async () => {
    if (esRef.current) {
      esRef.current.close()
    }

    const base = await waitForBaseUrl()
    const es = new EventSource(`${base}/events`)
    esRef.current = es

    const invalidate = (keys: string[]) => {
      keys.forEach(key => queryClient.invalidateQueries({ queryKey: [key] }))
    }

    es.addEventListener('tasks_updated', () => {
      invalidate(['tasks'])
      // Also invalidate any task-review queries (prefixed keys)
      queryClient.invalidateQueries({ predicate: q => q.queryKey[0] === 'task-reviews' })
      // And individual task queries
      queryClient.invalidateQueries({ predicate: q => q.queryKey[0] === 'task' })
    })
    es.addEventListener('executions_updated', () => {
      invalidate(['executions'])
      // Also invalidate individual execution queries and their sub-resources.
      queryClient.invalidateQueries({ predicate: q => q.queryKey[0] === 'execution' })
      queryClient.invalidateQueries({ predicate: q => q.queryKey[0] === 'execution-events' })
      queryClient.invalidateQueries({ predicate: q => q.queryKey[0] === 'execution-runs' })
      queryClient.invalidateQueries({ predicate: q => q.queryKey[0] === 'diff' })
    })
    es.addEventListener('crews_updated', () => invalidate(['crews']))
    es.addEventListener('threads_updated', () => invalidate(['threads']))
    es.addEventListener('brain_updated', () => invalidate(['brain']))
    es.addEventListener('cluster_opened', () => {
      invalidate(['status', 'tasks', 'executions', 'crews', 'threads'])
    })

    es.onerror = () => {
      es.close()
      esRef.current = null
      reconnectRef.current = setTimeout(connect, 3000)
    }
  }, [queryClient])

  useEffect(() => {
    connect()
    return () => {
      esRef.current?.close()
      if (reconnectRef.current) clearTimeout(reconnectRef.current)
    }
  }, [connect])
}
