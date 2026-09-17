import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { List, Button, Input, Toast, Selector, Popup } from 'antd-mobile'
import { useState } from 'react'
import { Link } from 'react-router-dom'
import { api, yuan } from './api'
import type { Counterparty, Overview, Trade, TradeItem } from './api'

const TYPE_LABEL: Record<string, string> = {
  sale: '销售',
  service: '服务',
  purchase: '采购',
  transfer: '调串货',
}

const TAG_LABEL: Record<string, string> = {
  customer: '客户',
  supplier: '供货商',
  peer: '同行',
  individual: '个人',
}

const TAG_OPTIONS = Object.entries(TAG_LABEL).map(([value, label]) => ({ value, label }))

const day = (iso: string) => {
  const y = new Date().getFullYear().toString()
  return iso.startsWith(y) ? iso.slice(5, 10) : iso.slice(0, 10)
}

// 老版本硬编码写进去的系统备注，显示时当没写过
const LEGACY_NOTE = '手机端新增'
const cleanNote = (note: string) => (note === LEGACY_NOTE ? '' : note)

function PageTitle({ children, sub }: { children: string; sub?: string }) {
  return (
    <header className="page-title px-4 pt-6 pb-2">
      <h1 className="font-display text-3xl font-black">{children}</h1>
      {sub && <p className="mt-1 text-sm text-fog">{sub}</p>}
    </header>
  )
}

// 后端时间线把类型拼在备注最前面（"sale 装机两台"），这里拆回人话。
function splitTradeNote(note: string): [string, string] {
  const i = note.indexOf(' ')
  if (i > 0 && TYPE_LABEL[note.slice(0, i)]) return [TYPE_LABEL[note.slice(0, i)], cleanNote(note.slice(i + 1))]
  if (TYPE_LABEL[note]) return [TYPE_LABEL[note], '']
  return ['', cleanNote(note)]
}

function SettleSheet({ trade, name, onClose }: { trade: Trade; name: string; onClose: () => void }) {
  const [payAmount, setPayAmount] = useState('')
  const [remaining, setRemaining] = useState(trade.remaining_cents)
  const qc = useQueryClient()
  const verb = trade.direction === 'in' ? '收款' : '付款'
  const pay = useMutation<{ paid_cents: number }>({
    mutationFn: () => api.pay(trade.id, toCents(payAmount)),
    onSuccess: (res) => {
      setRemaining(trade.total_cents - res.paid_cents)
      setPayAmount('')
      qc.invalidateQueries({ queryKey: ['overview'] })
      qc.invalidateQueries({ queryKey: ['parties'] })
      Toast.show('钱记上了')
    },
    onError: () => Toast.show('这次的钱比剩下的多，改小一点'),
  })
  return (
    <div className="p-4 pb-8">
      <p className="font-bold">
        {name} · {TYPE_LABEL[trade.type] ?? trade.type}
      </p>
      <p className="money mt-2 text-4xl font-bold">
        还剩 ¥{yuan(remaining)}
      </p>
      <p className="money mt-1 text-sm text-fog">
        共 ¥{yuan(trade.total_cents)}，已{verb} ¥{yuan(trade.total_cents - remaining)}
      </p>
      {remaining <= 0 ? (
        <div className="mt-4 text-center">
          <span className="seal seal-red seal-land text-base">已结清</span>
          <div><Button className="mt-4" onClick={onClose}>关掉</Button></div>
        </div>
      ) : (
        <>
          <div className="mt-4 flex gap-2">
            <Input placeholder={`这次${verb}多少`} inputMode="decimal" value={payAmount} onChange={setPayAmount} />
            <Button
              className="shrink-0 border-none bg-seal font-bold text-white"
              disabled={!moneyValid(payAmount) || pay.isPending}
              onClick={() => pay.mutate()}
            >
              记{verb}
            </Button>
          </div>
          {payAmount && !moneyValid(payAmount) && (
            <p className="mt-1 text-sm text-seal">金额填纯数字，最多两位小数</p>
          )}
        </>
      )}
    </div>
  )
}

