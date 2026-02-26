import { Link } from 'react-router-dom'
import { Shield, LogIn } from 'lucide-react'

/**
 * Shown at /logout after a successful sign-out.
 * Gives the user a clear confirmation and a path back to login.
 */
export function LogoutPage() {
  return (
    <div className="fixed inset-0 bg-[#0d0f14] flex items-center justify-center">
      <div className="flex flex-col items-center gap-6 text-center max-w-sm px-4">
        {/* Shield logo */}
        <div className="relative">
          <div className="absolute inset-0 rounded-full bg-blue-500 opacity-10 scale-150" />
          <Shield className="w-12 h-12 text-blue-500 relative" strokeWidth={1.5} />
        </div>

        <div>
          <h1 className="text-white text-xl font-semibold mb-1">You've been signed out</h1>
          <p className="text-[#4a5568] text-sm">Your session has been cleared securely.</p>
        </div>

        <Link
          to="/login"
          className="flex items-center gap-2 px-5 py-2.5 bg-blue-600 hover:bg-blue-500 text-white text-sm font-medium rounded-lg transition-colors"
        >
          <LogIn className="w-4 h-4" />
          Sign in again
        </Link>
      </div>
    </div>
  )
}
