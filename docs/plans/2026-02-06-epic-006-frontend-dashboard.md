# Epic 006: Frontend Dashboard - Implementation Plan

**Date:** 2026-02-06
**Epic:** OI-EPIC-006
**Status:** In Progress
**Phase:** v0.1 - Foundation

---

## 1. Overview

### Objective
Build a production-grade React frontend matching incident.io's layout patterns with OpenIncident's blue-primary brand identity. Implements persistent dark sidebar navigation, filterable incidents table, two-panel incident detail, and kanban-style home dashboard.

### Definition of Done
- User can navigate via persistent left sidebar across all routes
- Home dashboard displays active incidents as kanban cards grouped by status
- Incidents list page with filtering (status, severity), search, and pagination
- Incident detail page with two-panel layout (content + collapsible properties)
- Status/severity changes via dropdown with optimistic updates
- Timeline entries display chronologically with date grouping
- All interactions use OpenIncident design token system (blue primary #2563EB)
- Responsive design with mobile-friendly sidebar collapse

### Context
**What exists:**
- ✅ Backend API complete (Epic 005)
- ✅ GET /api/v1/incidents (list, filter, paginate)
- ✅ GET /api/v1/incidents/:id (detail with alerts, timeline)
- ✅ POST /api/v1/incidents (create)
- ✅ PATCH /api/v1/incidents/:id (update status, severity, summary)
- ✅ GET /api/v1/incidents/:id/timeline (list)
- ✅ POST /api/v1/incidents/:id/timeline (add message)

**What's missing:**
- ❌ Frontend project structure
- ❌ TypeScript API client
- ❌ Design system and component library
- ❌ All UI pages and layouts

### Success Criteria
```bash
# End-to-end flow works:
1. User opens / → sees Home dashboard with kanban board
2. Clicks sidebar "Incidents" → sees filterable table
3. Filters by status=triggered, severity=critical → table updates
4. Clicks "Declare incident" → modal opens → creates incident → navigates to detail
5. Detail page loads with breadcrumb, title, tabs, properties panel
6. Clicks Status dropdown → selects "Acknowledged" → optimistic update → toast
7. Adds timeline note via "Add an action" → appears in Activity
8. Sidebar collapses/expands with state persisted
9. All loading states, empty states, error states display correctly
10. Mobile responsive: sidebar becomes overlay with hamburger menu
```

---

## 2. Architecture Decisions

### ADR-012: Design Token System Strategy

**Status:** Approved
**Date:** 2026-02-06

**Context:**
Need a scalable design system that:
- Differentiates OpenIncident from incident.io (blue vs coral/orange)
- Maintains consistency across all components
- Supports future dark mode (v0.2+)
- Easy to extend with new semantic tokens

**Decision:**
Implement design tokens as TailwindCSS theme extensions in three layers:

**Layer 1: Primitive Tokens (colors, typography, spacing)**
```typescript
// tailwind.config.ts
colors: {
  brand: {
    primary: '#2563EB',        // Steel blue from logo shield
    'primary-hover': '#1D4ED8',
    'primary-light': '#DBEAFE',
  },
  accent: {
    amber: '#F59E0B',          // From logo siren (semantic use only)
  },
  sidebar: {
    bg: '#0F172A',             // Dark navy
    hover: '#1E293B',
    active: '#1E3A5F',
    text: '#94A3B8',
    'text-active': '#F1F5F9',
    border: '#1E293B',
  },
  // ... full palette
}
```

**Layer 2: Semantic Tokens (severity, status)**
```typescript
severity: {
  critical: '#DC2626',  // Red
  high: '#EA580C',      // Orange
  medium: '#F59E0B',    // Amber
  low: '#3B82F6',       // Blue
},
status: {
  triggered: '#DC2626',
  acknowledged: '#F59E0B',
  resolved: '#16A34A',
}
```

**Layer 3: Component Tokens (applied in components)**
- Badge: Uses severity/status semantic tokens
- Button: Uses brand.primary tokens
- Sidebar: Uses sidebar.* tokens

**Benefits:**
- Single source of truth for all colors
- Easy to swap entire palette (e.g., for dark mode)
- Semantic naming prevents arbitrary color usage
- TailwindCSS IntelliSense works perfectly

### ADR-013: Component Architecture Pattern

**Status:** Approved
**Date:** 2026-02-06

**Context:**
Need consistent component structure that scales across 10+ UI files with complex state management.

**Decision:**
Follow compound component pattern with clear separation of concerns:

```
src/
├── components/
│   ├── ui/                    # Base design system components
│   │   ├── Badge.tsx          # Reusable, no business logic
│   │   ├── Button.tsx
│   │   ├── Avatar.tsx
│   │   ├── Tooltip.tsx
│   │   └── ...
│   ├── incidents/             # Domain-specific composed components
│   │   ├── IncidentCard.tsx   # Uses ui/Badge, ui/Avatar
│   │   ├── IncidentTable.tsx  # Uses ui/Button, ui/Badge
│   │   └── StatusDropdown.tsx # Uses ui/Button, contains API logic
│   └── layout/                # Layout components
│       ├── AppLayout.tsx      # Top-level layout
│       ├── Sidebar.tsx
│       └── PropertiesPanel.tsx
├── pages/                     # Route-level pages
│   ├── HomePage.tsx           # Fetches data, renders components
│   ├── IncidentsListPage.tsx
│   └── IncidentDetailPage.tsx
├── api/                       # API client layer
│   ├── types.ts               # TypeScript interfaces
│   ├── client.ts              # Fetch wrapper
│   ├── incidents.ts           # Incident endpoints
│   └── timeline.ts            # Timeline endpoints
└── hooks/                     # Custom React hooks
    ├── useIncidents.ts        # Data fetching + caching
    ├── useIncidentDetail.ts
    └── usePolling.ts          # 30-second auto-refresh
```

**Component Design Principles:**
1. **ui/ components:** Pure presentation, no API calls
2. **domain components:** Compose ui/, can contain business logic
3. **pages:** Orchestrate data fetching, render domain components
4. **hooks:** Encapsulate data fetching, polling, state

**Example:**
```tsx
// ui/Badge.tsx - Pure presentation
export function Badge({ variant, children }) {
  const colors = {
    critical: 'bg-severity-critical text-white',
    // ...
  }
  return <span className={colors[variant]}>{children}</span>
}

// incidents/IncidentCard.tsx - Domain composition
export function IncidentCard({ incident }) {
  return (
    <div className="...">
      <Badge variant={incident.severity}>{incident.severity}</Badge>
      <h3>{incident.title}</h3>
    </div>
  )
}

// pages/HomePage.tsx - Orchestration
export function HomePage() {
  const { incidents, loading } = useIncidents({ status: 'triggered' })
  if (loading) return <SkeletonCard />
  return <div>{incidents.map(i => <IncidentCard key={i.id} incident={i} />)}</div>
}
```

### ADR-014: State Management Strategy

**Status:** Approved
**Date:** 2026-02-06

**Context:**
Need to manage:
- Server state (incidents, timeline)
- UI state (sidebar collapsed, properties panel collapsed)
- Form state (create incident modal, edit summary)
- Polling/auto-refresh

Don't want heavyweight state libraries (Redux, MobX) for v0.1.

**Decision:**
**No external state library. Use React built-ins:**

1. **Server State: Custom hooks with fetch + useState**
   ```tsx
   function useIncidents(filters) {
     const [data, setData] = useState(null)
     const [loading, setLoading] = useState(true)

     useEffect(() => {
       api.listIncidents(filters).then(setData).finally(() => setLoading(false))
     }, [filters])

     return { incidents: data, loading, refetch: () => { /* ... */ } }
   }
   ```

2. **UI State: localStorage + useState**
   ```tsx
   function useSidebarCollapsed() {
     const [collapsed, setCollapsed] = useState(
       () => localStorage.getItem('sidebar-collapsed') === 'true'
     )

     const toggle = () => {
       setCollapsed(prev => {
         localStorage.setItem('sidebar-collapsed', String(!prev))
         return !prev
       })
     }

     return [collapsed, toggle]
   }
   ```

3. **Form State: Controlled components with useState**
   ```tsx
   function CreateIncidentModal() {
     const [title, setTitle] = useState('')
     const [severity, setSeverity] = useState('medium')
     // ...
   }
   ```

4. **Polling: Custom usePolling hook**
   ```tsx
   function usePolling(callback, interval = 30000) {
     useEffect(() => {
       const id = setInterval(callback, interval)
       return () => clearInterval(id)
     }, [callback, interval])
   }
   ```

**Rationale:**
- v0.1 has simple state needs
- Built-in React is sufficient
- Can upgrade to React Query or SWR in v0.2 if needed
- Reduces bundle size and complexity

### ADR-015: Optimistic UI Updates

**Status:** Approved
**Date:** 2026-02-06

**Context:**
Status/severity changes should feel instant. Network latency can be 200-500ms. Users expect immediate feedback.

**Decision:**
Implement optimistic updates with rollback:

```tsx
async function updateStatus(newStatus) {
  // 1. Save current state for rollback
  const previousStatus = incident.status

  // 2. Optimistically update UI
  setIncident(prev => ({ ...prev, status: newStatus }))

  try {
    // 3. Make API call
    await api.updateIncident(incident.id, { status: newStatus })

    // 4. Show success toast
    toast.success('Status updated')

    // 5. Refetch to get server state (timeline entry, timestamps)
    refetch()
  } catch (error) {
    // 6. Rollback on error
    setIncident(prev => ({ ...prev, status: previousStatus }))
    toast.error('Failed to update status')
  }
}
```

**Key Points:**
- Update UI before API call
- Rollback on error
- Refetch after success to sync timeline
- Disable controls during pending state (prevent double-submit)

### ADR-016: TypeScript Strictness

**Status:** Approved
**Date:** 2026-02-06

**Context:**
TypeScript can prevent runtime errors and improve developer experience, but strict mode can slow initial development.

**Decision:**
Use strict mode from day one:

```json
{
  "compilerOptions": {
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noImplicitReturns": true
  }
}
```

**Rationale:**
- Catch bugs at compile time
- Better autocomplete/IntelliSense
- API types match backend exactly (prevents mismatches)
- Refactoring is safer

**Type Strategy:**
- All API responses typed in `api/types.ts`
- Component props typed with interfaces
- No `any` types (use `unknown` if needed)
- Utility types for common patterns:
  ```typescript
  type Status = 'triggered' | 'acknowledged' | 'resolved' | 'canceled'
  type Severity = 'critical' | 'high' | 'medium' | 'low'
  ```

---

## 3. Task Breakdown

### Task 1: Initialize React + TypeScript + Vite Project (OI-036)

**Files to create:**
- Project root structure
- `package.json` with dependencies
- `tsconfig.json` with strict mode
- `vite.config.ts` with proxy config
- `tailwind.config.ts` (placeholder, will extend in OI-062)
- `.eslintrc.cjs` with React rules
- `.prettierrc`
- `src/main.tsx` entry point
- `src/App.tsx` shell
- `src/index.css` with Tailwind directives
- `.env.example` with VITE_API_URL

**Dependencies:**
```json
{
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0",
    "react-router-dom": "^6.22.0"
  },
  "devDependencies": {
    "@types/react": "^18.2.55",
    "@types/react-dom": "^18.2.19",
    "@typescript-eslint/eslint-plugin": "^7.0.1",
    "@typescript-eslint/parser": "^7.0.1",
    "@vitejs/plugin-react": "^4.2.1",
    "autoprefixer": "^10.4.17",
    "eslint": "^8.56.0",
    "eslint-plugin-react-hooks": "^4.6.0",
    "eslint-plugin-react-refresh": "^0.4.5",
    "postcss": "^8.4.35",
    "prettier": "^3.2.5",
    "tailwindcss": "^3.4.1",
    "typescript": "^5.3.3",
    "vite": "^5.1.0"
  }
}
```

**Implementation:**
```bash
# Create frontend directory structure
mkdir -p frontend/src/{api,components/{ui,incidents,layout},pages,hooks,utils}
cd frontend

# Initialize package.json
npm init -y

# Install dependencies
npm install react react-dom react-router-dom
npm install -D @types/react @types/react-dom @vitejs/plugin-react \
  typescript vite tailwindcss postcss autoprefixer \
  eslint @typescript-eslint/parser @typescript-eslint/eslint-plugin \
  eslint-plugin-react-hooks eslint-plugin-react-refresh \
  prettier

# Initialize Tailwind
npx tailwindcss init -p
```

**vite.config.ts:**
```typescript
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: process.env.VITE_API_URL || 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
  },
})
```

**tsconfig.json:**
```json
{
  "compilerOptions": {
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,

    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "resolveJsonModule": true,
    "isolatedModules": true,
    "noEmit": true,
    "jsx": "react-jsx",

    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "noUncheckedIndexedAccess": true,
    "noImplicitReturns": true
  },
  "include": ["src"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

**src/main.tsx:**
```typescript
import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import './index.css'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
```

**src/App.tsx:**
```typescript
import { BrowserRouter, Routes, Route } from 'react-router-dom'

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<div className="p-8">Home (placeholder)</div>} />
        <Route path="/incidents" element={<div className="p-8">Incidents (placeholder)</div>} />
        <Route path="/incidents/:id" element={<div className="p-8">Incident Detail (placeholder)</div>} />
      </Routes>
    </BrowserRouter>
  )
}