export function Home() {
  const { data, isLoading } = useQuery<Overview>({
    queryKey: ['overview'],
    queryFn: api.overview,
  })
  const { data: parties } = useQuery<Counterparty[]>({ queryKey: ['parties'], queryFn: api.counterparties })
  const [selected, setSelected] = useState<Trade | null>(null)
  const nameOf = (id: string) => parties?.find((c) => c.id === id)?.name ?? '未知往来'
  if (isLoading) return <p className="p-4">翻账本…</p>
  const empty = !data || data.recent.length === 0
  return (
    <div>
      <PageTitle sub="谁欠谁，一眼看清">往来账</PageTitle>

      <section className="balance-panel mx-4 mt-3 flex py-4">
        <div className="flex-1 text-center">
          <p className="text-sm text-fog">待收</p>
          <p className="money mt-1 text-3xl font-bold text-carbon">¥{yuan(data?.receivable_cents ?? 0)}</p>
        </div>
        <div className="w-px self-stretch" style={{ background: 'var(--color-rule)' }} />
        <div className="flex-1 text-center">
          <p className="text-sm text-fog">待付</p>
          <p className="money mt-1 text-3xl font-bold text-seal">¥{yuan(data?.payable_cents ?? 0)}</p>
        </div>
      </section>

      <section className="section-block px-4 pt-6">
        <h2 className="section-heading font-display text-lg font-bold">近期流水</h2>
        {!empty && <p className="mt-0.5 text-xs text-fog">点一行，就能记收款 / 付款</p>}
        {empty ? (
          <div className="tl-panel mt-2 py-8 text-center">
            <p className="text-fog">账本还是空的，先记第一笔吧</p>
            <Link to="/create">
              <Button className="mt-3 border-none bg-seal font-bold text-white">记第一笔</Button>
            </Link>
          </div>
        ) : (
          <div className="tl-panel recent-panel mt-3">
          <List className="tl-list">
            {data?.recent.map((t) => (
              <List.Item key={t.id} className="ledger-row" clickable arrow={false} onClick={() => setSelected(t)}>
                <div className="flex items-center justify-between gap-2 py-1">
                  <div>
                    <p>
                      <span className="font-bold">{nameOf(t.counterparty_id)}</span>
                      <span className="text-fog"> · {TYPE_LABEL[t.type] ?? t.type} · {t.direction === 'in' ? '别人欠我' : '我欠别人'}</span>
                    </p>
                    <p className="mt-0.5 text-xs text-fog">
                      {cleanNote(t.note) ? `${cleanNote(t.note)} · ` : ''}{day(t.created_at)}
                    </p>
                  </div>
                  <div className="text-right">
                    <p className="money font-bold">¥{yuan(t.total_cents)}</p>
                    {t.remaining_cents <= 0 ? (
                      <span className="seal seal-red seal-land mt-1">已结清</span>
                    ) : (
                      <p className="money mt-0.5 text-xs text-carbon">剩 ¥{yuan(t.remaining_cents)}</p>
                    )}
                  </div>
                </div>
              </List.Item>
            ))}
          </List>
          </div>
        )}
      </section>

      <Popup visible={!!selected} onMaskClick={() => setSelected(null)}>
        {selected && (
          <SettleSheet trade={selected} name={nameOf(selected.counterparty_id)} onClose={() => setSelected(null)} />
        )}
      </Popup>
    </div>
  )
}

function BalanceSummary({ counterparty, compact = false }: { counterparty: Counterparty; compact?: boolean }) {
  const receivable = counterparty.receivable_cents ?? 0
  const payable = counterparty.payable_cents ?? 0
  const clear = receivable === 0 && payable === 0
  return (
    <div className={`party-balance ${compact ? 'party-balance-compact' : 'party-balance-detail'}`}>
      {(!compact || receivable > 0) && (
        <p className="balance-receivable">
          <span>应收</span>
          <strong>¥{yuan(receivable)}</strong>
        </p>
      )}
      {(!compact || payable > 0) && (
        <p className="balance-payable">
          <span>应付</span>
          <strong>¥{yuan(payable)}</strong>
        </p>
      )}
      {compact && clear && <p className="balance-clear">已结清</p>}
    </div>
  )
}

