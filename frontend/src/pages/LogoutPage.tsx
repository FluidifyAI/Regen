import { Link } from 'react-router-dom'
import { LogIn } from 'lucide-react'
import { BackgroundAnimation } from '../components/ui/BackgroundAnimation'

export function LogoutPage() {
  return (
    <div
      className="fixed inset-0 flex items-center justify-center"
      style={{
        backgroundImage: 'radial-gradient(circle, #f8bbd0 1px, transparent 1px), linear-gradient(160deg, #fce4ec 0%, #f8f9fa 60%)',
        backgroundSize: '28px 28px, 100% 100%',
      }}
    >
      <BackgroundAnimation />
      <div
        className="relative flex flex-col items-center gap-6 text-center px-10 py-12 rounded-2xl border border-[#f8bbd0] bg-white"
        style={{
          zIndex: 1,
          boxShadow: '0 1px 3px rgba(240,98,146,0.08), 0 24px 48px rgba(176,43,82,0.12)',
          maxWidth: '360px',
          width: '100%',
        }}
      >
        <img
          src="/logo-icon.png"
          alt="Fluidify Alert"
          style={{ width: '72px', height: '72px', objectFit: 'contain' }}
          draggable={false}
        />

        <div>
          <div className="text-2xl font-bold tracking-tight mb-2 text-[#121212]">
            Fluidify Alert
          </div>
          <h1 className="text-[#1E293B] text-lg font-semibold mb-1">You've been signed out</h1>
          <p className="text-[#64748B] text-sm">Your session has been cleared securely.</p>
        </div>

        <Link
          to="/login"
          className="relative group flex items-center gap-2 px-5 py-2.5 bg-[#b02b52] text-white text-sm font-medium rounded-lg overflow-hidden focus:outline-none focus:ring-2 focus:ring-[#b02b52] focus:ring-offset-2 focus:ring-offset-white"
        >
          <span className="absolute inset-0 bg-gradient-to-r from-[#b02b52] to-[#f06292] opacity-0 group-hover:opacity-100 transition-opacity duration-300 ease-in-out" />
          <LogIn className="relative z-10 w-4 h-4" />
          <span className="relative z-10">Sign in again</span>
        </Link>
      </div>
    </div>
  )
}
