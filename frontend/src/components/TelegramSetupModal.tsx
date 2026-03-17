/**
 * TelegramSetupModal — configure Telegram bot token + chat ID for incident notifications.
 * Step 1: Paste token → "Fetch Group" auto-discovers the chat ID.
 * Step 2: Confirm group name → Save.
 * Optional: "Send test" to verify the connection.
 */
import { useState } from 'react'
import { X, CheckCircle, AlertCircle, Loader2, Send } from 'lucide-react'
import {
  saveTelegramConfig,
  testTelegramConfig,
  fetchTelegramChatID,
  deleteTelegramConfig,
  type TelegramConfigStatus,
} from '../api/telegram_config'

interface TelegramSetupModalProps {
  onClose: () => void
  onSaved: (status: TelegramConfigStatus) => void
  existing?: TelegramConfigStatus
}

export function TelegramSetupModal({ onClose, onSaved, existing }: TelegramSetupModalProps) {
  const [botToken, setBotToken] = useState('')
  const [chatID, setChatID] = useState(existing?.chat_id ?? '')
  const [chatName, setChatName] = useState(existing?.chat_name ?? '')
  const [step, setStep] = useState<'token' | 'chat'>('token')
  const [fetching, setFetching] = useState(false)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [disconnecting, setDisconnecting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [testSuccess, setTestSuccess] = useState(false)

  async function handleFetchChatID() {
    if (!botToken.trim()) {
      setError('Paste your bot token first')
      return
    }
    setFetching(true)
    setError(null)
    try {
      const data = await fetchTelegramChatID(botToken.trim())
      setChatID(data.chat_id)
      setChatName(data.chat_name)
      setStep('chat')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to fetch chat ID')
    } finally {
      setFetching(false)
    }
  }

  async function handleSave() {
    setSaving(true)
    setError(null)
    try {
      const status = await saveTelegramConfig({
        bot_token: botToken.trim(),
        chat_id: chatID.trim(),
        chat_name: chatName.trim(),
      })
      onSaved(status)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  async function handleTest() {
    if (!botToken.trim() || !chatID.trim()) return
    setTesting(true)
    setError(null)
    setTestSuccess(false)
    try {
      await testTelegramConfig(botToken.trim(), chatID.trim())
      setTestSuccess(true)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Test failed')
    } finally {
      setTesting(false)
    }
  }

  async function handleDisconnect() {
    setDisconnecting(true)
    try {
      await deleteTelegramConfig()
      onSaved({ configured: false, has_token: false })
      onClose()
    } catch {
      setError('Failed to disconnect')
    } finally {
      setDisconnecting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm p-4">
      <div className="w-full max-w-md bg-white rounded-2xl shadow-xl border border-border overflow-hidden">
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-border">
          <div className="flex items-center gap-3">
            <div className="w-6 h-6 rounded flex items-center justify-center bg-[#2CA5E0]/10 text-[#2CA5E0] font-bold text-sm">
              ✈
            </div>
            <h2 className="text-sm font-semibold text-text-primary">Connect Telegram</h2>
          </div>
          <button onClick={onClose} className="text-text-tertiary hover:text-text-primary p-1 rounded">
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="px-6 py-5 space-y-5">
          {/* Error */}
          {error && (
            <div className="flex items-start gap-2 text-sm text-red-600 bg-red-50 border border-red-200 rounded-lg p-3">
              <AlertCircle className="w-4 h-4 mt-0.5 flex-shrink-0" />
              <span>{error}</span>
            </div>
          )}

          {/* Setup guide */}
          <div className="bg-surface-secondary rounded-lg p-4 text-xs text-text-secondary space-y-1">
            <p className="font-medium text-text-primary mb-2">Setup steps</p>
            <p>
              1. Message{' '}
              <code className="bg-white px-1 rounded border border-border">@BotFather</code> →{' '}
              <code className="bg-white px-1 rounded border border-border">/newbot</code> → copy token
            </p>
            <p>2. Add the bot to your incidents group</p>
            <p>3. Send any message in the group</p>
            <p>4. Paste the token below and click "Fetch Group"</p>
          </div>

          {/* Bot token */}
          <div>
            <label className="block text-xs font-medium text-text-secondary mb-1">Bot Token</label>
            <input
              type="password"
              value={botToken}
              onChange={(e) => setBotToken(e.target.value)}
              placeholder="1234567890:ABCdef..."
              className="w-full h-9 rounded-lg border border-border bg-white px-3 text-sm text-text-primary placeholder-text-tertiary focus:outline-none focus:ring-2 focus:ring-brand-primary/30"
            />
          </div>

          {/* Step 1: Fetch chat ID */}
          {step === 'token' && (
            <button
              onClick={handleFetchChatID}
              disabled={fetching || !botToken.trim()}
              className="w-full h-9 rounded-lg bg-brand-primary text-white text-sm font-medium hover:bg-brand-primary-hover disabled:opacity-50 flex items-center justify-center gap-2 transition-colors"
            >
              {fetching && <Loader2 className="w-4 h-4 animate-spin" />}
              {fetching ? 'Fetching…' : 'Fetch Group'}
            </button>
          )}

          {/* Step 2: Confirmed chat info + save */}
          {step === 'chat' && (
            <>
              <div>
                <label className="block text-xs font-medium text-text-secondary mb-1">
                  Group / Channel
                </label>
                <input
                  type="text"
                  value={chatName || chatID}
                  readOnly
                  className="w-full h-9 rounded-lg border border-border bg-surface-secondary px-3 text-sm text-text-primary"
                />
                <p className="text-[10px] text-text-tertiary mt-1">Chat ID: {chatID}</p>
              </div>

              {testSuccess && (
                <div className="flex items-center gap-2 text-sm text-green-600">
                  <CheckCircle className="w-4 h-4" />
                  Test message sent — check your Telegram group.
                </div>
              )}

              <div className="flex gap-2">
                <button
                  onClick={handleTest}
                  disabled={testing}
                  className="flex-1 h-9 rounded-lg border border-border text-sm text-text-secondary hover:bg-surface-secondary disabled:opacity-50 flex items-center justify-center gap-2 transition-colors"
                >
                  {testing ? <Loader2 className="w-4 h-4 animate-spin" /> : <Send className="w-3.5 h-3.5" />}
                  {testing ? 'Sending…' : 'Send test'}
                </button>
                <button
                  onClick={handleSave}
                  disabled={saving}
                  className="flex-1 h-9 rounded-lg bg-brand-primary text-white text-sm font-medium hover:bg-brand-primary-hover disabled:opacity-50 flex items-center justify-center gap-2 transition-colors"
                >
                  {saving && <Loader2 className="w-4 h-4 animate-spin" />}
                  {saving ? 'Saving…' : 'Save'}
                </button>
              </div>
            </>
          )}

          {/* Disconnect (only if already configured) */}
          {existing?.configured && (
            <button
              onClick={handleDisconnect}
              disabled={disconnecting}
              className="w-full text-xs text-red-500 hover:text-red-600 text-center py-1"
            >
              {disconnecting ? 'Disconnecting…' : 'Disconnect Telegram'}
            </button>
          )}
        </div>
      </div>
    </div>
  )
}
