import { Outlet } from 'react-router-dom'
import { Sidebar } from './Sidebar'
import { useWebSocket } from '../hooks/useWebSocket'

export function Layout() {
  useWebSocket()

  return (
    <div className="flex h-screen overflow-hidden">
      <Sidebar />
      <main className="flex-1 overflow-y-auto bg-void">
        <div className="p-6 max-w-[1600px] mx-auto">
          <Outlet />
        </div>
      </main>
    </div>
  )
}
