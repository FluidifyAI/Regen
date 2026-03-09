/**
 * LoginPage — split-screen layout.
 * Left: dark story panel with animated incident feed (Framer Motion).
 * Right: white login form (unchanged functionality).
 */
import { useState, useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { login, bootstrap, exchangeSetupToken, forgotPassword } from '../api/auth'
import { useAuth } from '../hooks/useAuth'
import { getSlackOAuthStatus } from '../api/slack'

// ── Story panel data ──────────────────────────────────────────────────────────

const FEED_EVENTS = [
  {
    id: 'alert',
    badge: '● CRITICAL',
    badgeColor: '#f87171',
    title: 'Payment gateway down',
    sub: 'error rate 94% · just now',
    bg: 'rgba(239,68,68,0.09)',
    borderColor: '#ef4444',
    gradient: false,
  },
  {
    id: 'inc',
    badge: '⚡ AUTO',
    badgeColor: '#fb923c',
    title: 'INC-043 created',
    sub: 'by Fluidify Agent · 0s',
    bg: 'rgba(249,115,22,0.09)',
    borderColor: '#f97316',
    gradient: false,
  },
  {
    id: 'ai',
    badge: '✦ AGENT',
    badgeColor: '#f06292',
    title: 'Root cause identified',
    sub: 'Redis timeout · deploy v3.1.2',
    bg: 'rgba(176,43,82,0.14)',
    borderColor: '#f06292',
    gradient: true,
  },
  {
    id: 'resolved',
    badge: '✓ RESOLVED',
    badgeColor: '#4ade80',
    title: 'INC-043 closed',
    sub: '6 min · MTTR ↓ 68%',
    bg: 'rgba(34,197,94,0.09)',
    borderColor: '#22c55e',
    gradient: false,
  },
]

const containerVariants = {
  hidden: {},
  visible: {
    transition: { staggerChildren: 0.85, delayChildren: 0.4 },
  },
  exit: {
    transition: { staggerChildren: 0.06, staggerDirection: -1 as const },
  },
}

const itemVariants = {
  hidden: { opacity: 0, x: -18, filter: 'blur(6px)' },
  visible: {
    opacity: 1,
    x: 0,
    filter: 'blur(0px)',
    transition: { duration: 0.5, ease: [0.4, 0, 0.2, 1] as const },
  },
  exit: {
    opacity: 0,
    x: 12,
    filter: 'blur(4px)',
    transition: { duration: 0.22 },
  },
}

// ── Story panel component ─────────────────────────────────────────────────────

function StoryPanel() {
  // Incrementing `cycle` remounts the AnimatePresence child, replaying the sequence
  const [cycle, setCycle] = useState(0)

  useEffect(() => {
    // entrance: ~0.4 + 3×0.85 + 0.5 ≈ 3.45s | show for 3s | exit ~0.4s | pause 1s ≈ 8s total
    const t = setTimeout(() => setCycle((c) => c + 1), 8000)
    return () => clearTimeout(t)
  }, [cycle])

  return (
    <div className="flex flex-col h-full select-none">
      {/* Top bar */}
      <div className="flex items-center h-16 px-6 flex-shrink-0 border-b border-white/[0.06]">
        <img
          src="/logo-icon.png"
          alt=""
          width={28}
          height={28}
          className="object-contain flex-shrink-0"
          draggable={false}
        />
        <span className="ml-2.5 text-white font-bold tracking-tight text-[15px]">
          Fluidify Regen
        </span>
      </div>

      {/* Main content */}
      <div className="flex-1 flex flex-col justify-center px-8 py-10 overflow-hidden">
        {/* Headline */}
        <div className="mb-9">
          <p className="text-[10px] font-bold tracking-[0.2em] text-[#f06292] uppercase mb-4">
            Live incident feed
          </p>
          <h2 className="text-[2.1rem] font-bold text-white leading-[1.15] tracking-tight">
            From alert<br />
            <span
              style={{
                backgroundImage: 'linear-gradient(90deg, #f06292, #b02b52)',
                WebkitBackgroundClip: 'text',
                WebkitTextFillColor: 'transparent',
              }}
            >
              to resolved.
            </span>
          </h2>
          <p className="text-[#52525b] text-sm mt-3 leading-relaxed">
            AI-powered incident management.<br />
            Open-source. Self-hosted. Free.
          </p>
        </div>

        {/* Animated feed */}
        <AnimatePresence mode="wait">
          <motion.div
            key={cycle}
            variants={containerVariants}
            initial="hidden"
            animate="visible"
            exit="exit"
            className="space-y-2.5 w-full max-w-[300px]"
          >
            {FEED_EVENTS.map((ev) => (
              <motion.div key={ev.id} variants={itemVariants}>
                {ev.gradient ? (
                  <div
                    className="px-4 py-3 rounded-xl"
                    style={{
                      background: `linear-gradient(${ev.bg}, ${ev.bg}) padding-box, linear-gradient(135deg, #b02b52, #f06292) border-box`,
                      border: '1px solid transparent',
                    }}
                  >
                    <span
                      className="text-[9.5px] font-bold tracking-[0.15em] uppercase"
                      style={{ color: ev.badgeColor }}
                    >
                      {ev.badge}
                    </span>
                    <p className="text-[12px] font-semibold text-white mt-0.5">{ev.title}</p>
                    <p className="text-[10px] text-[#71717a] mt-0.5">{ev.sub}</p>
                  </div>
                ) : (
                  <div
                    className="px-4 py-3 rounded-xl"
                    style={{
                      backgroundColor: ev.bg,
                      border: `1px solid ${ev.borderColor}30`,
                      borderLeft: `2.5px solid ${ev.borderColor}`,
                    }}
                  >
                    <span
                      className="text-[9.5px] font-bold tracking-[0.15em] uppercase"
                      style={{ color: ev.badgeColor }}
                    >
                      {ev.badge}
                    </span>
                    <p className="text-[12px] font-semibold text-white mt-0.5">{ev.title}</p>
                    <p className="text-[10px] text-[#71717a] mt-0.5">{ev.sub}</p>
                  </div>
                )}
              </motion.div>
            ))}
          </motion.div>
        </AnimatePresence>
      </div>

      {/* Footer */}
      <div className="px-6 py-4 flex-shrink-0 border-t border-white/[0.06]">
        <p className="text-[#3f3f46] text-[11px]">Open-source · AGPLv3 · Self-hosted</p>
      </div>
    </div>
  )
}

// ── Utility ───────────────────────────────────────────────────────────────────

function postLoginRedirect(): string {
  const params = new URLSearchParams(window.location.search)
  const next = params.get('next')
  if (next && next.startsWith('/') && !next.startsWith('//')) return next
  return '/'
}

// ── LoginPage ─────────────────────────────────────────────────────────────────

export function LoginPage() {
  const { ssoEnabled, openMode, loading: authLoading } = useAuth()
  const [slackLoginEnabled, setSlackLoginEnabled] = useState(false)

  useEffect(() => {
    getSlackOAuthStatus()
      .then((r) => setSlackLoginEnabled(r.enabled))
      .catch(() => {})
  }, [])

  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const token = params.get('setup')
    if (token) {
      exchangeSetupToken(token)
        .then(() => { window.location.href = postLoginRedirect() })
        .catch(() => setError('This setup link has expired or has already been used.'))
      return
    }
    const oauthError = params.get('error')
    if (oauthError === 'no_account') setError('No account found for your Slack email. Ask your admin to go to Settings → Users → Import from Slack to add you.')
    else if (oauthError === 'slack_auth_failed') setError('Slack authentication failed. Please try again.')
    else if (oauthError === 'invalid_state') setError('Login session expired. Please try again.')
  }, [])

  const [showSetup, setShowSetup] = useState(openMode)
  const [setupName, setSetupName] = useState('')
  const [setupEmail, setSetupEmail] = useState('')
  const [setupPassword, setSetupPassword] = useState('')
  const [setupError, setSetupError] = useState('')
  const [setupLoading, setSetupLoading] = useState(false)

  const [showForgot, setShowForgot] = useState(false)
  const [forgotEmail, setForgotEmail] = useState('')
  const [forgotLoading, setForgotLoading] = useState(false)
  const [forgotToken, setForgotToken] = useState('')
  const [forgotError, setForgotError] = useState('')
  const [forgotCopied, setForgotCopied] = useState(false)

  async function handleForgot(e: React.FormEvent) {
    e.preventDefault()
    setForgotLoading(true)
    setForgotError('')
    setForgotToken('')
    try {
      const res = await forgotPassword(forgotEmail)
      if (res.setup_token) setForgotToken(res.setup_token)
      else setForgotError('If that email has a local account, a reset link will appear here.')
    } catch {
      setForgotError('Something went wrong. Please try again.')
    } finally {
      setForgotLoading(false)
    }
  }

  function copyForgotLink() {
    navigator.clipboard.writeText(`${window.location.origin}/login?setup=${forgotToken}`)
    setForgotCopied(true)
    setTimeout(() => setForgotCopied(false), 2000)
  }

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      await login({ email, password })
      window.location.href = postLoginRedirect()
    } catch {
      setError('Invalid email or password')
    } finally {
      setLoading(false)
    }
  }

  async function handleSetup(e: React.FormEvent) {
    e.preventDefault()
    setSetupError('')
    setSetupLoading(true)
    try {
      await bootstrap({ name: setupName, email: setupEmail, password: setupPassword })
      await login({ email: setupEmail, password: setupPassword })
      window.location.href = '/'
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err)
      if (msg.includes('409') || msg.includes('already exist')) setSetupError('An admin account already exists. Sign in above.')
      else if (msg.includes('8')) setSetupError('Password must be at least 8 characters.')
      else setSetupError('Setup failed. Check your details and try again.')
    } finally {
      setSetupLoading(false)
    }
  }

  const inputClass = 'w-full h-10 rounded-lg bg-[#F8FAFC] border border-[#E2E8F0] text-[#1E293B] text-sm px-3 placeholder-[#94A3B8] focus:outline-none focus:ring-2 focus:ring-[#b02b52] focus:border-transparent transition-colors duration-150'

  return (
    <div className="fixed inset-0 flex">

      {/* ── Left: story panel — hidden on mobile ── */}
      <div
        className="hidden md:block md:w-[44%] flex-shrink-0 overflow-hidden relative"
        style={{ background: '#0a0a0a' }}
      >
        {/* Subtle ambient orb */}
        <div
          className="absolute pointer-events-none"
          style={{
            top: '-10%',
            left: '-15%',
            width: 420,
            height: 420,
            borderRadius: '50%',
            background: 'radial-gradient(circle, rgba(176,43,82,0.18) 0%, transparent 70%)',
            filter: 'blur(40px)',
          }}
        />
        <div
          className="absolute pointer-events-none"
          style={{
            bottom: '-5%',
            right: '-10%',
            width: 300,
            height: 300,
            borderRadius: '50%',
            background: 'radial-gradient(circle, rgba(240,98,146,0.12) 0%, transparent 70%)',
            filter: 'blur(40px)',
          }}
        />
        <StoryPanel />
      </div>

      {/* ── Right: login form ── */}
      <div className="flex-1 flex flex-col items-center justify-center p-8 overflow-y-auto bg-[#f8fafc]">
        <motion.div
          className="w-full max-w-sm"
          initial={{ opacity: 0, y: 24 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ type: 'spring', stiffness: 280, damping: 28 }}
        >
          {/* Card */}
          <div
            className="rounded-2xl border border-[#f0d0dc] bg-white p-10"
            style={{
              boxShadow: '0 1px 3px rgba(240,98,146,0.06), 0 20px 40px rgba(176,43,82,0.10)',
            }}
          >
            {/* Logo */}
            <div className="flex flex-col items-center mb-10">
              <img
                src="/logo-icon.png"
                alt="Fluidify Regen"
                style={{ width: '72px', height: '72px', objectFit: 'contain' }}
                draggable={false}
                className="mb-3"
              />
              <div className="text-2xl font-bold tracking-tight text-[#121212]">
                Fluidify Regen
              </div>
              <p className="text-[#64748B] text-sm mt-1">
                {openMode ? 'Set up your instance' : 'Stay ahead of every incident'}
              </p>
            </div>

            <div className="border-t border-[#E2E8F0] mb-8" />

            {openMode && (
              <div className="mb-6 rounded-lg border border-[#b02b52]/20 bg-[#fce4ec] px-4 py-3 text-center">
                <p className="text-[#b02b52] text-sm font-medium">Welcome — let's get you set up</p>
                <p className="text-[#64748B] text-xs mt-1">
                  No accounts exist yet. Create the admin account below to get started.
                </p>
              </div>
            )}

            {/* Sign-in form */}
            <form onSubmit={handleSubmit} className={`space-y-4 ${openMode ? 'hidden' : ''}`} noValidate>
              <div className="space-y-3">
                <div>
                  <label htmlFor="email" className="block text-[#374151] text-xs font-medium mb-1.5">
                    Email
                  </label>
                  <input
                    id="email"
                    type="email"
                    autoComplete="email"
                    required
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder="you@example.com"
                    className={inputClass}
                  />
                </div>
                <div>
                  <label htmlFor="password" className="block text-[#374151] text-xs font-medium mb-1.5">
                    Password
                  </label>
                  <input
                    id="password"
                    type="password"
                    autoComplete="current-password"
                    required
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    placeholder="••••••••"
                    className={inputClass}
                  />
                </div>
              </div>

              {error && (
                <p className="text-[#f87171] text-sm text-center" role="alert">{error}</p>
              )}

              {/* Sign in button with heartbeat */}
              <motion.div
                className="rounded-lg"
                animate={{
                  boxShadow: [
                    '0 0 0px 0px rgba(176,43,82,0)',
                    '0 0 28px 8px rgba(176,43,82,0.5)',
                    '0 0 0px 0px rgba(176,43,82,0)',
                  ],
                }}
                transition={{ duration: 0.8, delay: 0.6, repeat: 2, repeatDelay: 0.4 }}
                whileHover={{ scale: 1.02, boxShadow: '0 0 22px 6px rgba(176,43,82,0.4)' }}
                whileTap={{ scale: 0.97 }}
              >
                <button
                  type="submit"
                  disabled={loading}
                  className="relative group flex items-center justify-center gap-2.5 w-full h-11 rounded-lg bg-[#b02b52] disabled:opacity-50 disabled:cursor-not-allowed text-white text-sm font-medium overflow-hidden focus:outline-none focus:ring-2 focus:ring-[#b02b52] focus:ring-offset-2 focus:ring-offset-white"
                >
                  <span className="absolute inset-0 bg-gradient-to-r from-[#b02b52] to-[#f06292] opacity-0 group-hover:opacity-100 group-disabled:opacity-0 transition-opacity duration-300 ease-in-out" />
                  <span className="relative z-10">
                    {loading ? 'Signing in…' : 'Sign in'}
                  </span>
                </button>
              </motion.div>

              <div className="text-center">
                <button
                  type="button"
                  onClick={() => { setShowForgot(!showForgot); setForgotToken(''); setForgotError('') }}
                  className="text-[#94A3B8] hover:text-[#64748B] text-xs transition-colors"
                >
                  Forgot password?
                </button>
              </div>

              {showForgot && (
                <div className="rounded-lg border border-[#E2E8F0] bg-[#F8FAFC] p-4 space-y-3">
                  {forgotToken ? (
                    <>
                      <p className="text-[#b02b52] text-xs font-medium">Reset link generated</p>
                      <p className="text-[#64748B] text-xs">Copy this link and send it to the user. It expires in 7 days.</p>
                      <div className="flex gap-2">
                        <input
                          readOnly
                          value={`${window.location.origin}/login?setup=${forgotToken}`}
                          className="flex-1 text-xs rounded-lg bg-white border border-[#E2E8F0] text-[#1E293B] px-3 py-2 font-mono focus:outline-none"
                        />
                        <button
                          type="button"
                          onClick={copyForgotLink}
                          className="px-3 py-2 rounded-lg bg-[#b02b52] hover:bg-[#8f1e3f] text-white text-xs font-medium transition-colors min-w-[60px]"
                        >
                          {forgotCopied ? 'Copied!' : 'Copy'}
                        </button>
                      </div>
                    </>
                  ) : (
                    <form onSubmit={handleForgot} className="space-y-3">
                      <p className="text-[#64748B] text-xs">Enter the account email and we'll generate a reset link you can share.</p>
                      <input
                        type="email"
                        required
                        value={forgotEmail}
                        onChange={(e) => setForgotEmail(e.target.value)}
                        placeholder="you@example.com"
                        className="w-full h-9 rounded-lg bg-white border border-[#E2E8F0] text-[#1E293B] text-xs px-3 placeholder-[#94A3B8] focus:outline-none focus:ring-2 focus:ring-[#b02b52] focus:border-transparent"
                      />
                      {forgotError && <p className="text-[#f87171] text-xs">{forgotError}</p>}
                      <button
                        type="submit"
                        disabled={forgotLoading}
                        className="w-full h-9 rounded-lg border border-[#E2E8F0] hover:bg-white text-[#64748B] text-xs font-medium transition-colors disabled:opacity-50"
                      >
                        {forgotLoading ? 'Generating…' : 'Generate reset link'}
                      </button>
                    </form>
                  )}
                </div>
              )}
            </form>

            {/* SSO */}
            {!authLoading && ssoEnabled && (
              <>
                <div className="flex items-center gap-3 my-6">
                  <div className="flex-1 border-t border-[#E2E8F0]" />
                  <span className="text-[#94A3B8] text-xs">or</span>
                  <div className="flex-1 border-t border-[#E2E8F0]" />
                </div>
                <a
                  href="/saml/login"
                  className="flex items-center justify-center gap-2.5 w-full h-11 rounded-lg border border-[#E2E8F0] bg-white hover:bg-[#F8FAFC] text-[#374151] hover:text-[#1E293B] text-sm font-medium transition-colors duration-150 focus:outline-none focus:ring-2 focus:ring-[#b02b52] focus:ring-offset-2"
                >
                  <KeyIcon className="w-4 h-4" />
                  Sign in with SSO
                </a>
              </>
            )}

            {/* Slack */}
            {!authLoading && slackLoginEnabled && (
              <>
                <div className="flex items-center gap-3 my-6">
                  <div className="flex-1 border-t border-[#E2E8F0]" />
                  <span className="text-[#94A3B8] text-xs">or</span>
                  <div className="flex-1 border-t border-[#E2E8F0]" />
                </div>
                <a
                  href="/api/v1/auth/slack"
                  className="flex items-center justify-center gap-2.5 w-full h-11 rounded-lg border border-[#333] bg-[#1c1c1c] hover:bg-[#242424] text-[#e4e4e7] hover:text-white text-sm font-medium transition-colors duration-150 focus:outline-none focus:ring-2 focus:ring-[#b02b52] focus:ring-offset-2"
                >
                  <SlackIcon className="w-4 h-4" />
                  Continue with Slack
                </a>
              </>
            )}

            {/* First-time setup */}
            <div className={openMode ? '' : 'mt-6'}>
              {!openMode && (
                <div className="flex items-center gap-3 mb-4">
                  <div className="flex-1 border-t border-[#E2E8F0]" />
                  <button
                    type="button"
                    onClick={() => { setShowSetup(!showSetup); setSetupError('') }}
                    className="text-[#94A3B8] hover:text-[#64748B] text-xs transition-colors whitespace-nowrap"
                  >
                    {showSetup ? 'Cancel setup' : 'First time here? Create admin account →'}
                  </button>
                  <div className="flex-1 border-t border-[#E2E8F0]" />
                </div>
              )}

              {showSetup && (
                <form onSubmit={handleSetup} className="space-y-3" noValidate>
                  <div>
                    <label htmlFor="setup-name" className="block text-[#374151] text-xs font-medium mb-1.5">Full name</label>
                    <input id="setup-name" type="text" autoComplete="name" required value={setupName} onChange={(e) => setSetupName(e.target.value)} placeholder="Jane Smith" className={inputClass} />
                  </div>
                  <div>
                    <label htmlFor="setup-email" className="block text-[#374151] text-xs font-medium mb-1.5">Email</label>
                    <input id="setup-email" type="email" autoComplete="email" required value={setupEmail} onChange={(e) => setSetupEmail(e.target.value)} placeholder="admin@example.com" className={inputClass} />
                  </div>
                  <div>
                    <label htmlFor="setup-password" className="block text-[#374151] text-xs font-medium mb-1.5">
                      Password <span className="text-[#94A3B8] font-normal">(min. 8 characters)</span>
                    </label>
                    <input id="setup-password" type="password" autoComplete="new-password" required minLength={8} value={setupPassword} onChange={(e) => setSetupPassword(e.target.value)} placeholder="••••••••" className={inputClass} />
                  </div>
                  {setupError && <p className="text-[#f87171] text-sm text-center" role="alert">{setupError}</p>}
                  <button
                    type="submit"
                    disabled={setupLoading}
                    className="flex items-center justify-center gap-2.5 w-full h-11 rounded-lg border border-[#b02b52] bg-transparent hover:bg-[#fce4ec] disabled:opacity-50 disabled:cursor-not-allowed text-[#b02b52] text-sm font-medium transition-colors duration-150 focus:outline-none focus:ring-2 focus:ring-[#b02b52] focus:ring-offset-2"
                  >
                    {setupLoading ? 'Creating account…' : 'Create admin account & sign in'}
                  </button>
                </form>
              )}
            </div>

            <p className="mt-6 text-center text-[#94A3B8] text-xs">
              {openMode
                ? 'This account will have full admin access'
                : ssoEnabled
                  ? 'Access is managed by your identity provider or local accounts'
                  : 'Sign in with your Fluidify Regen credentials'}
            </p>
          </div>

          <p className="text-center mt-5 text-[#94A3B8] text-xs">
            Self-hosting?{' '}
            <a
              href="https://github.com/openincident/openincident"
              target="_blank"
              rel="noopener noreferrer"
              className="text-[#64748B] hover:text-[#374151] transition-colors underline underline-offset-2"
            >
              View documentation
            </a>
          </p>
        </motion.div>
      </div>
    </div>
  )
}

