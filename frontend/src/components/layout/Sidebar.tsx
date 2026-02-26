import { useState } from 'react'
import { NavLink, Link, useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import {
  LayoutDashboard, Brain, Settings, ChevronLeft, ChevronRight,
  MessageSquare,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { api } from '@/lib/api'

const NAV_BASE = 'flex items-center gap-3 rounded-md px-3 py-2 text-sm font-medium text-zinc-400 transition-colors hover:bg-zinc-800 hover:text-zinc-100'
const NAV_ACTIVE = 'bg-zinc-800 text-zinc-100'

export function Sidebar() {
  const [collapsed, setCollapsed] = useState(false)
  const navigate = useNavigate()

  const { data: status } = useQuery({
    queryKey: ['status'],
    queryFn: api.status.get,
    refetchInterval: 5000,
  })

  const { data: threads } = useQuery({
    queryKey: ['threads'],
    queryFn: api.threads.list,
    enabled: status?.has_cluster ?? false,
  })

  const hasCluster = status?.has_cluster ?? false

  return (
    <aside
      className={cn(
        'flex flex-col border-r border-zinc-800 bg-zinc-950 transition-all duration-300 flex-shrink-0',
        collapsed ? 'w-16' : 'w-64'
      )}
    >
      {/* Logo */}
      <div
        className={cn(
          'flex items-center h-14 px-4 border-b border-zinc-800',
          collapsed ? 'justify-center' : 'justify-between'
        )}
      >
        {!collapsed && (
          <Link to="/" className="flex items-center gap-2">
            <span className="text-lg font-bold tracking-tight text-zinc-100">BORE</span>
            <span className="text-xs font-medium text-zinc-500">Engine</span>
          </Link>
        )}
        <button
          onClick={() => setCollapsed(c => !c)}
          className="text-zinc-400 hover:text-zinc-100 transition-colors"
          title={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
        >
          {collapsed ? <ChevronRight className="h-4 w-4" /> : <ChevronLeft className="h-4 w-4" />}
        </button>
      </div>

      {/* Nav */}
      <nav className="flex-1 flex flex-col overflow-hidden py-4 gap-1 px-2">
        <NavLink
          to="/dashboard"
          className={({ isActive }) => cn(NAV_BASE, isActive && NAV_ACTIVE)}
          title="Dashboard"
        >
          <LayoutDashboard className="h-4 w-4 flex-shrink-0" />
          {!collapsed && <span>Dashboard</span>}
        </NavLink>

        {hasCluster && (
          <NavLink
            to="/commander"
            className={({ isActive }) => cn(NAV_BASE, isActive && NAV_ACTIVE)}
            title="Commander"
          >
            <Brain className="h-4 w-4 flex-shrink-0" />
            {!collapsed && <span>Commander</span>}
          </NavLink>
        )}

        <NavLink
          to="/settings"
          className={({ isActive }) => cn(NAV_BASE, isActive && NAV_ACTIVE)}
          title="Settings"
        >
          <Settings className="h-4 w-4 flex-shrink-0" />
          {!collapsed && <span>Settings</span>}
        </NavLink>

        {/* Threads section */}
        {hasCluster && !collapsed && threads && threads.length > 0 && (
          <div className="mt-4">
            <div className="px-3 py-1 text-xs font-semibold text-zinc-500 uppercase tracking-wider">
              Threads
            </div>
            {threads.map(t => (
              <button
                key={t.id}
                onClick={() => navigate(`/dashboard?thread=${t.id}`)}
                className="w-full text-left flex items-center gap-2 px-3 py-1.5 text-xs text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800 rounded-md transition-colors"
              >
                <MessageSquare className="h-3 w-3 flex-shrink-0" />
                <span className="truncate">{t.name}</span>
              </button>
            ))}
          </div>
        )}
      </nav>

      {/* Cluster status dot */}
      {!collapsed && (
        <div className="px-4 py-3 border-t border-zinc-800">
          <div className="flex items-center gap-2">
            <div className={cn('h-2 w-2 rounded-full', hasCluster ? 'bg-emerald-500' : 'bg-zinc-600')} />
            <span className="text-xs text-zinc-400 truncate">
              {hasCluster ? (status?.cluster?.name ?? 'Connected') : 'No cluster'}
            </span>
          </div>
        </div>
      )}
    </aside>
  )
}
