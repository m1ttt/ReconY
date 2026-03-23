import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { Layout } from './components/Layout'
import { WorkspaceList } from './pages/WorkspaceList'
import { WorkspaceDashboard } from './pages/WorkspaceDashboard'
import { ScansPage } from './pages/ScansPage'
import { SubdomainsPage } from './pages/SubdomainsPage'
import { PortsPage } from './pages/PortsPage'
import { TechnologiesPage } from './pages/TechnologiesPage'
import { VulnerabilitiesPage } from './pages/VulnerabilitiesPage'
import { SecretsPage } from './pages/SecretsPage'
import {
  URLsPage, CloudAssetsPage, ScreenshotsPage,
  DNSPage, WhoisPage, HistoricalURLsPage,
  ParametersPage, ClassificationsPage
} from './pages/GenericResultPage'
import { WorkflowsPage } from './pages/WorkflowsPage'
import { ToolsPage } from './pages/ToolsPage'
import { SettingsPage } from './pages/SettingsPage'
import { AuthConfigPage } from './pages/AuthConfigPage'
import { ReconConsolePage } from './pages/ReconConsolePage'

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          {/* Main */}
          <Route path="/" element={<WorkspaceList />} />
          <Route path="/workflows" element={<WorkflowsPage />} />
          <Route path="/tools" element={<ToolsPage />} />
          <Route path="/settings" element={<SettingsPage />} />

          {/* Workspace */}
          <Route path="/workspace/:workspaceId" element={<WorkspaceDashboard />} />
          <Route path="/workspace/:workspaceId/recon" element={<ReconConsolePage />} />
          <Route path="/workspace/:workspaceId/scans" element={<ScansPage />} />
          <Route path="/workspace/:workspaceId/subdomains" element={<SubdomainsPage />} />
          <Route path="/workspace/:workspaceId/ports" element={<PortsPage />} />
          <Route path="/workspace/:workspaceId/technologies" element={<TechnologiesPage />} />
          <Route path="/workspace/:workspaceId/vulnerabilities" element={<VulnerabilitiesPage />} />
          <Route path="/workspace/:workspaceId/secrets" element={<SecretsPage />} />
          <Route path="/workspace/:workspaceId/dns" element={<DNSPage />} />
          <Route path="/workspace/:workspaceId/whois" element={<WhoisPage />} />
          <Route path="/workspace/:workspaceId/urls" element={<URLsPage />} />
          <Route path="/workspace/:workspaceId/historical-urls" element={<HistoricalURLsPage />} />
          <Route path="/workspace/:workspaceId/parameters" element={<ParametersPage />} />
          <Route path="/workspace/:workspaceId/classifications" element={<ClassificationsPage />} />
          <Route path="/workspace/:workspaceId/cloud" element={<CloudAssetsPage />} />
          <Route path="/workspace/:workspaceId/screenshots" element={<ScreenshotsPage />} />
          <Route path="/workspace/:workspaceId/auth" element={<AuthConfigPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}
