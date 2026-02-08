<script setup>
import { ref, computed, watch } from 'vue'

const url = ref('')
const loading = ref(false)
const error = ref('')
const videoInfo = ref(null)
const mode = ref('video') // 'video' or 'audio'

// Download progress state
const downloading = ref(false)
const stage = ref('')
const percent = ref(-1)

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
  downloading.value = true
  stage.value = mode.value === 'audio' ? 'downloading_audio' : 'downloading_video'
  percent.value = 0
  error.value = ''

  const params = new URLSearchParams({
    url: url.value.trim(),
    format: format.formatId,
    mode: mode.value,
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

function handleKeydown(e) {
  if (e.key === 'Enter') fetchInfo()
}
</script>

<template>
  <div>
    <!-- Input -->
    <div class="relative">
      <input
        v-model="url"
        @keydown="handleKeydown"
        type="text"
        placeholder="Paste YouTube link"
        :disabled="downloading"
        class="w-full bg-zinc-900/80 border border-zinc-800 rounded-2xl pl-5 pr-20 py-4 text-[15px] text-white placeholder-zinc-600 focus:outline-none focus:ring-1 focus:ring-zinc-700 disabled:opacity-40 transition-all"
      />
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

    <!-- Error -->
    <p v-if="error" class="mt-4 text-[13px] text-red-400 px-1">{{ error }}</p>

    <!-- Video info -->
    <div v-if="videoInfo" class="mt-10 fade-in">
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
          <p class="text-[12px] text-zinc-600 mt-0.5">{{ videoInfo.duration }}</p>
        </div>
      </div>

      <!-- Mode toggle -->
      <div v-if="!downloading" class="mt-6 flex gap-1 bg-zinc-900/80 rounded-xl p-1 w-fit">
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

      <!-- Progress view -->
      <div v-if="downloading" class="mt-8 fade-in">
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

      <!-- Format list -->
      <div v-else class="mt-4 fade-in">
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
  </div>
</template>
