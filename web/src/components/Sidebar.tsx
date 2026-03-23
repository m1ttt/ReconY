import { NavLink, useParams } from 'react-router-dom'
import { clsx } from 'clsx'
import {
  Globe, Server, Code2, FileSearch, Cloud,
  Bug, Settings, Wrench, LayoutDashboard, List, Workflow,
  Crosshair, Eye, Cpu, History, Search, Fingerprint, Camera, Key, Shield, Waypoints
} from 'lucide-react'

const mainNav = [
  { to: '/', icon: LayoutDashboard, label: 'Workspaces' },
  { to: '/workflows', icon: Workflow, label: 'Workflows' },
  { to: '/tools', icon: Wrench, label: 'Tools' },
  { to: '/settings', icon: Settings, label: 'Settings' },
]

const workspaceNav = [
  { to: '', icon: Crosshair, label: 'Dashboard' },
  { to: '/recon', icon: Waypoints, label: 'Recon Console' },
  { to: '/scans', icon: Eye, label: 'Scans' },
  { to: '/auth', icon: Shield, label: 'Auth' },
  { to: '/subdomains', icon: Globe, label: 'Subdomains' },
  { to: '/dns', icon: Cpu, label: 'DNS Records' },
  { to: '/whois', icon: Search, label: 'WHOIS' },
  { to: '/ports', icon: Server, label: 'Ports' },
  { to: '/technologies', icon: Code2, label: 'Technologies' },
  { to: '/classifications', icon: Fingerprint, label: 'Classifications' },
  { to: '/vulnerabilities', icon: Bug, label: 'Vulnerabilities' },
  { to: '/secrets', icon: Key, label: 'Secrets' },
  { to: '/urls', icon: FileSearch, label: 'URLs' },
  { to: '/historical-urls', icon: History, label: 'Historical URLs' },
  { to: '/parameters', icon: List, label: 'Parameters' },
  { to: '/cloud', icon: Cloud, label: 'Cloud Assets' },
  { to: '/screenshots', icon: Camera, label: 'Screenshots' },
]

function SideLink({ to, icon: Icon, label, end }: {
  to: string; icon: any; label: string; end?: boolean
}) {
  return (
    <NavLink
      to={to}
      end={end}
      className={({ isActive }) => clsx(
        'flex items-center gap-3 px-3 py-2 rounded-md text-sm font-medium transition-all duration-150',
        isActive
          ? 'bg-accent/10 text-accent border-l-2 border-accent -ml-[2px] pl-[14px]'
          : 'text-muted hover:text-text hover:bg-raised/60'
      )}
    >
      <Icon size={16} strokeWidth={1.8} />
      <span>{label}</span>
    </NavLink>
  )
}

export function Sidebar() {
  const { workspaceId } = useParams()

  return (
    <aside className="w-56 h-screen flex flex-col bg-abyss border-r border-border overflow-y-auto shrink-0">
      {/* Logo */}
      <div className="px-4 py-5 border-b border-border">
        <div className="flex items-center gap-2">
          <div className="w-7 h-7 rounded bg-gradient-to-br from-accent to-accent-dim flex items-center justify-center">
            <Crosshair size={15} className="text-void" strokeWidth={2.5} />
          </div>
          <span className="font-bold text-heading tracking-tight text-lg">
            Recon<span className="text-accent">X</span>
          </span>
        </div>
        <p className="text-[10px] font-mono text-muted mt-1 tracking-wider uppercase">
          v0.1.0-dev
        </p>
      </div>

      {/* Main Navigation */}
      <nav className="px-3 py-3 space-y-0.5">
        {mainNav.map((item) => (
          <SideLink key={item.to} {...item} end={item.to === '/'} />
        ))}
      </nav>

      {/* Workspace Navigation */}
      {workspaceId && (
        <>
          <div className="px-4 pt-2 pb-1">
            <div className="text-[10px] font-mono text-muted tracking-wider uppercase">
              Workspace
            </div>
          </div>
          <nav className="px-3 pb-3 space-y-0.5 flex-1">
            {workspaceNav.map((item) => (
              <SideLink
                key={item.to}
                to={`/workspace/${workspaceId}${item.to}`}
                icon={item.icon}
                label={item.label}
                end={item.to === ''}
              />
            ))}
          </nav>
        </>
      )}

      {/* Footer */}
      <div className="px-4 py-3 border-t border-border mt-auto">
        <p className="text-[10px] font-mono text-muted/60">
          {new Date().toLocaleDateString()} &bull; ops ready
        </p>
      </div>
    </aside>
  )
}
