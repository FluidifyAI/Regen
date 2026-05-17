interface Props {
  onComplete: () => void
}

export function WizardStepDone({ onComplete }: Props) {
  return (
    <div className="space-y-6 text-center">
      <div className="flex flex-col items-center gap-3">
        <div className="relative flex items-center justify-center w-16 h-16">
          <div className="absolute inset-0 rounded-full bg-green-100" />
          <span className="relative text-3xl">🎉</span>
        </div>
        <div>
          <h2 className="text-xl font-semibold text-text-primary">You're set up</h2>
          <p className="text-sm text-text-secondary mt-1">Regen is ready. Here's where to go next.</p>
        </div>
      </div>

      <div className="grid gap-2 text-left">
        <a
          href="/incidents"
          className="flex items-center gap-3 rounded-lg border border-border bg-surface-primary hover:bg-surface-secondary/50 px-4 py-3 transition-colors"
        >
          <span className="text-lg">🚨</span>
          <div>
            <p className="text-sm font-medium text-text-primary">View your first incident</p>
            <p className="text-xs text-text-tertiary">See how alerts become incidents in real time</p>
          </div>
        </a>

        <a
          href="/on-call"
          className="flex items-center gap-3 rounded-lg border border-border bg-surface-primary hover:bg-surface-secondary/50 px-4 py-3 transition-colors"
        >
          <span className="text-lg">📅</span>
          <div>
            <p className="text-sm font-medium text-text-primary">Go to your schedule</p>
            <p className="text-xs text-text-tertiary">Manage rotations, overrides, and escalations</p>
          </div>
        </a>

        <a
          href="https://github.com/FluidifyAI/Regen#readme"
          target="_blank"
          rel="noopener noreferrer"
          className="flex items-center gap-3 rounded-lg border border-border bg-surface-primary hover:bg-surface-secondary/50 px-4 py-3 transition-colors"
        >
          <span className="text-lg">📖</span>
          <div>
            <p className="text-sm font-medium text-text-primary">Read the docs</p>
            <p className="text-xs text-text-tertiary">Webhook integrations, alert routing, and more</p>
          </div>
        </a>
      </div>

      <button
        onClick={onComplete}
        className="w-full h-10 rounded-lg bg-brand-primary hover:bg-brand-primary-hover text-white text-sm font-medium transition-colors"
      >
        Go to dashboard →
      </button>
    </div>
  )
}
