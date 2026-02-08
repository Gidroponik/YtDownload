<script setup>
import { ref, computed, watch, onMounted } from 'vue'

const url = ref('')
const loading = ref(false)
const error = ref('')
const videoInfo = ref(null)
const mode = ref('video') // 'video' or 'audio'

// Download progress state
const downloading = ref(false)
const stage = ref('')
const percent = ref(-1)

// History
const HISTORY_KEY = 'yturl_history'
const showHistory = ref(false)
const history = ref(loadHistory())

function loadHistory() {
  try {
    return JSON.parse(localStorage.getItem(HISTORY_KEY) || '[]')
  } catch { return [] }
}

function saveHistory() {
  localStorage.setItem(HISTORY_KEY, JSON.stringify(history.value))
}

function addToHistory(entry) {
  // Remove previous entry with same URL + format + mode to avoid duplicates
  history.value = history.value.filter(
    h => !(h.url === entry.url && h.formatId === entry.formatId && h.mode === entry.mode)
  )
  history.value.unshift(entry)
  if (history.value.length > 50) history.value = history.value.slice(0, 50)
  saveHistory()
}

function removeFromHistory(index) {
  history.value.splice(index, 1)
  saveHistory()
}

function clearHistory() {
  history.value = []
  saveHistory()
}

function formatDate(iso) {
  const d = new Date(iso)
  const day = d.toLocaleDateString('en-GB', { day: '2-digit', month: 'short' })
  const time = d.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' })
  return `${day}, ${time}`
}

const platformNames = { youtube: 'YouTube', tiktok: 'TikTok', instagram: 'Instagram' }

const stageLabel = computed(() => {
  switch (stage.value) {
    case 'downloading_video': return 'Downloading video'
    case 'downloading_audio': return 'Downloading audio'
    case 'merging': return 'Merging tracks'
    case 'converting': return 'Converting to MP3'
    case 'done': return 'Complete'
    default: return 'Preparing'
  }
})

const stagePercent = computed(() => {
  if (percent.value < 0) return ''
  return `${Math.round(percent.value)}%`
})

const platformLabel = computed(() => {
  if (!videoInfo.value?.platform) return ''
  return platformNames[videoInfo.value.platform] || videoInfo.value.platform
})

// Clear video info when input is emptied
watch(url, (v) => {
  if (!v.trim() && !downloading.value) {
    videoInfo.value = null
    error.value = ''
  }
})

// Re-fetch formats when mode changes (if video info is loaded)
watch(mode, () => {
  if (videoInfo.value && url.value.trim()) {
    fetchInfo()
  }
})

async function fetchInfo() {
  if (!url.value.trim()) return

  loading.value = true
  error.value = ''
  videoInfo.value = null

  try {
    const res = await fetch('/api/video/info', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url: url.value.trim(), mode: mode.value }),
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || 'Something went wrong')
    videoInfo.value = data
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}

function formatSize(bytes) {
  if (!bytes || bytes <= 0) return ''
  const units = ['B', 'KB', 'MB', 'GB']
  let i = 0
  let size = bytes
  while (size >= 1024 && i < units.length - 1) {
    size /= 1024
    i++
  }
  return `${size.toFixed(size >= 100 ? 0 : 1)} ${units[i]}`
}

function downloadVideo(format) {
  startDownload({
    url: url.value.trim(),
    formatId: format.formatId,
    quality: format.quality,
    size: format.size,
    mode: mode.value,
    title: videoInfo.value?.title || '',
    platform: videoInfo.value?.platform || 'unknown',
    thumbnail: videoInfo.value?.thumbnail || '',
  })
}

function redownloadFromHistory(item) {
  showHistory.value = false
  startDownload({
    url: item.url,
    formatId: item.formatId,
    quality: item.quality,
    size: item.size,
    mode: item.mode,
    title: item.title,
    platform: item.platform,
    thumbnail: item.thumbnail || '',
  })
}

