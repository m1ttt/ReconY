# Connection Status Header Feature

**Feature ID:** F001
**Status:** Draft
**Created:** 2025-03-23
**Author:** ReconX Team

---

## 1. Feature Overview

### 1.1 Purpose

The Connection Status Header provides users with real-time visibility into their application connectivity status. This feature displays critical connection information in a persistent header element, enabling users to quickly diagnose connectivity issues and understand their current session context.

### 1.2 User Value

- **Immediate awareness** of WebSocket connection status for real-time updates
- **Visibility of public IP address** for reconnaissance context awareness
- **Geolocation context** displayed as country code for operational awareness
- **Reduced troubleshooting time** when connection issues occur

### 1.3 Display Location

The header will be displayed in the main application layout (`web/src/components/Layout.tsx`), positioned as a horizontal bar above the main content area but below any top-level navigation.

---

## 2. Technical Requirements

### 2.1 Frontend Requirements

#### 2.1.1 New Component: `ConnectionHeader`

Create a new React component at `web/src/components/ConnectionHeader.tsx`.

**Responsibilities:**
- Display WebSocket connection status with visual indicator
- Fetch and display user's public IP address
- Display country based on IP geolocation
- Auto-refresh IP info on connection changes

**Interface:**
```typescript
interface ConnectionHeaderProps {
  // No props required - uses store and hooks internally
}

interface ConnectionStatus {
  state: 'connected' | 'disconnected' | 'reconnecting'
  lastChange: Date
}

interface IPInfo {
  ip: string
  country: string
  countryCode: string // ISO 3166-1 alpha-2
  city?: string
  isLoading: boolean
}
```

#### 2.1.2 Store Extensions

Extend `web/src/store/index.ts` to include connection state:

```typescript
interface AppStore {
  // ... existing properties ...

  // Connection state
  connectionStatus: 'connected' | 'disconnected' | 'reconnecting'
  setConnectionStatus: (status: 'connected' | 'disconnected' | 'reconnecting') => void

  // IP info state
  ipInfo: IPInfo | null
  setIPInfo: (info: IPInfo | null) => void
}
```

#### 2.1.3 WebSocket Hook Modifications

Update `web/src/hooks/useWebSocket.ts` to:
- Emit connection status changes to the store
- Track reconnection attempts
- Provide connection state to consumers

```typescript
interface UseWebSocketReturn {
  status: 'connected' | 'disconnected' | 'reconnecting'
  reconnectAttempt?: number
}
```

### 2.2 Backend Requirements

#### 2.2.1 New API Endpoint: `/api/v1/ip-info`

Create a new handler in `internal/api/server.go`:

**Endpoint:** `GET /api/v1/ip-info`

**Response:**
```json
{
  "ip": "203.0.113.42",
  "country": "United States",
  "country_code": "US",
  "city": "San Francisco",
  "is_proxy": false,
  "is_tor": false
}
```

**Implementation Details:**
- Extract client IP from `X-Real-IP` or `X-Forwarded-For` headers (handled by middleware.RealIP)
- Use IP geolocation service (recommend: freegeoip.app or local database like MaxMind GeoLite2)
- Cache results for 5 minutes per session
- Return 500 if geolocation service unavailable (graceful degradation)

#### 2.2.2 Dependencies

**Backend (Go):**
```go
// Add to go.mod
require (
    github.com/oschwald/geoip2-golang v1.5.0  // For local GeoIP lookup
    // OR use external API
)
```

**Frontend:**
```bash
# No new dependencies required - uses existing:
# - zustand (state management)
# - react (hooks)
# - lucide-react (icons)
```

---

## 3. UI/UX Specifications

### 3.1 Visual Design

The header follows the existing dark theme design system defined in `web/src/index.css`.

**Layout Structure:**
```
+---------------------------------------------------------------+
|  [•] Connected    IP: 203.0.113.42    [US] United States     |
+---------------------------------------------------------------+
```

**Component Structure:**
```tsx
<div className="h-10 bg-surface border-b border-border flex items-center px-4 gap-6 text-xs">
  <ConnectionStatusIndicator />
  <IPDisplay />
  <CountryDisplay />
</div>
```

