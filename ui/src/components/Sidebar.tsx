interface Props {
  chats: any[]
  activeId?: string
  onNew: () => void
  onSelect: (id: string) => void
}

export default function Sidebar({ chats, activeId, onNew, onSelect }: Props) {
  return (
    <div className="h-full border-r border-zinc-800 flex flex-col">
      <div className="px-3 py-2 flex items-center justify-between border-b border-zinc-800">
        <div className="font-semibold">Chats</div>
        <button className="text-sm px-2 py-1 bg-zinc-800 rounded" onClick={onNew}>New</button>
      </div>
      <div className="flex-1 overflow-auto">
        {chats.map(c => (
          <button key={c.id} onClick={() => onSelect(c.id)} className={`w-full text-left px-3 py-2 hover:bg-zinc-900 ${activeId === c.id ? 'bg-zinc-900' : ''}`}>
            <div className="truncate">{c.title}</div>
            <div className="text-xs text-zinc-400 truncate">{c.model}</div>
          </button>
        ))}
      </div>
    </div>
  )
}