// ── Icons ─────────────────────────────────────────────────────────────────────

function KeyIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 5.25a3 3 0 013 3m3 0a6 6 0 01-7.029 5.912c-.563-.097-1.159.026-1.563.43L10.5 17.25H8.25v2.25H6v2.25H2.25v-2.818c0-.597.237-1.17.659-1.591l6.499-6.499c.404-.404.527-1 .43-1.563A6 6 0 1121.75 8.25z" />
    </svg>
  )
}

function SlackIcon({ className }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="currentColor">
      <path d="M5.042 15.165a2.528 2.528 0 0 1-2.52 2.523A2.528 2.528 0 0 1 0 15.165a2.527 2.527 0 0 1 2.522-2.52h2.52v2.52zm1.271 0a2.527 2.527 0 0 1 2.521-2.52 2.527 2.527 0 0 1 2.521 2.52v6.313A2.528 2.528 0 0 1 8.834 24a2.528 2.528 0 0 1-2.521-2.522v-6.313zM8.834 5.042a2.528 2.528 0 0 1-2.521-2.52A2.528 2.528 0 0 1 8.834 0a2.528 2.528 0 0 1 2.521 2.522v2.52H8.834zm0 1.271a2.528 2.528 0 0 1 2.521 2.521 2.528 2.528 0 0 1-2.521 2.521H2.522A2.528 2.528 0 0 1 0 8.834a2.528 2.528 0 0 1 2.522-2.521h6.312zm10.122 2.521a2.528 2.528 0 0 1 2.522-2.521A2.528 2.528 0 0 1 24 8.834a2.528 2.528 0 0 1-2.522 2.521h-2.522V8.834zm-1.268 0a2.528 2.528 0 0 1-2.523 2.521 2.527 2.527 0 0 1-2.52-2.521V2.522A2.527 2.527 0 0 1 15.165 0a2.528 2.528 0 0 1 2.523 2.522v6.312zm-2.523 10.122a2.528 2.528 0 0 1 2.523 2.522A2.528 2.528 0 0 1 15.165 24a2.527 2.527 0 0 1-2.52-2.522v-2.522h2.52zm0-1.268a2.527 2.527 0 0 1-2.52-2.523 2.526 2.526 0 0 1 2.52-2.52h6.313A2.527 2.527 0 0 1 24 15.165a2.528 2.528 0 0 1-2.522 2.523h-6.313z" />
    </svg>
  )
}