export function Parties() {
  const { data } = useQuery<Counterparty[]>({ queryKey: ['parties'], queryFn: api.counterparties })
  const [name, setName] = useState('')
  const [phone, setPhone] = useState('')
  const [note, setNote] = useState('')
  const [tags, setTags] = useState<string[]>([])
  const qc = useQueryClient()
  const mut = useMutation<Counterparty>({
    mutationFn: () => api.createCounterparty({ name: name.trim(), phone: phone.trim(), note: note.trim(), tags }),
    onSuccess: () => {
      setName('')
      setPhone('')
      setNote('')
      setTags([])
      qc.invalidateQueries({ queryKey: ['parties'] })
      Toast.show('记下了')
    },
    onError: () => Toast.show('没记上，起个名字再试'),
  })
  const [timeline, setTimeline] = useState<{ counterparty: Counterparty; list: Awaited<ReturnType<typeof api.timeline>>; trades: Trade[] } | null>(null)
  const [offsetReceivableID, setOffsetReceivableID] = useState('')
  const [offsetPayableID, setOffsetPayableID] = useState('')
  const [offsetAmount, setOffsetAmount] = useState('')
  const loadParty = (counterparty: Counterparty) => {
    setOffsetReceivableID('')
    setOffsetPayableID('')
    setOffsetAmount('')
    return Promise.all([api.timeline(counterparty.id), api.trades(counterparty.id), api.counterparties()]).then(([list, trades, counterparties]) => {
      const current = counterparties.find((item) => item.id === counterparty.id) ?? counterparty
      setTimeline({ counterparty: current, list, trades })
    })
  }
  const offsetMut = useMutation({
    mutationFn: () => api.offset(offsetReceivableID, offsetPayableID, toCents(offsetAmount)),
    onSuccess: () => {
      const current = timeline?.counterparty
      setOffsetReceivableID('')
      setOffsetPayableID('')
      setOffsetAmount('')
      qc.invalidateQueries({ queryKey: ['overview'] })
      qc.invalidateQueries({ queryKey: ['parties'] })
      if (current) void loadParty(current)
      Toast.show('抵扣记下了')
    },
    onError: () => Toast.show('抵扣金额不能超过两边剩余金额'),
  })
  const receivableTrades = timeline?.trades.filter((trade) => trade.direction === 'in' && trade.remaining_cents > 0) ?? []
  const payableTrades = timeline?.trades.filter((trade) => trade.direction === 'out' && trade.remaining_cents > 0) ?? []
  return (
    <div>
      <PageTitle sub="点一行，看它的整本流水">往来对象</PageTitle>
      <div className="search-row flex gap-2 px-4">
        <Input placeholder="新对象名字，比如老王电脑" value={name} onChange={setName} />
        <Button
          className="shrink-0 border-none bg-seal font-bold text-white"
          disabled={!name.trim()}
          onClick={() => mut.mutate()}
        >
          加一个
        </Button>
      </div>
      <div className="mt-2 flex gap-2 px-4">
        <Input className="flex-1" placeholder="电话（可不填）" value={phone} onChange={setPhone} />
        <Input className="flex-1" placeholder="备注（可不填）" value={note} onChange={setNote} />
      </div>
      <Selector className="mt-2 px-4" options={TAG_OPTIONS} value={tags} onChange={setTags} />
      {(!data || data.length === 0) && (
        <p className="px-4 pt-6 text-center text-fog">还没有往来对象，上面起个名字就行</p>
      )}
      <div className="tl-panel party-panel mx-4 mt-4">
      <List className="tl-list">
        {data?.map((c) => (
          <List.Item
            key={c.id}
            className="ledger-row"
            clickable
            arrow={<span className="text-lg text-fog">›</span>}
            onClick={() => void loadParty(c)}
          >
            <div className="party-row py-1">
              <div className="min-w-0">
                <p className="truncate font-bold">{c.name}</p>
                <p className="mt-0.5 text-xs text-fog">{c.tags.map((t) => TAG_LABEL[t] ?? t).join('、')}</p>
              </div>
              <BalanceSummary counterparty={c} compact />
            </div>
          </List.Item>
        ))}
      </List>
      </div>
      {timeline && (
        <section className="timeline-section px-4 pt-6">
          <h2 className="section-heading font-display text-lg font-bold">{timeline.counterparty.name}的流水</h2>
          <BalanceSummary counterparty={timeline.counterparty} />
          {receivableTrades.length > 0 && payableTrades.length > 0 && (
            <div className="mt-4 rounded-2xl border border-rule bg-white/70 p-3">
              <p className="font-bold">应收应付抵扣</p>
              <div className="mt-2 grid gap-2">
                <Selector
                  options={receivableTrades.map((trade) => ({ value: trade.id, label: `应收 · ${TYPE_LABEL[trade.type] ?? trade.type} · 剩 ¥${yuan(trade.remaining_cents)}` }))}
                  value={offsetReceivableID ? [offsetReceivableID] : []}
                  onChange={(value) => setOffsetReceivableID(value[0] ?? '')}
                />
                <Selector
                  options={payableTrades.map((trade) => ({ value: trade.id, label: `应付 · ${TYPE_LABEL[trade.type] ?? trade.type} · 剩 ¥${yuan(trade.remaining_cents)}` }))}
                  value={offsetPayableID ? [offsetPayableID] : []}
                  onChange={(value) => setOffsetPayableID(value[0] ?? '')}
                />
                <div className="flex gap-2">
                  <Input className="flex-1" placeholder="抵扣金额" inputMode="decimal" value={offsetAmount} onChange={setOffsetAmount} />
                  <Button
                    className="shrink-0 border-none bg-seal font-bold text-white"
                    disabled={!offsetReceivableID || !offsetPayableID || !moneyValid(offsetAmount) || offsetMut.isPending}
                    onClick={() => offsetMut.mutate()}
                  >
                    记抵扣
                  </Button>
                </div>
              </div>
            </div>
          )}
          {timeline.list.length === 0 ? (
            <p className="pt-3 text-fog">这家还没发生过业务，去记一笔吧</p>
          ) : (
            <div className="mt-2 flex flex-col gap-3">
              {timeline.list.map((e) => {
                const [kindLabel, rest] = e.kind === 'payment' ? ['结算', e.note] : e.kind === 'offset' ? ['抵扣', e.note] : splitTradeNote(e.note)
                const label = kindLabel || '记账'
                return (
                  <div key={e.kind + e.id} className={`rail ${e.kind !== 'trade' ? 'rail-settle' : ''}`}>
                    <div className="flex items-baseline justify-between gap-2">
                      <p>
                        {label}
                        {rest ? <span className="text-fog"> · {rest}</span> : null}
                      </p>
                      <p className="money shrink-0 font-bold">¥{yuan(e.amount_cents)}</p>
                    </div>
                    <p className="mt-0.5 text-xs text-fog">{day(e.at)}</p>
                  </div>
                )
              })}
            </div>
          )}
        </section>
      )}
    </div>
  )
}

