import { apiClient } from './client'

export interface TelegramConfigStatus {
  configured: boolean
  chat_id?: string
  chat_name?: string
  has_token: boolean
  connected_at?: string
}

export interface SaveTelegramConfigRequest {
  bot_token: string
  chat_id: string
  chat_name?: string
}

export async function getTelegramConfig(): Promise<TelegramConfigStatus> {
  return apiClient.get<TelegramConfigStatus>('/api/v1/settings/telegram')
}

export async function saveTelegramConfig(req: SaveTelegramConfigRequest): Promise<TelegramConfigStatus> {
  return apiClient.post<TelegramConfigStatus>('/api/v1/settings/telegram', req)
}

export async function testTelegramConfig(
  botToken: string,
  chatId: string
): Promise<{ bot_username: string; message: string }> {
  return apiClient.post('/api/v1/settings/telegram/test', { bot_token: botToken, chat_id: chatId })
}

export async function fetchTelegramChatID(
  botToken: string
): Promise<{ chat_id: string; chat_name: string }> {
  return apiClient.post('/api/v1/settings/telegram/fetch-chat-id', { bot_token: botToken })
}

export async function deleteTelegramConfig(): Promise<void> {
  return apiClient.delete('/api/v1/settings/telegram')
}