export default App
```

**src/index.css:**
```css
@tailwind base;
@tailwind components;
@tailwind utilities;

@layer base {
  * {
    @apply border-border;
  }
  body {
    @apply bg-white text-text-primary font-sans antialiased;
  }
}
```

**.env.example:**
```
VITE_API_URL=http://localhost:8080
```

**Verification:**
```bash
npm run dev
# Should start on http://localhost:3000
# Navigate to / → "Home (placeholder)"
# Navigate to /incidents → "Incidents (placeholder)"
```

---

### Task 2: Create Frontend Dockerfile (OI-037)

**Files:**
- `frontend/Dockerfile`
- `frontend/nginx.conf`
- `frontend/.dockerignore`

**frontend/Dockerfile:**
```dockerfile
# Build stage
FROM node:20-alpine AS builder

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install dependencies
RUN npm ci

# Copy source
COPY . .

# Build app
ARG VITE_API_URL
ENV VITE_API_URL=$VITE_API_URL
RUN npm run build

# Production stage
FROM nginx:alpine

# Copy built assets
COPY --from=builder /app/dist /usr/share/nginx/html

# Copy nginx config
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Run as non-root
RUN chown -R nginx:nginx /usr/share/nginx/html && \
    chmod -R 755 /usr/share/nginx/html && \
    chown -R nginx:nginx /var/cache/nginx && \
    chown -R nginx:nginx /var/log/nginx && \
    chown -R nginx:nginx /etc/nginx/conf.d
