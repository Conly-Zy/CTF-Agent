import { Routes, Route, NavLink, Navigate } from 'react-router-dom'
import { LayoutDashboard, Terminal, History, BookOpen, Settings } from 'lucide-react'
import Dashboard from '@/pages/Dashboard'
import Solve from '@/pages/Solve'
import Sessions from '@/pages/Sessions'
import Knowledge from '@/pages/Knowledge'
import SettingsPage from '@/pages/Settings'

const navItems = [
  { to: '/dashboard', icon: LayoutDashboard, label: '仪表盘' },
  { to: '/solve', icon: Terminal, label: '解题' },
  { to: '/sessions', icon: History, label: '会话记录' },
  { to: '/knowledge', icon: BookOpen, label: '知识库' },
  { to: '/settings', icon: Settings, label: '设置' },
]

export default function App() {
  return (
    <div className="flex h-screen">
      {/* Sidebar */}
      <aside className="w-56 border-r bg-muted/30 flex flex-col shrink-0">
        <div className="h-14 flex items-center px-5 border-b">
          <span className="text-lg font-semibold tracking-tight">CTF-Agent</span>
        </div>
        <nav className="flex-1 p-3 space-y-1">
          {navItems.map(({ to, icon: Icon, label }) => (
            <NavLink
              key={to}
              to={to}
              className={({ isActive }) =>
                `flex items-center gap-3 px-3 py-2 rounded-md text-sm transition-colors ${
                  isActive
                    ? 'bg-primary text-primary-foreground'
                    : 'text-muted-foreground hover:bg-accent hover:text-accent-foreground'
                }`
              }
            >
              <Icon className="h-4 w-4" />
              {label}
            </NavLink>
          ))}
        </nav>
      </aside>

      {/* Main content */}
      <main className="flex-1 overflow-auto">
        <Routes>
          <Route path="/" element={<Navigate to="/dashboard" replace />} />
          <Route path="/dashboard" element={<Dashboard />} />
          <Route path="/solve" element={<Solve />} />
          <Route path="/sessions" element={<Sessions />} />
          <Route path="/knowledge" element={<Knowledge />} />
          <Route path="/settings" element={<SettingsPage />} />
        </Routes>
      </main>
    </div>
  )
}