### 3.2 Status Indicator States

| State | Icon | Color | Animation |
|-------|------|-------|-----------|
| Connected | Solid dot (`•`) | `--color-completed` (#00ff9d) | None |
| Reconnecting | Pulsing dot | `--color-medium` (#ffc233) | Pulse animation |
| Disconnected | Hollow dot/dash | `--color-failed` (#ff2b5e) | None |

**CSS Classes:**
```css
.status-indicator-connected { color: var(--color-completed); }
.status-indicator-reconnecting { color: var(--color-medium); animation: pulse-glow 1.5s infinite; }
.status-indicator-disconnected { color: var(--color-failed); }
```

### 3.3 Typography & Spacing

- **Font:** `var(--font-mono)` for IP address (JetBrains Mono)
- **Font:** `var(--font-sans)` for labels (Instrument Sans)
- **Size:** `text-xs` (12px)
- **Height:** `h-10` (40px) fixed header height
- **Padding:** `px-4` horizontal padding, flex gap of 6 for element separation

### 3.4 Responsive Design

| Breakpoint | Behavior |
|------------|----------|
| >= 768px | Full display: Status + IP + Country |
| < 768px | Condensed: Status icon + IP only (country on hover) |
| < 480px | Minimal: Status icon only (full info in tooltip) |

**Implementation:**
```tsx
<div className="hidden md:inline text-muted">IP:</div>
<span className={clsx("font-mono", ipInfo.isLoading && "animate-pulse")}>
  {ipInfo?.ip || 'Detecting...'}
</span>
```

### 3.5 Loading States

- **Initial load:** Show "Detecting..." with pulse animation
- **Error state:** Show "Unknown" with muted color, retry on click

---

## 4. Implementation Notes

### 4.1 Files to Create

| File | Purpose |
|------|---------|
| `web/src/components/ConnectionHeader.tsx` | Main header component |
| `internal/api/ip_info.go` | IP info API handler |
| `internal/geoip/geoip.go` | Geolocation service wrapper |

### 4.2 Files to Modify

| File | Changes |
|------|---------|
| `web/src/components/Layout.tsx` | Import and render ConnectionHeader |
| `web/src/store/index.ts` | Add connectionStatus and ipInfo state |
| `web/src/hooks/useWebSocket.ts` | Track and emit connection status |
| `internal/api/server.go` | Register `/api/v1/ip-info` route |
| `internal/api/routes.go` (if exists) | Add IP info handler |

### 4.3 Implementation Sequence

1. **Backend First**
   - Create geolocation service wrapper
   - Add `/api/v1/ip-info` endpoint
   - Test with curl/Postman

2. **Store & State**
   - Extend Zustand store with new state
   - Update WebSocket hook

3. **UI Component**
   - Create ConnectionHeader component
   - Add to Layout
   - Style according to design specs

4. **Integration Testing**
   - Test connection state transitions
   - Test IP geolocation accuracy
   - Test responsive behavior

### 4.4 Error Handling

- **WebSocket failures:** Gracefully show disconnected state, auto-reconnect
- **IP API failures:** Show last known IP or "Unknown", retry silently
- **Geolocation failures:** Display IP without country info

### 4.5 Security Considerations

- IP information is client-visible only (no storage)
- No logging of IP addresses on backend
- Cache in-memory only, not persisted
- Consider rate limiting for IP info endpoint

---

## 5. Acceptance Criteria

- [ ] Header displays in all pages (above main content)
- [ ] WebSocket status updates in real-time
- [ ] Public IP displays correctly on load
- [ ] Country displays based on IP geolocation
- [ ] Reconnecting state shows pulsing indicator
- [ ] Disconnected state shows red indicator
- [ ] Responsive design works on mobile (< 768px)
- [ ] No performance impact on initial load
- [ ] Auto-refreshes IP info when WebSocket reconnects
- [ ] Matches existing dark theme styling

---

## 6. Future Enhancements

- Latency indicator (ping to server)
- Toggle to show/hide header
- Click to copy IP address
- VPN/proxy detection indicator
- Region-based feature availability notices
