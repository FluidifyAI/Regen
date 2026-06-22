import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { WizardStepSlack } from '../components/onboarding/WizardStepSlack'
import { WizardStepSchedule } from '../components/onboarding/WizardStepSchedule'
import { WizardStepInvite } from '../components/onboarding/WizardStepInvite'
import { WizardStepTestAlert } from '../components/onboarding/WizardStepTestAlert'
import { WizardStepDone } from '../components/onboarding/WizardStepDone'
import { getSetupStatus } from '../api/setup'
import { LOGO_ICON_DATA_URI } from '../assets/logoIcon'

const WIZARD_STORAGE_KEY = 'regen_setup_wizard_v1'
const TOTAL_STEPS = 5

const STEP_LABELS = [
  'Connect Slack',
  'Set up schedule',
  'Invite teammates',
  'Test alert',
  'Done',
]

type Step = 1 | 2 | 3 | 4 | 5

function readStoredStep(): Step {
  try {
    const raw = localStorage.getItem(WIZARD_STORAGE_KEY)
    if (!raw) return 1
    const parsed = JSON.parse(raw)
    if (parsed.dismissed) return 1
    const s = Number(parsed.currentStep)
    if (s >= 1 && s <= TOTAL_STEPS) return s as Step
  } catch {
    // ignore
  }
  return 1
}

function persistStep(step: Step) {
  localStorage.setItem(WIZARD_STORAGE_KEY, JSON.stringify({ currentStep: step }))
}

export function SetupWizardPage() {
  const navigate = useNavigate()
  const [currentStep, setCurrentStep] = useState<Step>(readStoredStep)
  const [hasSchedule, setHasSchedule] = useState(false)

  useEffect(() => {
    getSetupStatus()
      .then((s) => setHasSchedule(s.has_schedule))
      .catch(() => {})
  }, [])

  function advance() {
    if (currentStep < TOTAL_STEPS) {
      const next = (currentStep + 1) as Step
      setCurrentStep(next)
      persistStep(next)
    }
  }

  function completeFinal() {
    localStorage.setItem(WIZARD_STORAGE_KEY, JSON.stringify({ dismissed: true, currentStep: TOTAL_STEPS }))
    navigate('/')
  }

  return (
    <div className="min-h-screen" style={{ backgroundColor: '#F1F5F9' }}>
      <div className="max-w-2xl mx-auto px-4 py-10">
        {/* Header */}
        <div className="mb-8">
          <div className="flex items-center gap-2 mb-6">
            <img src={LOGO_ICON_DATA_URI} alt="Regen" className="w-7 h-7" />
            <span className="text-base font-bold text-text-primary tracking-tight">Regen</span>
          </div>
          <h1 className="text-2xl font-bold text-text-primary">Let's get you set up</h1>
          <p className="text-sm text-text-secondary mt-1">
            Follow these steps to go from zero to your first incident in about 15 minutes.
          </p>
        </div>

        {/* Progress bar */}
        <div className="mb-8">
          <div className="flex items-center justify-between mb-2">
            <span className="text-xs font-medium text-text-secondary">
              Step {currentStep} of {TOTAL_STEPS} — {STEP_LABELS[currentStep - 1]}
            </span>
            <span className="text-xs text-text-tertiary">{Math.round((currentStep / TOTAL_STEPS) * 100)}%</span>
          </div>
          <div className="h-1.5 w-full rounded-full bg-surface-secondary overflow-hidden">
            <div
              className="h-full rounded-full bg-brand-primary transition-all duration-500"
              style={{ width: `${(currentStep / TOTAL_STEPS) * 100}%` }}
            />
          </div>
          <div className="flex mt-2 gap-1">
            {Array.from({ length: TOTAL_STEPS }, (_, i) => (
              <div
                key={i}
                className={`flex-1 h-0.5 rounded-full transition-colors duration-300 ${i < currentStep ? 'bg-brand-primary' : 'bg-border'}`}
              />
            ))}
          </div>
        </div>

        {/* Step card */}
        <div className="bg-surface-primary rounded-xl border border-border shadow-sm p-6">
          <h2 className="text-base font-semibold text-text-primary mb-4">
            {STEP_LABELS[currentStep - 1]}
          </h2>

          {currentStep === 1 && <WizardStepSlack onComplete={advance} onSkip={advance} />}
          {currentStep === 2 && <WizardStepSchedule hasSchedule={hasSchedule} onComplete={advance} onSkip={advance} />}
          {currentStep === 3 && <WizardStepInvite onComplete={advance} onSkip={advance} />}
          {currentStep === 4 && <WizardStepTestAlert onComplete={advance} onSkip={advance} />}
          {currentStep === 5 && <WizardStepDone onComplete={completeFinal} />}
        </div>

        {/* Footer */}
        <p className="text-center text-xs text-text-tertiary mt-6">
          You can access these settings anytime from the{' '}
          <a href="/integrations" className="hover:underline text-text-secondary">Integrations</a> and{' '}
          <a href="/settings" className="hover:underline text-text-secondary">Settings</a> pages.
        </p>
      </div>
    </div>
  )
}