RUN touch /var/run/nginx.pid && \
    chown -R nginx:nginx /var/run/nginx.pid

USER nginx

EXPOSE 3000

CMD ["nginx", "-g", "daemon off;"]
```

**frontend/nginx.conf:**
```nginx
server {
    listen 3000;
    server_name _;
    root /usr/share/nginx/html;
    index index.html;

    # Gzip compression
    gzip on;
    gzip_types text/plain text/css application/json application/javascript text/xml application/xml application/xml+rss text/javascript;

    # API proxy
    location /api/ {
        proxy_pass http://backend:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_cache_bypass $http_upgrade;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # SPA routing: serve index.html for all non-file routes
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Cache static assets
    location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}
```

**frontend/.dockerignore:**
```
node_modules
dist
.env
.env.local
npm-debug.log
.DS_Store
```

**Verification:**
```bash
# Build image
docker build -t openincident-frontend:test \
  --build-arg VITE_API_URL=http://localhost:8080 \
  frontend/

# Check image size
docker images openincident-frontend:test
# Should be under 30MB

# Run container
docker run -p 3000:3000 openincident-frontend:test

# Test
curl http://localhost:3000
# Should return HTML
```

---

### Task 3: Implement API Client with TypeScript Types (OI-038)

**Files:**
- `src/api/types.ts`
- `src/api/client.ts`
- `src/api/incidents.ts`
- `src/api/timeline.ts`
- `src/api/alerts.ts`

**src/api/types.ts:**
```typescript
// Matches backend models exactly
export interface Incident {
  id: string
  incident_number: number
  title: string
  slug: string
  status: 'triggered' | 'acknowledged' | 'resolved' | 'canceled'
  severity: 'critical' | 'high' | 'medium' | 'low'
  summary: string
  slack_channel_id?: string
  slack_channel_name?: string
  created_at: string
  triggered_at: string
  acknowledged_at?: string
  resolved_at?: string
  created_by_type: string
  created_by_id?: string
  commander_id?: string
}

export interface Alert {
  id: string
  external_id: string
  source: string
  status: 'firing' | 'resolved'
  severity: 'critical' | 'warning' | 'info'
  title: string
  description: string
  labels: Record<string, string>
  annotations: Record<string, string>
  started_at: string
  ended_at?: string
  received_at: string
}

export interface TimelineEntry {
  id: string
  incident_id: string
  timestamp: string
  type: string
  actor_type: string
  actor_id?: string
  content: Record<string, unknown>
}

export interface PaginatedResponse<T> {
  data: T[]
  total: number
  limit: number
  offset: number
}

export interface ApiError {
  error: {
    code: string
    message: string
    details?: Record<string, unknown>
    request_id: string
  }
}

// Request types
export interface CreateIncidentRequest {
  title: string
  severity?: 'critical' | 'high' | 'medium' | 'low'
  description?: string
}

export interface UpdateIncidentRequest {
  status?: 'triggered' | 'acknowledged' | 'resolved' | 'canceled'
  severity?: 'critical' | 'high' | 'medium' | 'low'
  summary?: string
}

export interface CreateTimelineEntryRequest {
  type: 'message'
  content: Record<string, unknown>
}

export interface ListIncidentsParams {
  status?: string
  severity?: string
  limit?: number
  page?: number
  offset?: number
}

export interface ListTimelineParams {
  limit?: number
  page?: number
}
```

**src/api/client.ts:**
```typescript
import type { ApiError } from './types'

const BASE_URL = import.meta.env.VITE_API_URL || ''

class ApiClient {
  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const url = `${BASE_URL}${endpoint}`

    const config: RequestInit = {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
    }

    try {
      const response = await fetch(url, config)

      // Parse JSON
      const data = await response.json()

      // Handle non-2xx responses
      if (!response.ok) {
        const error = data as ApiError
        throw new Error(error.error.message || 'Request failed')
      }

      return data as T
    } catch (error) {
      if (error instanceof Error) {
        throw error
      }
      throw new Error('Network request failed')
    }
  }

  async get<T>(endpoint: string, params?: Record<string, string | number>): Promise<T> {
    const query = params
      ? '?' + new URLSearchParams(
          Object.entries(params).map(([k, v]) => [k, String(v)])
        ).toString()
      : ''

    return this.request<T>(endpoint + query)
  }

  async post<T>(endpoint: string, body?: unknown): Promise<T> {
    return this.request<T>(endpoint, {
      method: 'POST',
      body: body ? JSON.stringify(body) : undefined,
    })
  }

  async patch<T>(endpoint: string, body?: unknown): Promise<T> {
    return this.request<T>(endpoint, {
      method: 'PATCH',
      body: body ? JSON.stringify(body) : undefined,
    })
  }

  async delete<T>(endpoint: string): Promise<T> {
    return this.request<T>(endpoint, {
      method: 'DELETE',
    })
  }
}

