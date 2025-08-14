import { useEffect, useRef, useState } from 'react'
import { createChat, listChats, listMessages, addUserMessage, listModels, chatStream, onToken, onDone, cancelStream } from './lib/ipc'
import Sidebar from './components/Sidebar'
import ChatView from './components/ChatView'
import Composer from './components/Composer'
import ModelSwitcher from './components/ModelSwitcher'

export default function App() {
  const [chats, setChats] = useState<any[]>([])
  const [activeChat, setActiveChat] = useState<any | null>(null)
  const [messages, setMessages] = useState<any[]>([])
  const [models, setModels] = useState<string[]>([])
  const [isStreaming, setIsStreaming] = useState(false)
  const streamIdRef = useRef<string | null>(null)

  useEffect(() => {
    (async () => {
      const [chs, mds] = await Promise.all([listChats(), listModels()])
      setChats(chs)
      setModels(mds)
      if (chs.length) {
        setActiveChat(chs[0])
        setMessages(await listMessages(chs[0].id))
      }
    })()
  }, [])

  useEffect(() => {
    if (!activeChat) return
    const unlistenToken = onToken(activeChat.id, token => {
      setMessages(prev => {
        const arr = [...prev]
        const last = arr[arr.length - 1]
        if (last && last.role === 'assistant' && isStreaming) {
          last.content += token
        }
        return arr
      })
    })
    const unlistenDone = onDone(activeChat.id, () => setIsStreaming(false))
    return () => { unlistenToken.then(u => u()) ; unlistenDone.then(u => u()) }
  }, [activeChat, isStreaming])

  async function handleNewChat() {
    const title = 'New chat'
    const model = models[0] ?? 'llama3:8b'
    const id = await createChat(title, model)
    const chs = await listChats()
    setChats(chs)
    const chat = chs.find((c: any) => c.id === id)
    setActiveChat(chat)
    setMessages([])
  }

  async function handleSend(text: string) {
    if (!activeChat) return
    await addUserMessage(activeChat.id, text)
    setMessages(prev => [...prev, { id: 'temp-u', role: 'user', content: text, created_at: Date.now() }, { id: 'temp-a', role: 'assistant', content: '', created_at: Date.now() }])
    setIsStreaming(true)
    const sid = await chatStream(activeChat.id)
    streamIdRef.current = sid
  }

  async function handleSwitch(chatId: string) {
    const ch = chats.find((c: any) => c.id === chatId)
    setActiveChat(ch)
    setMessages(await listMessages(ch.id))
  }

  async function handleCancel() {
    if (streamIdRef.current) {
      await cancelStream(streamIdRef.current)
    }
  }

  return (
    <div className="h-screen w-screen grid grid-cols-[260px_1fr]">
      <Sidebar chats={chats} activeId={activeChat?.id} onNew={handleNewChat} onSelect={handleSwitch} />
      <div className="flex flex-col h-full">
        <div className="flex items-center justify-between border-b border-zinc-800 px-4 py-2">
          <div className="font-semibold">{activeChat?.title ?? 'No chat selected'}</div>
          <ModelSwitcher models={models} chat={activeChat} onChanged={() => setChats([...chats])} />
        </div>
        <ChatView messages={messages} isStreaming={isStreaming} onCancel={handleCancel} />
        <Composer onSend={handleSend} disabled={!activeChat} />
      </div>
    </div>
  )
}