function reopenFromHistory(item) {
  window.open(item.url, '_blank')
}

function startDownload({ url: dlUrl, formatId, quality, size, mode: dlMode, title, platform, thumbnail }) {
  downloading.value = true
  stage.value = dlMode === 'audio' ? 'downloading_audio' : 'downloading_video'
  percent.value = 0
  error.value = ''

  // Set input URL so progress UI has context
  url.value = dlUrl

  const params = new URLSearchParams({
    url: dlUrl,
    format: formatId,
    mode: dlMode,
  })

  const es = new EventSource(`/api/video/download?${params}`)

  es.onmessage = (event) => {
    const data = JSON.parse(event.data)

    stage.value = data.stage
    percent.value = data.percent

    if (data.stage === 'error') {
      es.close()
      downloading.value = false
      error.value = data.error || 'Download failed'
      return
    }

    if (data.stage === 'done') {
      es.close()
      const link = document.createElement('a')
      link.href = `/api/video/file/${data.fileId}`
      link.download = ''
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)

      addToHistory({
        date: new Date().toISOString(),
        platform,
        title,
        url: dlUrl,
        thumbnail,
        formatId,
        quality,
        size,
        mode: dlMode,
      })

      setTimeout(() => {
        downloading.value = false
        stage.value = ''
        percent.value = -1
      }, 600)
    }
  }

  es.onerror = () => {
    es.close()
    downloading.value = false
    error.value = 'Connection lost'
  }
}

function clearInput() {
  url.value = ''
  videoInfo.value = null
  error.value = ''
}

// Telegram bot
const botUsername = ref('')

onMounted(async () => {
  try {
    const res = await fetch('/api/bot/info')
    const data = await res.json()
    if (data.connected && data.username) {
      botUsername.value = data.username
    }
  } catch {}
})

function handleKeydown(e) {
  if (e.key === 'Enter') fetchInfo()
}
</script>

