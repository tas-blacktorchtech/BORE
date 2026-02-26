import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { MainLayout } from '@/components/layout/MainLayout'
import { Home } from '@/pages/Home'
import { Dashboard } from '@/pages/Dashboard'
import { ExecutionPage } from '@/pages/ExecutionPage'
import { TaskPage } from '@/pages/TaskPage'
import { CommanderPage } from '@/pages/CommanderPage'
import { Settings } from '@/pages/Settings'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<MainLayout />}>
          <Route index element={<Home />} />
          <Route path="dashboard" element={<Dashboard />} />
          <Route path="tasks/:id" element={<TaskPage />} />
          <Route path="executions/:id" element={<ExecutionPage />} />
          <Route path="commander" element={<CommanderPage />} />
          <Route path="settings" element={<Settings />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
