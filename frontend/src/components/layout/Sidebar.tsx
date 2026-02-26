import { useState, useEffect } from 'react'
import { Link, useLocation } from 'react-router-dom'
import {
  Home,
  Search,
  Flame,
  Phone,
  ChevronLeft,
  ChevronRight,
  ChevronDown,
  Settings,
  Bell,
  Shield,
  GitFork,
  GitBranch,
  FileText,
  LogOut,
  Users,
} from 'lucide-react'
import { Tooltip } from '../ui/Tooltip'
import { useAuth } from '../../hooks/useAuth'
import type { CurrentUser } from '../../api/auth'

interface NavItem {
  id: string
  label: string
  icon: React.ComponentType<{ className?: string }>
  href?: string
  onClick?: () => void
  disabled?: boolean
  disabledTooltip?: string
  matchPaths?: string[]
}

interface NavSection {
  id: string
  label: string
  items: NavItem[]
  collapsible?: boolean
}

/**
 * Persistent dark left sidebar navigation matching incident.io patterns
 * Features: collapsible sections, localStorage persistence, mobile overlay
 */
export function Sidebar() {
  const location = useLocation()
  const [isCollapsed, setIsCollapsed] = useState(() => {
    const saved = localStorage.getItem('sidebar-collapsed')
    return saved === 'true'
  })
  const [isMobileOpen, setIsMobileOpen] = useState(false)
  const [sectionsExpanded, setSectionsExpanded] = useState<Record<string, boolean>>({
    organization: true,
  })
  const { user: currentUser } = useAuth()

  // Persist collapse state
  useEffect(() => {
    localStorage.setItem('sidebar-collapsed', String(isCollapsed))
  }, [isCollapsed])

  const toggleCollapse = () => setIsCollapsed(!isCollapsed)
  const toggleSection = (sectionId: string) => {
    setSectionsExpanded((prev) => ({ ...prev, [sectionId]: !prev[sectionId] }))
  }

  const showToast = (message: string) => {
    // TODO: Implement toast notification system in OI-041
    alert(message)
  }

  const topNavItems: NavItem[] = [
    {
      id: 'home',
      label: 'Home',
      icon: Home,
      href: '/',
    },
    {
      id: 'search',
      label: 'Search',
      icon: Search,
      onClick: () => showToast('Search coming soon'),
    },
  ]

  const navSections: NavSection[] = [
    {
      id: 'organization',
      label: 'Your organization',
      collapsible: true,
      items: [
        {
          id: 'incidents',
          label: 'Incidents',
          icon: Flame,
          href: '/incidents',
          matchPaths: ['/incidents', '/incidents/'],
        },
        {
          id: 'routing-rules',
          label: 'Routing Rules',
          icon: GitFork,
          href: '/routing-rules',
          matchPaths: ['/routing-rules'],
        },
        {
          id: 'oncall',
          label: 'On-call',
          icon: Phone,
          href: '/on-call',
          matchPaths: ['/on-call'],
        },
        {
          id: 'escalation-policies',
          label: 'Escalation',
          icon: GitBranch,
          href: '/escalation-policies',
          matchPaths: ['/escalation-policies'],
        },
        {
          id: 'post-mortem-templates',
          label: 'PM Templates',
          icon: FileText,
          href: '/post-mortem-templates',
          matchPaths: ['/post-mortem-templates'],
        },
      ],
    },
    {
      id: 'settings',
      label: 'Settings',
      collapsible: false,
      items: [
        {
          id: 'settings-users',
          label: 'Users',
          icon: Users,
          href: '/settings/users',
          matchPaths: ['/settings/users'],
        },
      ],
    },
  ]

  const isActive = (item: NavItem): boolean => {
    if (!item.href) return false
    if (item.matchPaths) {
      return item.matchPaths.some((path) => location.pathname.startsWith(path))
    }
    return location.pathname === item.href
  }

  const renderNavItem = (item: NavItem) => {
    const Icon = item.icon
    const active = isActive(item)

    const content = (
      <div
        className={`
          flex items-center h-10 px-3 rounded transition-colors duration-200 group
          ${active ? 'bg-sidebar-active text-sidebar-text-active' : 'text-sidebar-text hover:bg-sidebar-hover'}
          ${item.disabled ? 'opacity-50 cursor-not-allowed' : 'cursor-pointer'}
          ${active ? 'border-l-3 border-brand-primary' : ''}
        `}
        style={active ? { borderLeftWidth: '3px' } : undefined}
      >
        <Icon className={`flex-shrink-0 w-5 h-5 ${isCollapsed ? 'mx-auto' : 'mr-3'}`} />
        {!isCollapsed && <span className="text-sm font-medium">{item.label}</span>}
      </div>
    )

    if (item.disabled) {
      return (
        <Tooltip key={item.id} content={item.disabledTooltip || 'Coming soon'}>
          {content}
        </Tooltip>
      )
    }

    if (item.onClick) {
      return (
        <div key={item.id} onClick={item.onClick}>
          {isCollapsed ? <Tooltip content={item.label}>{content}</Tooltip> : content}
        </div>
      )
    }

    if (item.href) {
      const linkContent = (
        <Link key={item.id} to={item.href}>
          {content}
        </Link>
      )
      return isCollapsed ? (
        <Tooltip key={item.id} content={item.label}>
          {linkContent}
        </Tooltip>
      ) : (
        linkContent
      )
    }

    return null
  }

  const sidebarContent = (
    <div className="h-full flex flex-col bg-sidebar-bg text-sidebar-text">
      {/* Top Bar */}
      <div className="flex items-center h-14 px-3 border-b border-sidebar-border">
        {!isCollapsed && (
          <>
            <Shield className="text-brand-primary flex-shrink-0 w-6 h-6" />
            <span className="ml-2 text-sidebar-text-active font-semibold text-base">
              OpenIncident
            </span>
          </>
        )}
        {isCollapsed && <Shield className="text-brand-primary mx-auto w-6 h-6" />}
        <button
          onClick={toggleCollapse}
          className="ml-auto p-1.5 rounded hover:bg-sidebar-hover transition-colors"
          aria-label={isCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
        >
          {isCollapsed ? <ChevronRight className="w-4.5 h-4.5" /> : <ChevronLeft className="w-4.5 h-4.5" />}
        </button>
      </div>

      {/* Navigation */}
      <div className="flex-1 overflow-y-auto py-4 px-2">
        {/* Top nav items */}
        <div className="space-y-1 mb-4">{topNavItems.map(renderNavItem)}</div>

        {/* Sections */}
        {navSections.map((section) => (
          <div key={section.id} className="mb-4">
            {section.collapsible && !isCollapsed && (
              <button
                onClick={() => toggleSection(section.id)}
                className="flex items-center w-full px-3 py-2 text-xs uppercase tracking-wider text-text-tertiary hover:text-sidebar-text transition-colors"
              >
                <span className="flex-1 text-left">{section.label}</span>
                <ChevronDown
                  className={`w-3.5 h-3.5 transition-transform duration-200 ${sectionsExpanded[section.id] ? '' : '-rotate-90'}`}
                />
              </button>
            )}
            {(sectionsExpanded[section.id] || isCollapsed) && (
              <div className="space-y-1">{section.items.map(renderNavItem)}</div>
            )}
          </div>
        ))}
      </div>

      {/* Bottom Bar - User */}
      <div className="border-t border-sidebar-border">
        <div className="flex items-center h-14 px-3">
          {!isCollapsed ? (
            <>
              <UserAvatar user={currentUser} size="md" />
              <div className="ml-3 flex-1 min-w-0">
                <div className="text-sm font-medium text-sidebar-text-active truncate">
                  {userDisplayName(currentUser)}
                </div>
                {currentUser?.email && (
                  <div className="text-xs text-text-tertiary truncate">{currentUser.email}</div>
                )}
                {currentUser?.mode === 'open' && (
                  <a
                    href="/login"
                    className="text-xs text-blue-400 hover:text-blue-300 hover:underline"
                  >
                    Sign in
                  </a>
                )}
              </div>
              <button
                className="p-1.5 rounded hover:bg-sidebar-hover transition-colors ml-1"
                aria-label="Notifications"
              >
                <Bell className="w-4.5 h-4.5" />
              </button>
              {currentUser?.authenticated && (
                <Tooltip content="Sign out">
                  <a
                    href="/auth/logout"
                    className="p-1.5 rounded hover:bg-sidebar-hover transition-colors ml-1 text-sidebar-text"
                    aria-label="Sign out"
                  >
                    <LogOut className="w-4 h-4" />
                  </a>
                </Tooltip>
              )}
            </>
          ) : (
            <Tooltip content={userDisplayName(currentUser)}>
              <UserAvatar user={currentUser} size="md" className="mx-auto" />
            </Tooltip>
          )}
        </div>
      </div>
    </div>
  )

  return (
    <>
      {/* Desktop Sidebar */}
      <aside
        className={`
          hidden md:block fixed left-0 top-0 h-full z-40
          transition-all duration-200
          ${isCollapsed ? 'w-14' : 'w-60'}
        `}
      >
        {sidebarContent}
      </aside>

      {/* Mobile Menu Button */}
      <button
        onClick={() => setIsMobileOpen(true)}
        className="md:hidden fixed top-4 left-4 z-50 p-2 bg-sidebar-bg text-sidebar-text rounded shadow-lg"
        aria-label="Open menu"
      >
        <Settings className="w-5 h-5" />
      </button>

      {/* Mobile Sidebar Overlay */}
      {isMobileOpen && (
        <>
          <div
            className="md:hidden fixed inset-0 bg-black bg-opacity-50 z-40"
            onClick={() => setIsMobileOpen(false)}
          />
          <aside className="md:hidden fixed left-0 top-0 h-full w-60 z-50">
            {sidebarContent}
          </aside>
        </>
      )}
    </>
  )
}

// ── Helpers ──────────────────────────────────────────────────────────────────

function userDisplayName(user: CurrentUser | null): string {
  if (!user) return '...'
  if (user.mode === 'open') return 'Open Mode'
  if (user.name) return user.name
  if (user.email) return user.email.split('@')[0] ?? user.email
  return 'You'
}

function initials(user: CurrentUser | null): string {
  const name = user?.name || user?.email || ''
  const parts = name.split(/[\s@._-]+/).filter(Boolean).slice(0, 2)
  if (parts.length === 0) return '?'
  return parts.map((p) => p[0]!.toUpperCase()).join('')
}

function UserAvatar({
  user,
  size = 'md',
  className = '',
}: {
  user: CurrentUser | null
  size?: 'sm' | 'md'
  className?: string
}) {
  const dim = size === 'sm' ? 'h-6 w-6 text-xs' : 'h-8 w-8 text-sm'
  return (
    <div
      className={`${dim} rounded-full bg-blue-500 flex items-center justify-center text-white font-medium flex-shrink-0 ${className}`}
    >
      {initials(user)}
    </div>
  )
}