<template>
  <div>
    <!-- Platform icons header -->
    <div class="flex items-center justify-center gap-5 mb-6">
      <svg class="w-6 h-6 text-zinc-500" viewBox="0 0 24 24" fill="currentColor">
        <path d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z"/>
      </svg>
      <svg class="w-5 h-5 text-zinc-500" viewBox="0 0 24 24" fill="currentColor">
        <path d="M19.59 6.69a4.83 4.83 0 0 1-3.77-4.25V2h-3.45v13.67a2.89 2.89 0 0 1-2.88 2.5 2.89 2.89 0 0 1-2.89-2.89 2.89 2.89 0 0 1 2.89-2.89c.3 0 .59.04.86.12V9.01a6.27 6.27 0 0 0-.86-.06 6.34 6.34 0 0 0-6.34 6.34 6.34 6.34 0 0 0 6.34 6.34 6.34 6.34 0 0 0 6.34-6.34V8.75a8.18 8.18 0 0 0 4.78 1.53V6.84a4.84 4.84 0 0 1-1.02-.15z"/>
      </svg>
      <svg class="w-5 h-5 text-zinc-500" viewBox="0 0 24 24" fill="currentColor">
        <path d="M12 2.163c3.204 0 3.584.012 4.85.07 3.252.148 4.771 1.691 4.919 4.919.058 1.265.069 1.645.069 4.849 0 3.205-.012 3.584-.069 4.849-.149 3.225-1.664 4.771-4.919 4.919-1.266.058-1.644.07-4.85.07-3.204 0-3.584-.012-4.849-.07-3.26-.149-4.771-1.699-4.919-4.92-.058-1.265-.07-1.644-.07-4.849 0-3.204.013-3.583.07-4.849.149-3.227 1.664-4.771 4.919-4.919 1.266-.057 1.645-.069 4.849-.069zM12 0C8.741 0 8.333.014 7.053.072 2.695.272.273 2.69.073 7.052.014 8.333 0 8.741 0 12c0 3.259.014 3.668.072 4.948.2 4.358 2.618 6.78 6.98 6.98C8.333 23.986 8.741 24 12 24c3.259 0 3.668-.014 4.948-.072 4.354-.2 6.782-2.618 6.979-6.98.059-1.28.073-1.689.073-4.948 0-3.259-.014-3.667-.072-4.947-.196-4.354-2.617-6.78-6.979-6.98C15.668.014 15.259 0 12 0zm0 5.838a6.162 6.162 0 1 0 0 12.324 6.162 6.162 0 0 0 0-12.324zM12 16a4 4 0 1 1 0-8 4 4 0 0 1 0 8zm6.406-11.845a1.44 1.44 0 1 0 0 2.881 1.44 1.44 0 0 0 0-2.881z"/>
      </svg>
    </div>

    <!-- Subtitle -->
    <p class="text-center text-[13px] text-zinc-600 mb-4">Paste YouTube, TikTok or Instagram video URL</p>

    <!-- Input -->
    <div class="relative">
      <input
        v-model="url"
        @keydown="handleKeydown"
        type="text"
        placeholder="Paste link here"
        :disabled="downloading"
        class="w-full bg-zinc-900/80 border border-zinc-800 rounded-2xl pl-5 pr-20 py-4 text-[15px] text-white placeholder-zinc-600 focus:outline-none focus:ring-1 focus:ring-zinc-700 disabled:opacity-40 transition-all"
      />
      <button
        v-if="url.trim() && !downloading"
        @click="clearInput"
        class="absolute right-16 top-1/2 -translate-y-1/2 w-6 h-6 flex items-center justify-center rounded-full text-zinc-600 hover:text-zinc-300 hover:bg-zinc-800 transition-all"
      >
        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
        </svg>
      </button>
      <button
        @click="fetchInfo"
        :disabled="loading || !url.trim() || downloading"
        class="absolute right-2.5 top-1/2 -translate-y-1/2 bg-white text-black text-sm font-medium px-4 py-1.5 rounded-lg disabled:opacity-20 hover:bg-zinc-200 active:scale-95 transition-all"
      >
        <svg v-if="loading" class="animate-spin h-4 w-4 mx-1" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="3" fill="none" />
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
        </svg>
        <span v-else>Go</span>
      </button>
    </div>

    <!-- History button -->
    <div v-if="history.length && !videoInfo && !downloading" class="mt-3 flex justify-center">
      <button
        @click="showHistory = true"
        class="text-[12px] text-zinc-600 hover:text-zinc-400 transition-colors flex items-center gap-1.5"
      >
        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 6v6l4 2m6-2a10 10 0 11-20 0 10 10 0 0120 0z" />
        </svg>
        {{ history.length }} recent download{{ history.length === 1 ? '' : 's' }}
      </button>
    </div>

    <!-- Error -->
    <p v-if="error" class="mt-4 text-[13px] text-red-400 px-1">{{ error }}</p>

    <!-- Progress view (standalone, works for both normal and history downloads) -->
    <div v-if="downloading" class="mt-10 fade-in">
      <div class="flex items-baseline justify-between mb-3">
        <p class="text-[13px] text-zinc-400">{{ stageLabel }}</p>
        <p class="text-[13px] text-zinc-500 tabular-nums">{{ stagePercent }}</p>
      </div>
      <div class="h-1 bg-zinc-800/80 rounded-full overflow-hidden">
        <div
          v-if="percent >= 0"
          class="h-full bg-white rounded-full transition-all duration-500 ease-out"
          :style="{ width: Math.max(percent, 1) + '%' }"
        ></div>
        <div
          v-else
          class="h-full w-1/4 bg-white rounded-full animate-indeterminate"
        ></div>
      </div>
    </div>

    <!-- Video info -->
    <div v-if="videoInfo && !downloading" class="mt-10 fade-in">
      <!-- Video card -->
      <div class="flex gap-4 items-start">
        <img
          v-if="videoInfo.thumbnail"
          :src="videoInfo.thumbnail"
          :alt="videoInfo.title"
          class="w-36 aspect-video rounded-xl object-cover flex-shrink-0"
        />
        <div class="min-w-0 py-0.5">
          <p class="text-[15px] font-medium text-white leading-snug line-clamp-2">{{ videoInfo.title }}</p>
          <p class="text-[13px] text-zinc-500 mt-1">{{ videoInfo.author }}</p>
          <div class="flex items-center gap-2 mt-0.5">
            <p class="text-[12px] text-zinc-600">{{ videoInfo.duration }}</p>
            <span class="text-zinc-800">&middot;</span>
            <span class="inline-flex items-center gap-1 text-[11px] text-zinc-500 bg-zinc-900 px-1.5 py-0.5 rounded">
              <svg v-if="videoInfo.platform === 'youtube'" class="w-3 h-3" viewBox="0 0 24 24" fill="currentColor">
                <path d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z"/>
              </svg>
              <svg v-else-if="videoInfo.platform === 'tiktok'" class="w-3 h-3" viewBox="0 0 24 24" fill="currentColor">
                <path d="M19.59 6.69a4.83 4.83 0 0 1-3.77-4.25V2h-3.45v13.67a2.89 2.89 0 0 1-2.88 2.5 2.89 2.89 0 0 1-2.89-2.89 2.89 2.89 0 0 1 2.89-2.89c.3 0 .59.04.86.12V9.01a6.27 6.27 0 0 0-.86-.06 6.34 6.34 0 0 0-6.34 6.34 6.34 6.34 0 0 0 6.34 6.34 6.34 6.34 0 0 0 6.34-6.34V8.75a8.18 8.18 0 0 0 4.78 1.53V6.84a4.84 4.84 0 0 1-1.02-.15z"/>
              </svg>
              <svg v-else-if="videoInfo.platform === 'instagram'" class="w-3 h-3" viewBox="0 0 24 24" fill="currentColor">
                <path d="M12 2.163c3.204 0 3.584.012 4.85.07 3.252.148 4.771 1.691 4.919 4.919.058 1.265.069 1.645.069 4.849 0 3.205-.012 3.584-.069 4.849-.149 3.225-1.664 4.771-4.919 4.919-1.266.058-1.644.07-4.85.07-3.204 0-3.584-.012-4.849-.07-3.26-.149-4.771-1.699-4.919-4.92-.058-1.265-.07-1.644-.07-4.849 0-3.204.013-3.583.07-4.849.149-3.227 1.664-4.771 4.919-4.919 1.266-.057 1.645-.069 4.849-.069zM12 0C8.741 0 8.333.014 7.053.072 2.695.272.273 2.69.073 7.052.014 8.333 0 8.741 0 12c0 3.259.014 3.668.072 4.948.2 4.358 2.618 6.78 6.98 6.98C8.333 23.986 8.741 24 12 24c3.259 0 3.668-.014 4.948-.072 4.354-.2 6.782-2.618 6.979-6.98.059-1.28.073-1.689.073-4.948 0-3.259-.014-3.667-.072-4.947-.196-4.354-2.617-6.78-6.979-6.98C15.668.014 15.259 0 12 0zm0 5.838a6.162 6.162 0 1 0 0 12.324 6.162 6.162 0 0 0 0-12.324zM12 16a4 4 0 1 1 0-8 4 4 0 0 1 0 8zm6.406-11.845a1.44 1.44 0 1 0 0 2.881 1.44 1.44 0 0 0 0-2.881z"/>
              </svg>
              {{ platformLabel }}
            </span>
          </div>
        </div>
      </div>

      <!-- Mode toggle -->
      <div class="mt-6 flex gap-1 bg-zinc-900/80 rounded-xl p-1 w-fit">
        <button
          @click="mode = 'video'"
          :class="mode === 'video' ? 'bg-zinc-700 text-white' : 'text-zinc-500 hover:text-zinc-300'"
          class="text-[13px] font-medium px-4 py-1.5 rounded-lg transition-all"
        >
          Video
        </button>
        <button
          @click="mode = 'audio'"
          :class="mode === 'audio' ? 'bg-zinc-700 text-white' : 'text-zinc-500 hover:text-zinc-300'"
          class="text-[13px] font-medium px-4 py-1.5 rounded-lg transition-all"
        >
          MP3
        </button>
      </div>

      <!-- Format list -->
      <div class="mt-4 fade-in">
        <button
          v-for="format in videoInfo.formats"
          :key="format.formatId"
          @click="downloadVideo(format)"
          class="w-full flex items-center justify-between py-3.5 border-b border-zinc-900/80 last:border-0 group active:opacity-60 transition-opacity"
        >
          <div class="flex items-baseline gap-3">
            <span class="text-[15px] font-medium text-white tabular-nums w-24">{{ format.quality }}</span>
            <span v-if="formatSize(format.size)" class="text-[12px] text-zinc-600">{{ formatSize(format.size) }}</span>
          </div>
          <svg class="w-[18px] h-[18px] text-zinc-700 group-hover:text-white transition-colors" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3" />
          </svg>
        </button>
      </div>
    </div>

    <!-- Telegram bot link -->
    <div v-if="botUsername" class="mt-12 flex justify-center">
      <a
        :href="`https://t.me/${botUsername}`"
        target="_blank"
        class="flex items-center gap-2 text-[12px] text-zinc-600 hover:text-zinc-400 transition-colors"
      >
        <svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
          <path d="M11.944 0A12 12 0 0 0 0 12a12 12 0 0 0 12 12 12 12 0 0 0 12-12A12 12 0 0 0 12 0a12 12 0 0 0-.056 0zm4.962 7.224c.1-.002.321.023.465.14a.506.506 0 0 1 .171.325c.016.093.036.306.02.472-.18 1.898-.962 6.502-1.36 8.627-.168.9-.499 1.201-.82 1.23-.696.065-1.225-.46-1.9-.902-1.056-.693-1.653-1.124-2.678-1.8-1.185-.78-.417-1.21.258-1.91.177-.184 3.247-2.977 3.307-3.23.007-.032.014-.15-.056-.212s-.174-.041-.249-.024c-.106.024-1.793 1.14-5.061 3.345-.48.33-.913.49-1.302.48-.428-.008-1.252-.241-1.865-.44-.752-.245-1.349-.374-1.297-.789.027-.216.325-.437.893-.663 3.498-1.524 5.83-2.529 6.998-3.014 3.332-1.386 4.025-1.627 4.476-1.635z"/>
        </svg>
        Also available as Telegram bot
      </a>
    </div>

    <!-- History modal -->
    <Teleport to="body">
      <Transition name="modal">
        <div
          v-if="showHistory"
          class="fixed inset-0 z-50 flex items-end sm:items-center justify-center"
          @click.self="showHistory = false"
        >
          <div class="absolute inset-0 bg-black/70 backdrop-blur-sm" @click="showHistory = false"></div>

          <div class="relative w-full max-w-lg max-h-[80vh] bg-zinc-950 border border-zinc-800/80 rounded-2xl sm:rounded-2xl rounded-b-none sm:rounded-b-2xl overflow-hidden flex flex-col mx-0 sm:mx-4">
            <!-- Header -->
            <div class="flex items-center justify-between px-5 py-4 border-b border-zinc-800/60">
              <div class="flex items-center gap-2">
                <svg class="w-4 h-4 text-zinc-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M12 6v6l4 2m6-2a10 10 0 11-20 0 10 10 0 0120 0z" />
                </svg>
                <h2 class="text-[15px] font-medium text-white">History</h2>
                <span class="text-[11px] text-zinc-600 bg-zinc-900 px-1.5 py-0.5 rounded">{{ history.length }}</span>
              </div>
              <div class="flex items-center gap-2">
                <button
                  v-if="history.length"
                  @click="clearHistory"
                  class="text-[12px] text-zinc-600 hover:text-red-400 transition-colors"
                >
                  Clear all
                </button>
                <button
                  @click="showHistory = false"
                  class="w-7 h-7 flex items-center justify-center rounded-lg text-zinc-500 hover:text-white hover:bg-zinc-800 transition-all"
                >
                  <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>
            </div>

            <!-- List -->
            <div class="overflow-y-auto flex-1 overscroll-contain">
              <div v-if="!history.length" class="px-5 py-12 text-center">
                <p class="text-[13px] text-zinc-600">No downloads yet</p>
              </div>

              <div
                v-for="(item, i) in history"
                :key="i"
                class="group px-4 py-3 border-b border-zinc-900/60 last:border-0 hover:bg-zinc-900/40 transition-colors"
              >
                <div class="flex gap-3">
                  <!-- Thumbnail -->
                  <div class="w-20 flex-shrink-0">
                    <div class="aspect-video rounded-lg overflow-hidden bg-zinc-900">
                      <img
                        v-if="item.thumbnail"
                        :src="item.thumbnail"
                        :alt="item.title"
                        class="w-full h-full object-cover"
                      />
                      <div v-else class="w-full h-full flex items-center justify-center">
                        <svg class="w-5 h-5 text-zinc-800" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M15.75 10.5l4.72-4.72a.75.75 0 011.28.53v11.38a.75.75 0 01-1.28.53l-4.72-4.72M4.5 18.75h9a2.25 2.25 0 002.25-2.25v-9a2.25 2.25 0 00-2.25-2.25h-9A2.25 2.25 0 002.25 7.5v9a2.25 2.25 0 002.25 2.25z" />
                        </svg>
                      </div>
                    </div>
                  </div>

                  <!-- Info -->
                  <div class="min-w-0 flex-1">
                    <p class="text-[13px] text-white leading-snug line-clamp-1">
                      {{ item.title || item.url }}
                    </p>
                    <div class="flex items-center gap-1.5 mt-1 flex-wrap">
                      <!-- Platform pill -->
                      <span class="inline-flex items-center gap-1 text-[10px] text-zinc-500 bg-zinc-900 px-1.5 py-0.5 rounded">
                        <svg v-if="item.platform === 'youtube'" class="w-2.5 h-2.5" viewBox="0 0 24 24" fill="currentColor">
                          <path d="M23.498 6.186a3.016 3.016 0 0 0-2.122-2.136C19.505 3.545 12 3.545 12 3.545s-7.505 0-9.377.505A3.017 3.017 0 0 0 .502 6.186C0 8.07 0 12 0 12s0 3.93.502 5.814a3.016 3.016 0 0 0 2.122 2.136c1.871.505 9.376.505 9.376.505s7.505 0 9.377-.505a3.015 3.015 0 0 0 2.122-2.136C24 15.93 24 12 24 12s0-3.93-.502-5.814zM9.545 15.568V8.432L15.818 12l-6.273 3.568z"/>
                        </svg>
                        <svg v-else-if="item.platform === 'tiktok'" class="w-2.5 h-2.5" viewBox="0 0 24 24" fill="currentColor">
                          <path d="M19.59 6.69a4.83 4.83 0 0 1-3.77-4.25V2h-3.45v13.67a2.89 2.89 0 0 1-2.88 2.5 2.89 2.89 0 0 1-2.89-2.89 2.89 2.89 0 0 1 2.89-2.89c.3 0 .59.04.86.12V9.01a6.27 6.27 0 0 0-.86-.06 6.34 6.34 0 0 0-6.34 6.34 6.34 6.34 0 0 0 6.34 6.34 6.34 6.34 0 0 0 6.34-6.34V8.75a8.18 8.18 0 0 0 4.78 1.53V6.84a4.84 4.84 0 0 1-1.02-.15z"/>
                        </svg>
                        <svg v-else-if="item.platform === 'instagram'" class="w-2.5 h-2.5" viewBox="0 0 24 24" fill="currentColor">
                          <path d="M12 2.163c3.204 0 3.584.012 4.85.07 3.252.148 4.771 1.691 4.919 4.919.058 1.265.069 1.645.069 4.849 0 3.205-.012 3.584-.069 4.849-.149 3.225-1.664 4.771-4.919 4.919-1.266.058-1.644.07-4.85.07-3.204 0-3.584-.012-4.849-.07-3.26-.149-4.771-1.699-4.919-4.92-.058-1.265-.07-1.644-.07-4.849 0-3.204.013-3.583.07-4.849.149-3.227 1.664-4.771 4.919-4.919 1.266-.057 1.645-.069 4.849-.069zM12 0C8.741 0 8.333.014 7.053.072 2.695.272.273 2.69.073 7.052.014 8.333 0 8.741 0 12c0 3.259.014 3.668.072 4.948.2 4.358 2.618 6.78 6.98 6.98C8.333 23.986 8.741 24 12 24c3.259 0 3.668-.014 4.948-.072 4.354-.2 6.782-2.618 6.979-6.98.059-1.28.073-1.689.073-4.948 0-3.259-.014-3.667-.072-4.947-.196-4.354-2.617-6.78-6.979-6.98C15.668.014 15.259 0 12 0zm0 5.838a6.162 6.162 0 1 0 0 12.324 6.162 6.162 0 0 0 0-12.324zM12 16a4 4 0 1 1 0-8 4 4 0 0 1 0 8zm6.406-11.845a1.44 1.44 0 1 0 0 2.881 1.44 1.44 0 0 0 0-2.881z"/>
                        </svg>
                        {{ platformNames[item.platform] || item.platform }}
                      </span>
                      <span class="text-[10px] text-zinc-500 bg-zinc-900 px-1.5 py-0.5 rounded uppercase">{{ item.mode === 'audio' ? 'mp3' : 'mp4' }}</span>
                      <span class="text-[10px] text-zinc-600">{{ item.quality }}</span>
                      <span v-if="item.size" class="text-[10px] text-zinc-700">{{ formatSize(item.size) }}</span>
                      <span class="text-[10px] text-zinc-700">{{ formatDate(item.date) }}</span>
                    </div>
                  </div>

                  <!-- Actions -->
                  <div class="flex items-center gap-0.5 flex-shrink-0 opacity-0 group-hover:opacity-100 transition-opacity">
                    <!-- Re-open (load into main UI) -->
                    <button
                      @click="reopenFromHistory(item)"
                      title="Open"
                      class="w-7 h-7 flex items-center justify-center rounded-lg text-zinc-700 hover:text-white hover:bg-zinc-800/80 transition-all"
                    >
                      <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M13.5 6H5.25A2.25 2.25 0 003 8.25v10.5A2.25 2.25 0 005.25 21h10.5A2.25 2.25 0 0018 18.75V10.5m-10.5 6L21 3m0 0h-5.25M21 3v5.25" />
                      </svg>
                    </button>
                    <!-- Re-download -->
                    <button
                      @click="redownloadFromHistory(item)"
                      title="Download again"
                      class="w-7 h-7 flex items-center justify-center rounded-lg text-zinc-700 hover:text-white hover:bg-zinc-800/80 transition-all"
                    >
                      <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3" />
                      </svg>
                    </button>
                    <!-- Delete -->
                    <button
                      @click="removeFromHistory(i)"
                      title="Remove"
                      class="w-7 h-7 flex items-center justify-center rounded-lg text-zinc-700 hover:text-red-400 hover:bg-zinc-800/80 transition-all"
                    >
                      <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                      </svg>
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}
.modal-enter-active .relative,
.modal-leave-active .relative {
  transition: transform 0.2s ease;
}
.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
.modal-enter-from .relative {
  transform: translateY(20px) scale(0.98);
}
.modal-leave-to .relative {
  transform: translateY(10px) scale(0.99);
}
</style>