const TRADE_TYPES = [
  { label: '销售', value: 'sale' },
  { label: '服务', value: 'service' },
  { label: '采购', value: 'purchase' },
  { label: '调串货', value: 'transfer' },
]

const TRANSFER_DIRS = [
  { label: '调入，别人欠我', value: 'in' },
  { label: '调出，我欠别人', value: 'out' },
]

const moneyValid = (s: string) => /^\d+(\.\d{1,2})?$/.test(s.trim()) && parseFloat(s) > 0
const toCents = (s: string) => Math.round(parseFloat(s.trim()) * 100)
type ItemDraft = { name: string; qty: string; price: string }

function Steps({ done }: { done: boolean[] }) {
  const labels = ['选对象', '记金额', '记钱']
  return (
    <div className="steps flex items-center gap-1 px-4 pt-3 text-[13px]">
      {labels.map((l, i) => (
        <span key={l} className="flex items-center gap-1">
          {i > 0 && <span className="mx-1 text-fog">→</span>}
          <span
            className={`rounded-full px-2.5 py-1 ${done[i] ? 'bg-ink text-white' : i === done.findIndex((d) => !d) ? 'bg-seal text-white' : 'bg-ink/10 text-fog'}`}
          >
            {done[i] ? '✓ ' : ''}{l}
          </span>
        </span>
      ))}
    </div>
  )
}

