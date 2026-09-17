import { Routes, Route, useNavigate, useLocation } from 'react-router-dom'
import { Button, Input, TabBar } from 'antd-mobile'
import { AppOutline, TeamOutline, AddCircleOutline, SetOutline } from 'antd-mobile-icons'
import { useEffect, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { Home, Parties, Create, Mine } from './pages'
import { api, auth } from './api'

const tabs = [
  { key: '/home', title: '首页', icon: <AppOutline /> },
  { key: '/parties', title: '往来', icon: <TeamOutline /> },
  { key: '/create', title: '记一笔', icon: <span className="seal-action"><AddCircleOutline /></span> },
  { key: '/mine', title: '我的', icon: <SetOutline /> },
]

export default function App() {
  const nav = useNavigate()
  const loc = useLocation()
  const activeKey = loc.pathname === '/' ? '/home' : loc.pathname
  const queryClient = useQueryClient()
  const [authRequired, setAuthRequired] = useState(auth.required())
  const [password, setPassword] = useState('')

  useEffect(() => {
    const onAuthRequired = () => setAuthRequired(true)
    window.addEventListener('trade-ledger-auth-required', onAuthRequired)
    return () => window.removeEventListener('trade-ledger-auth-required', onAuthRequired)
  }, [])

  const submitPassword = () => {
    if (!password) return
    auth.set(password)
    setPassword('')
    setAuthRequired(false)
    queryClient.invalidateQueries()
  }

  return (
    <div className="app-shell min-h-screen pb-24">
      <main>
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/home" element={<Home />} />
          <Route path="/parties" element={<Parties />} />
          <Route path="/create" element={<Create />} />
          <Route path="/mine" element={<Mine />} />
        </Routes>
      </main>
      <div className="nav-shell fixed bottom-0 w-full">
        <TabBar className="tl-tabbar" activeKey={activeKey} onChange={nav}>
          {tabs.map((t) => (
            <TabBar.Item key={t.key} icon={t.icon} title={t.title} />
          ))}
        </TabBar>
      </div>
      {authRequired && (
        <div className="fixed inset-0 z-20 flex items-center justify-center bg-ink/40 px-6">
          <div className="w-full max-w-sm rounded-2xl bg-white p-5 shadow-xl">
            <h2 className="font-display text-xl font-bold">输入账本口令</h2>
            <p className="mt-1 text-sm text-fog">口令只保存在当前浏览器会话里。</p>
            <Input
              className="mt-4"
              type="password"
              placeholder="TRADE_LEDGER_PASSWORD"
              value={password}
              onChange={setPassword}
              onEnterPress={submitPassword}
            />
            <Button block className="mt-3 border-none bg-seal font-bold text-white" disabled={!password} onClick={submitPassword}>
              进入账本
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