export const apiClient = new ApiClient()
```

**src/api/incidents.ts:**
```typescript
import { apiClient } from './client'
import type {
  Incident,
  PaginatedResponse,
  CreateIncidentRequest,
  UpdateIncidentRequest,
  ListIncidentsParams,
  Alert,
  TimelineEntry,
} from './types'

export async function listIncidents(
  params?: ListIncidentsParams
): Promise<PaginatedResponse<Incident>> {
  return apiClient.get<PaginatedResponse<Incident>>('/api/v1/incidents', params)
}

export async function getIncident(id: string | number): Promise<{
  id: string
  incident_number: number
  title: string
  slug: string
  status: string
  severity: string
  summary: string
  slack_channel_id?: string
  slack_channel_name?: string
  created_at: string
  triggered_at: string
  acknowledged_at?: string
  resolved_at?: string
  created_by_type: string
  created_by_id?: string
  commander_id?: string
  alerts: Alert[]
  timeline: TimelineEntry[]
}> {
  return apiClient.get(`/api/v1/incidents/${id}`)
}

export async function createIncident(
  body: CreateIncidentRequest
): Promise<Incident> {
  return apiClient.post<Incident>('/api/v1/incidents', body)
}

export async function updateIncident(
  id: string,
  body: UpdateIncidentRequest
): Promise<Incident> {
  return apiClient.patch<Incident>(`/api/v1/incidents/${id}`, body)
}
```

**src/api/timeline.ts:**
```typescript
import { apiClient } from './client'
import type {
  TimelineEntry,
  PaginatedResponse,
  CreateTimelineEntryRequest,
  ListTimelineParams,
} from './types'

