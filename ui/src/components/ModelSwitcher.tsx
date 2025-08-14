import { useEffect, useState } from 'react'
import { invoke } from '@tauri-apps/api/core'

export default function ModelSwitcher({ models, chat, onChanged }: { models: string[], chat: any, onChanged: () => void }) {
  const [model, setModel] = useState(chat?.model)
  useEffect(() => setModel(chat?.model), [chat])

  async function updateModel(m: string) {
    if (!chat) return
    setModel(m)
    await invoke('set_chat_model', { chatId: chat.id, model: m })
    onChanged()
  }

  return (
    <select value={model} onChange={e => updateModel(e.target.value)} className="bg-zinc-900 border border-zinc-700 rounded px-2 py-1">
      {models.map(m => <option key={m} value={m}>{m}</option>)}
    </select>
  )
}
