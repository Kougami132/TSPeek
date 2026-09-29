import { useState, useEffect, useCallback, useRef } from 'react'
import type { ActivityEvent, ActivityPage } from '../types'

export function useActivities(pageSize = 50) {
  const [items, setItems] = useState<ActivityEvent[]>([])
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)
  const [totalPages, setTotalPages] = useState(0)
  const [loading, setLoading] = useState(true)
  const [unseenCount, setUnseenCount] = useState(0)

  const currentPageRef = useRef(page)
  currentPageRef.current = page

  const fetchPage = useCallback(
    async (pageToFetch: number) => {
      setLoading(true)
      try {
        const resp = await fetch(
          `/api/v1/activities?page=${pageToFetch}&page_size=${pageSize}`,
          { cache: 'no-store' }
        )
        if (!resp.ok) {
          throw new Error('failed to fetch activities')
        }
        const data: ActivityPage = await resp.json()
        setItems(data.items ?? [])
        setPage(data.page)
        setTotal(data.total)
        setTotalPages(data.total_pages)
        if (pageToFetch === 1) {
          setUnseenCount(0)
        }
      } catch (err) {
        console.error('Error fetching activities:', err)
      } finally {
        setLoading(false)
      }
    },
    [pageSize]
  )

  useEffect(() => {
    fetchPage(1)
  }, [fetchPage])

  // 监听 SSE 实时推送的 activity 事件
  useEffect(() => {
    const handleActivity = (event: Event) => {
      const customEvent = event as CustomEvent<ActivityEvent[]>
      const newEvents = customEvent.detail
      if (!newEvents || newEvents.length === 0) return

      if (currentPageRef.current === 1) {
        // 当前在第 1 页：平滑追加到列表顶部并去重
        setItems((prev) => {
          const existingIds = new Set(prev.map((item) => item.id))
          const fresh = newEvents.filter((item) => !existingIds.has(item.id))
          return [...fresh, ...prev]
        })
        setTotal((prev) => {
          const next = prev + newEvents.length
          setTotalPages(Math.ceil(next / pageSize))
          return next
        })
      } else {
        // 用户正在查阅历史页面（第 2 页及之后）：不打扰阅读，累加未读提示计数
        setUnseenCount((prev) => prev + newEvents.length)
        setTotal((prev) => {
          const next = prev + newEvents.length
          setTotalPages(Math.ceil(next / pageSize))
          return next
        })
      }
    }

    window.addEventListener('tspeek:activity', handleActivity)
    return () => {
      window.removeEventListener('tspeek:activity', handleActivity)
    }
  }, [pageSize, total])

  const goToPage = useCallback(
    (targetPage: number) => {
      fetchPage(targetPage)
    },
    [fetchPage]
  )

  return {
    items,
    page,
    pageSize,
    total,
    totalPages,
    loading,
    unseenCount,
    goToPage,
    refresh: () => fetchPage(page),
  }
}
