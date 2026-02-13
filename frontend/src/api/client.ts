import type { ApiError } from './types'

const BASE_URL = import.meta.env.VITE_API_URL || ''

/**
 * ApiClient provides a typed wrapper around fetch with:
 * - Automatic JSON parsing
 * - Error handling and typed ApiError throwing
 * - Request/response logging in development
 */
class ApiClient {
  private async request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    const url = `${BASE_URL}${endpoint}`

    const config: RequestInit = {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
    }

    // Log requests in development
    if (import.meta.env.DEV) {
      console.log(`[API] ${options.method || 'GET'} ${endpoint}`)
    }

    try {
      const response = await fetch(url, config)

      // Handle 204 No Content - no body to parse
      if (response.status === 204) {
        if (import.meta.env.DEV) {
          console.log(`[API] ${options.method || 'GET'} ${endpoint} → 204 No Content`)
        }
        return undefined as T
      }

      // Parse JSON response
      const data = await response.json()

      // Handle non-2xx responses
      if (!response.ok) {
        const error = data as ApiError
        const message = error.error?.message || 'Request failed'
        const apiError = new Error(message) as Error & { response: ApiError }
        apiError.response = error
        throw apiError
      }

      if (import.meta.env.DEV) {
        console.log(`[API] ${options.method || 'GET'} ${endpoint} → ${response.status}`)
      }

      return data as T
    } catch (error) {
      if (error instanceof Error && 'response' in error) {
        // Already an API error with response attached
        throw error
      }

      // Network error or other exception
      if (error instanceof Error) {
        throw error
      }

      throw new Error('Network request failed')
    }
  }

  async get<T>(endpoint: string, params?: Record<string, string | number | undefined> | object): Promise<T> {
    let query = ''
    if (params) {
      const filteredParams = Object.entries(params)
        .filter(([, value]) => value !== undefined)
        .map(([key, value]) => [key, String(value)])

      if (filteredParams.length > 0) {
        query = '?' + new URLSearchParams(filteredParams).toString()
      }
    }

    return this.request<T>(endpoint + query, {
      method: 'GET',
    })
  }

  async post<T>(endpoint: string, body?: unknown): Promise<T> {
    return this.request<T>(endpoint, {
      method: 'POST',
      body: body ? JSON.stringify(body) : undefined,
    })
  }

  async patch<T>(endpoint: string, body?: unknown): Promise<T> {
    return this.request<T>(endpoint, {
      method: 'PATCH',
      body: body ? JSON.stringify(body) : undefined,
    })
  }

  async delete<T>(endpoint: string): Promise<T> {
    return this.request<T>(endpoint, {
      method: 'DELETE',
    })
  }
}

export const apiClient = new ApiClient()
