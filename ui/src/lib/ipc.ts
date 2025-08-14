import { invoke } from '@tauri-apps/api/core'
import { listen, emit } from '@tauri-apps/api/event'

export async function listModels(): Promise<string[]> {
  return invoke('list_models')
}

export async function listChats() {
  return invoke('list_chats')
}

export async function createChat(title: string, model: string) {
  return invoke('create_chat', { title, model })
}

export async function listMessages(chatId: string) {
  return invoke('list_messages', { chatId })
}

export async function addUserMessage(chatId: string, content: string) {
  return invoke('add_message', { chatId, role: 'user', content })
}

export async function chatStream(chatId: string) {
  // Start stream; returns a stream id used for cancellation
  return invoke<string>('chat_stream', { chatId })
}

export async function cancelStream(streamId: string) {
  return invoke('cancel_stream', { streamId })
}

export function onToken(chatId: string, cb: (t: string) => void) {
  return listen<string>(`chat-token:${chatId}`, e => cb(e.payload))
}

export function onDone(chatId: string, cb: () => void) {
  return listen(`chat-done:${chatId}`, cb)
}

export function emitToast(message: string) {
  return emit('toast', { message })
}
