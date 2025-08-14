import Loader from './Loader'

export default function ChatView({ messages, isStreaming, onCancel }: { messages: any[], isStreaming: boolean, onCancel: () => void }) {
  return (
    <div className="flex-1 overflow-auto p-4 space-y-4 scrollbar">
      {messages.map((m, i) => (
        <div key={i} className="max-w-3xl">
          <div className={m.role === 'user' ? 'text-zinc-100' : 'text-zinc-200'}>
            <div className="text-xs mb-1 text-zinc-500">{m.role}</div>
            <div className="whitespace-pre-wrap leading-relaxed">{m.content}</div>
          </div>
        </div>
      ))}

      {isStreaming && (
        <div className="max-w-3xl flex items-center gap-3 text-zinc-400">
          <Loader />
          <button onClick={onCancel} className="text-xs px-2 py-1 bg-zinc-800 rounded">Stop</button>
        </div>
      )}
    </div>
  )
}
