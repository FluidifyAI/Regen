/**
 * LoginPage — shown when SAML SSO is configured and the user has no active session.
 * In open-access mode (no SAML configured), this page is never rendered.
 *
 * Design: cohesive with the app's dark navy (#0F172A) + blue (#2563EB) identity.
 * Background uses a CSS dot-grid for depth without adding visual noise.
 */
export function LoginPage() {
  return (
    <div
      className="min-h-screen bg-[#0F172A] flex items-center justify-center p-4"
      style={{
        backgroundImage: `radial-gradient(circle, #1E293B 1px, transparent 1px)`,
        backgroundSize: '28px 28px',
      }}
    >
      {/* Subtle ambient glow behind the card */}
      <div
        className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-96 h-96 rounded-full pointer-events-none"
        style={{
          background: 'radial-gradient(circle, rgba(37,99,235,0.08) 0%, transparent 70%)',
        }}
      />

      <div className="relative w-full max-w-sm">
        {/* Card */}
        <div
          className="rounded-2xl border border-[#1E293B] bg-[#0F172A] p-10"
          style={{ boxShadow: '0 0 0 1px #1E293B, 0 24px 48px rgba(0,0,0,0.4)' }}
        >
          {/* Logo + wordmark */}
          <div className="flex flex-col items-center mb-10">
            <div className="relative mb-5">
              {/* Outer ring */}
              <div className="absolute inset-0 rounded-full border border-[#2563EB] opacity-20 scale-150" />
              {/* Icon container */}
              <div className="w-14 h-14 rounded-xl bg-[#1E3A5F] flex items-center justify-center border border-[#2563EB] border-opacity-30">
                <ShieldIcon className="w-7 h-7 text-[#2563EB]" />
              </div>
            </div>

            <h1 className="text-[#F1F5F9] text-xl font-semibold tracking-tight">
              OpenIncident
            </h1>
            <p className="text-[#475569] text-sm mt-1">
              Incident management, self-hosted
            </p>
          </div>

          {/* Divider */}
          <div className="border-t border-[#1E293B] mb-8" />

          {/* Sign in section */}
          <div className="space-y-4">
            <p className="text-[#94A3B8] text-sm text-center">
              Your organization uses Single Sign-On.
            </p>

            <a
              href="/saml/login"
              className="flex items-center justify-center gap-2.5 w-full h-11 rounded-lg bg-[#2563EB] hover:bg-[#1D4ED8] text-white text-sm font-medium transition-colors duration-150 focus:outline-none focus:ring-2 focus:ring-[#2563EB] focus:ring-offset-2 focus:ring-offset-[#0F172A]"
            >
              <KeyIcon className="w-4 h-4" />
              Sign in with SSO
            </a>
          </div>

          {/* Footer note */}
          <p className="mt-8 text-center text-[#334155] text-xs">
            Access is managed by your identity provider
          </p>
        </div>

        {/* Below-card help link */}
        <p className="text-center mt-6 text-[#334155] text-xs">
          Self-hosting?{' '}
          <a
            href="https://github.com/openincident/openincident"
            target="_blank"
            rel="noopener noreferrer"
            className="text-[#475569] hover:text-[#94A3B8] transition-colors underline underline-offset-2"
          >
            View documentation
          </a>
        </p>
      </div>
    </div>
  )
}

// ── Inline SVG icons (no extra deps) ────────────────────────────────────────

function ShieldIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth={1.75}
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"
      />
    </svg>
  )
}

function KeyIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth={2}
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        d="M15.75 5.25a3 3 0 013 3m3 0a6 6 0 01-7.029 5.912c-.563-.097-1.159.026-1.563.43L10.5 17.25H8.25v2.25H6v2.25H2.25v-2.818c0-.597.237-1.17.659-1.591l6.499-6.499c.404-.404.527-1 .43-1.563A6 6 0 1121.75 8.25z"
      />
    </svg>
  )
}
