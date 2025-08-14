import { useState } from 'react'

export default function Composer({ onSend, disabled }: { onSend: (t: string) => void, disabled?: boolean }) {
  const [v, setV] = useState('')
  return (
    <form className="p-3 border-t border-zinc-800" onSubmit={e => { e.preventDefault(); if (!v.trim()) return; onSend(v.trim()); setV('') }}>
      <div className="max-w-3xl flex gap-2">
        <textarea value={v} onChange={e => setV(e.target.value)} disabled={disabled}
          className="flex-1 resize-none h-[88px] rounded bg-zinc-900 border border-zinc-800 px-3 py-2 focus:outline-none"
          placeholder={disabled ? 'Create or select a chat...' : 'Type a message...'} />
        <button disabled={disabled} className="px-4 py-2 rounded bg-blue-600 hover:bg-blue-500 disabled:opacity-50">Send</button>
      </div>
    </form>
  )
}