export function Create() {
  const { data: parties } = useQuery<Counterparty[]>({ queryKey: ['parties'], queryFn: api.counterparties })
  const [partyId, setPartyId] = useState('')
  const [newName, setNewName] = useState('')
  const [newPhone, setNewPhone] = useState('')
  const [newNote, setNewNote] = useState('')
  const [newTags, setNewTags] = useState<string[]>([])
  const [type, setType] = useState('sale')
  const [transferDir, setTransferDir] = useState('in')
  const [amount, setAmount] = useState('')
  const [note, setNote] = useState('')
  const [trade, setTrade] = useState<Trade | null>(null)
  const [itemDrafts, setItemDrafts] = useState<ItemDraft[]>([])
  const [payAmount, setPayAmount] = useState('')
  const qc = useQueryClient()

  const party = parties?.find((c) => c.id === partyId)
  const dir = type === 'transfer' ? transferDir : type === 'purchase' ? 'out' : 'in'
  const oweText = dir === 'in' ? '别人欠我（应收）' : '我欠别人（应付）'
  const payVerb = dir === 'in' ? '收款' : '付款'
  const amountOk = moneyValid(amount)
  const itemsOk = itemDrafts.every((item) => item.name.trim() && /^\d+$/.test(item.qty) && Number(item.qty) > 0 && moneyValid(item.price))
  const payOk = trade && moneyValid(payAmount)

  const addParty = useMutation<Counterparty>({
    mutationFn: () => api.createCounterparty({ name: newName.trim(), phone: newPhone.trim(), note: newNote.trim(), tags: newTags }),
    onSuccess: (c) => {
      setPartyId(c.id)
      setNewName('')
      setNewPhone('')
      setNewNote('')
      setNewTags([])
      qc.invalidateQueries({ queryKey: ['parties'] })
      Toast.show(`认准${c.name}了，继续往下记`)
    },
    onError: () => Toast.show('没加上，换个名字试试'),
  })

  const createTrade = useMutation<Trade>({
    mutationFn: () => api.createTrade(
      partyId,
      type,
      toCents(amount),
      note.trim(),
      dir,
      itemDrafts.map((item) => ({ name: item.name.trim(), qty: Number(item.qty), price_cents: toCents(item.price) })),
    ),
    onSuccess: (t) => {
      setTrade(t)
      qc.invalidateQueries({ queryKey: ['overview'] })
      qc.invalidateQueries({ queryKey: ['parties'] })
      Toast.show('这笔记下了')
    },
    onError: () => Toast.show('没记上，看看金额对不对'),
  })

  const pay = useMutation<{ paid_cents: number }>({
    mutationFn: () => trade ? api.pay(trade.id, toCents(payAmount)) : Promise.reject(new Error('no trade')),
    onSuccess: (res) => {
      if (trade) setTrade({ ...trade, paid_cents: res.paid_cents, remaining_cents: trade.total_cents - res.paid_cents })
      setPayAmount('')
      qc.invalidateQueries({ queryKey: ['overview'] })
      qc.invalidateQueries({ queryKey: ['parties'] })
      Toast.show('钱记上了')
    },
    onError: () => Toast.show('这次的钱比剩下的多，改小一点'),
  })

  const again = () => {
    setTrade(null)
    setAmount('')
    setNote('')
    setPayAmount('')
    setItemDrafts([])
  }

  return (
    <div>
      <PageTitle sub="选对象，记金额，记钱，钱不到位第三步可跳过">记一笔</PageTitle>
      <Steps done={[!!party, !!trade, !!trade && trade.remaining_cents <= 0]} />

      <section className="form-section px-4 pt-5">
        <h2 className="section-heading font-display text-lg font-bold">一、跟谁的生意</h2>
        {party ? (
          <p className="selected-party mt-3 p-3">
            跟 <span className="font-bold">{party.name}</span> 的生意
            <button className="ml-2 text-sm text-carbon" onClick={() => { setPartyId(''); setTrade(null) }}>换一家</button>
          </p>
        ) : (
          <>
            {(parties ?? []).length > 0 && (
              <Selector
                className="mt-2"
                options={(parties ?? []).map((c) => ({ label: c.name, value: c.id }))}
                onChange={(v) => setPartyId(v[0] ?? '')}
              />
            )}
            <div className="mt-2 flex gap-2">
              <Input placeholder="名单里没有？现起一个，比如老王电脑" value={newName} onChange={setNewName} />
              <Button
                className="shrink-0 border-none bg-seal font-bold text-white"
                disabled={!newName.trim()}
                onClick={() => addParty.mutate()}
              >
                加上
              </Button>
            </div>
            <div className="mt-2 flex gap-2">
              <Input className="flex-1" placeholder="电话（可不填）" value={newPhone} onChange={setNewPhone} />
              <Input className="flex-1" placeholder="备注（可不填）" value={newNote} onChange={setNewNote} />
            </div>
            <Selector className="mt-2" options={TAG_OPTIONS} value={newTags} onChange={setNewTags} />
          </>
        )}
      </section>

      <section className="form-section trade-form-section px-4 pt-6">
        <h2 className="section-heading font-display text-lg font-bold">二、什么事，多少钱</h2>
        <div className="trade-type-field mt-4">
          <p className="field-label">业务类型</p>
          <Selector
            className="trade-type-selector mt-2"
            options={TRADE_TYPES}
            value={[type]}
            onChange={(v) => { setType(v[0]); setTrade(null) }}
          />
          {type === 'transfer' && (
            <Selector
              className="trade-direction-selector mt-2"
              options={TRANSFER_DIRS}
              value={[transferDir]}
              onChange={(v) => setTransferDir(v[0])}
            />
          )}
        </div>
        <div className={`trade-direction trade-direction-${dir} mt-3`}>
          <span>记为</span>
          <strong>{oweText}</strong>
        </div>
        <div className="trade-detail-panel mt-3">
          <div className="amount-field">
            <span className="field-label">金额</span>
            <Input placeholder="比如 1200" inputMode="decimal" value={amount} onChange={setAmount} />
          </div>
          <div className="note-field mt-3">
            <span className="field-label">备注</span>
            <Input placeholder="可不填，比如装机两台" value={note} onChange={setNote} />
          </div>
          <div className="mt-4">
            <div className="flex items-center justify-between">
              <span className="field-label">明细（可不填）</span>
              <button
                type="button"
                className="text-sm font-bold text-carbon"
                onClick={() => setItemDrafts((current) => [...current, { name: '', qty: '1', price: '' }])}
              >
                + 加一行
              </button>
            </div>
            {itemDrafts.map((item, index) => (
              <div key={index} className="mt-2 grid grid-cols-[minmax(0,1fr)_58px_92px_auto] items-center gap-2">
                <Input placeholder="商品或服务" value={item.name} onChange={(name) => setItemDrafts((current) => current.map((row, i) => i === index ? { ...row, name } : row))} />
                <Input placeholder="数量" inputMode="numeric" value={item.qty} onChange={(qty) => setItemDrafts((current) => current.map((row, i) => i === index ? { ...row, qty } : row))} />
                <Input placeholder="单价" inputMode="decimal" value={item.price} onChange={(price) => setItemDrafts((current) => current.map((row, i) => i === index ? { ...row, price } : row))} />
                <button type="button" className="text-sm text-seal" onClick={() => setItemDrafts((current) => current.filter((_, i) => i !== index))}>删</button>
              </div>
            ))}
          </div>
        </div>
        <Button
          block
          className="trade-submit mt-4 font-bold"
          disabled={createTrade.isPending}
          onClick={() => {
            if (!party) {
              Toast.show('先选第一步：跟谁的生意')
              return
            }
            if (!amountOk) {
              Toast.show('金额填纯数字，比如 1200')
              return
            }
            if (!itemsOk) {
              Toast.show('明细请填名称、正整数数量和单价')
              return
            }
            createTrade.mutate()
          }}
        >
          记下来
        </Button>
        {!party && <p className="mt-1 text-sm text-fog">还没选对象，点了会提醒你回去选</p>}
        {party && amount && !amountOk && <p className="mt-1 text-sm text-seal">金额填纯数字，最多两位小数，比如 1200</p>}
        {party && itemDrafts.length > 0 && !itemsOk && <p className="mt-1 text-sm text-seal">明细请填名称、正整数数量和单价</p>}
      </section>

      {trade && (
        <section className="trade-summary mx-4 mt-4 p-3">
          <div className="flex items-center justify-between gap-2">
            <p>
              <span className="font-bold">{party?.name}</span> · {TYPE_LABEL[trade.type] ?? trade.type} ·{' '}
              <span className="money font-bold">¥{yuan(trade.total_cents)}</span>
            </p>
            {trade.remaining_cents <= 0 && <span className="seal seal-red seal-land">已结清</span>}
          </div>
          <p className="money mt-1 text-sm text-carbon">还剩 ¥{yuan(trade.remaining_cents)}</p>
          {trade.items.length > 0 && (
            <div className="mt-2 border-t border-rule pt-2 text-sm text-fog">
              {trade.items.map((item, index) => <p key={`${index}-${item.name}`}>{item.name} × {item.qty} · ¥{yuan(item.price_cents)}</p>)}
            </div>
          )}
        </section>
      )}

      <section className="form-section px-4 pt-6">
        <h2 className="section-heading font-display text-lg font-bold">三、这次{payVerb}多少</h2>
        {!trade ? (
          <p className="mt-2 text-fog">第二步记下来之后，这里才能记钱；一分没收到就跳过，去首页看账就行</p>
        ) : trade.remaining_cents <= 0 ? (
          <div className="mt-2">
            <p className="text-fog">这笔已经两清，不用再记钱了</p>
            <Button className="mt-2" onClick={again}>再记一笔</Button>
          </div>
        ) : (
          <>
            <div className="mt-2 flex gap-2">
              <Input placeholder={`这次${payVerb}多少`} inputMode="decimal" value={payAmount} onChange={setPayAmount} />
              <Button
                className="shrink-0 border-none bg-seal font-bold text-white"
                disabled={!payOk || pay.isPending}
                onClick={() => pay.mutate()}
              >
                记{payVerb}
              </Button>
            </div>
            {payAmount && !moneyValid(payAmount) && (
              <p className="mt-1 text-sm text-seal">金额填纯数字，最多两位小数</p>
            )}
            <button className="mt-3 text-sm text-carbon" onClick={again}>这笔先这样，再记一笔</button>
          </>
        )}
      </section>
    </div>
  )
}

export function Mine() {
  return (
    <div>
      <PageTitle sub="机器自己的事">我的</PageTitle>
      <section className="px-4 pt-2 text-fog">
        <p>现在是单人记账，不用登录。</p>
        <p className="mt-2">以后设了密码，请求会带着口令，后台才认。</p>
      </section>
    </div>
  )
}
