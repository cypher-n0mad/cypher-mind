export type Role = 'user' | 'assistant' | 'system'

export interface Chat {
  id: string
  title: string
  created_at: number
  updated_at: number
  model: string
}

export interface Message {
  id: string
  chat_id: string
  role: Role
  content: string
  created_at: number
}

export interface OllamaTag { name: string }