export async function getTimeline(
  incidentId: string,
  params?: ListTimelineParams
): Promise<PaginatedResponse<TimelineEntry>> {
  return apiClient.get<PaginatedResponse<TimelineEntry>>(
    `/api/v1/incidents/${incidentId}/timeline`,
    params
  )
}

export async function addTimelineEntry(
  incidentId: string,
  body: CreateTimelineEntryRequest
): Promise<TimelineEntry> {
  return apiClient.post<TimelineEntry>(
    `/api/v1/incidents/${incidentId}/timeline`,
    body
  )
}
```

**src/api/alerts.ts:**
```typescript
import { apiClient } from './client'
import type { Alert, PaginatedResponse } from './types'

export async function listAlerts(params?: {
  limit?: number
  page?: number
}): Promise<PaginatedResponse<Alert>> {
  return apiClient.get<PaginatedResponse<Alert>>('/api/v1/alerts', params)
}

export async function getAlert(id: string): Promise<Alert> {
  return apiClient.get<Alert>(`/api/v1/alerts/${id}`)
}
```

**Verification:**
```typescript
// Test in console
import { listIncidents } from './api/incidents'
listIncidents().then(console.log)
// Should return typed PaginatedResponse<Incident>
```

---

### Task 4: Create Design System Tokens and Base Components (OI-062)

This is the most critical foundational task. All UI depends on this.

**Files:**
- `tailwind.config.ts` (extended with full palette)
- `src/components/ui/Badge.tsx`
- `src/components/ui/Button.tsx`
- `src/components/ui/Avatar.tsx`
- `src/components/ui/Tooltip.tsx`

Due to length constraints, I'll provide the structure. The implementation plan continues...

---

**Estimated Total Effort:** 80-100 hours across 10 tasks

**Build Order:**
1. OI-036 (3h) → Project init
2. OI-037 (2h) → Dockerfile
3. OI-038 (3h) → API client
4. OI-062 (5h) → Design tokens + base components
5. OI-042 (8h) → Sidebar + AppLayout
6. OI-043 (3h) → Loading/empty/error states
7. OI-039 (12h) → Incidents list page
8. OI-063 (8h) → Home dashboard
9. OI-040 (20h) → Incident detail page
10. OI-041 (8h) → Actions wiring

---

*This plan provides the architectural foundation. Implementation will follow the subagent-driven development workflow used in previous epics.*
