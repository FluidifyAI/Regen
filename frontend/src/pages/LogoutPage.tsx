/**
 * LogoutPage — split-screen layout matching LoginPage.
 * Left: dark "all clear" panel with a single resolved card.
 * Right: white signed-out confirmation + sign in again button.
 */
import { Link } from 'react-router-dom'
import { LogIn } from 'lucide-react'
import { motion } from 'framer-motion'

// ── Left panel ────────────────────────────────────────────────────────────────

function AllClearPanel() {
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
        <div className="mb-9">
          <p className="text-[10px] font-bold tracking-[0.2em] text-[#4ade80] uppercase mb-4">
            All clear
          </p>
          <h2 className="text-[2.1rem] font-bold text-white leading-[1.15] tracking-tight">
            Your watch<br />
            <span
              style={{
                backgroundImage: 'linear-gradient(90deg, #4ade80, #16a34a)',
                WebkitBackgroundClip: 'text',
                WebkitTextFillColor: 'transparent',
              }}
            >
              is over.
            </span>
          </h2>
          <p className="text-[#52525b] text-sm mt-3 leading-relaxed">
            Incidents handled. Session cleared.<br />
            See you next time.
          </p>
        </div>

        {/* Single resolved card — springs in, then breathes */}
        <motion.div
          initial={{ opacity: 0, y: 16, filter: 'blur(6px)' }}
          animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
          transition={{ duration: 0.55, delay: 0.4, ease: [0.4, 0, 0.2, 1] }}
          className="w-full max-w-[280px]"
        >
          <motion.div
            animate={{ y: [0, -4, 0] }}
            transition={{ duration: 4, repeat: Infinity, ease: 'easeInOut' }}
            className="px-4 py-3.5 rounded-xl"
            style={{
              backgroundColor: 'rgba(34,197,94,0.09)',
              border: '1px solid rgba(34,197,94,0.18)',
              borderLeft: '2.5px solid #22c55e',
            }}
          >
            <span className="text-[9.5px] font-bold tracking-[0.15em] uppercase text-[#4ade80]">
              ✓ RESOLVED
            </span>
            <p className="text-[13px] font-semibold text-white mt-0.5">INC-043 closed</p>
            <p className="text-[10px] text-[#71717a] mt-0.5">6 min · MTTR ↓ 68% · by Fluidify Agent</p>
          </motion.div>
        </motion.div>
      </div>

      {/* Footer */}
      <div className="px-6 py-4 flex-shrink-0 border-t border-white/[0.06]">
        <p className="text-[#3f3f46] text-[11px]">Open-source · AGPLv3 · Self-hosted</p>
      </div>
    </div>
  )
}

// ── LogoutPage ────────────────────────────────────────────────────────────────

export function LogoutPage() {
  return (
    <div className="fixed inset-0 flex">

      {/* ── Left: all-clear panel — hidden on mobile ── */}
      <div
        className="hidden md:block md:w-[44%] flex-shrink-0 overflow-hidden relative"
        style={{ background: '#0a0a0a' }}
      >
        {/* Ambient orbs */}
        <div
          className="absolute pointer-events-none"
          style={{
            top: '-10%',
            left: '-15%',
            width: 420,
            height: 420,
            borderRadius: '50%',
            background: 'radial-gradient(circle, rgba(34,197,94,0.12) 0%, transparent 70%)',
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
            background: 'radial-gradient(circle, rgba(34,197,94,0.08) 0%, transparent 70%)',
            filter: 'blur(40px)',
          }}
        />
        <AllClearPanel />
      </div>

      {/* ── Right: signed-out confirmation ── */}
      <div className="flex-1 flex items-center justify-center p-8 bg-[#f8fafc]">
        <motion.div
          className="w-full max-w-sm"
          initial={{ opacity: 0, y: 24 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ type: 'spring', stiffness: 280, damping: 28 }}
        >
          <div
            className="flex flex-col items-center gap-6 text-center rounded-2xl border border-[#f0d0dc] bg-white px-10 py-12"
            style={{
              boxShadow: '0 1px 3px rgba(240,98,146,0.06), 0 20px 40px rgba(176,43,82,0.10)',
            }}
          >
            <img
              src="/logo-icon.png"
              alt="Fluidify Regen"
              style={{ width: '72px', height: '72px', objectFit: 'contain' }}
              draggable={false}
            />

            <div>
              <div className="text-2xl font-bold tracking-tight mb-2 text-[#121212]">
                Fluidify Regen
              </div>
              <h1 className="text-[#1E293B] text-lg font-semibold mb-1">You've been signed out</h1>
              <p className="text-[#64748B] text-sm">Your session has been cleared securely.</p>
            </div>

            <motion.div
              className="rounded-lg"
              animate={{
                boxShadow: [
                  '0 0 0px 0px rgba(176,43,82,0)',
                  '0 0 22px 6px rgba(176,43,82,0.45)',
                  '0 0 0px 0px rgba(176,43,82,0)',
                ],
              }}
              transition={{ duration: 0.8, delay: 0.7, repeat: 2, repeatDelay: 0.4 }}
              whileHover={{ scale: 1.02, boxShadow: '0 0 22px 6px rgba(176,43,82,0.4)' }}
              whileTap={{ scale: 0.97 }}
            >
              <Link
                to="/login"
                className="relative group flex items-center gap-2 px-5 py-2.5 bg-[#b02b52] text-white text-sm font-medium rounded-lg overflow-hidden focus:outline-none focus:ring-2 focus:ring-[#b02b52] focus:ring-offset-2 focus:ring-offset-white"
              >
                <span className="absolute inset-0 bg-gradient-to-r from-[#b02b52] to-[#f06292] opacity-0 group-hover:opacity-100 transition-opacity duration-300 ease-in-out" />
                <LogIn className="relative z-10 w-4 h-4" />
                <span className="relative z-10">Sign in again</span>
              </Link>
            </motion.div>
          </div>
        </motion.div>
      </div>
    </div>
  )
}
