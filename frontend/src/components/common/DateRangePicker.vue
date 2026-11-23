<template>
  <div class="relative" ref="containerRef">
    <button
      type="button"
      @click="toggle"
      :class="['date-picker-trigger', isOpen && 'date-picker-trigger-open']"
    >
      <span class="date-picker-icon">
        <Icon name="calendar" size="sm" />
      </span>
      <span class="date-picker-value">
        {{ displayValue }}
      </span>
      <span class="date-picker-chevron">
        <Icon
          name="chevronDown"
          size="sm"
          :class="['transition-transform duration-200', isOpen && 'rotate-180']"
        />
      </span>
    </button>

    <Transition name="date-picker-dropdown">
      <div v-if="isOpen" class="date-picker-dropdown">
        <!-- Quick presets -->
        <div class="date-picker-presets">
          <button
            v-for="preset in presets"
            :key="preset.value"
            @click="selectPreset(preset)"
            :class="['date-picker-preset', isPresetActive(preset) && 'date-picker-preset-active']"
          >
            {{ t(preset.labelKey) }}
          </button>
        </div>

        <div class="date-picker-divider"></div>

        <!-- Custom date range inputs -->
        <div class="date-picker-custom">
          <div class="date-picker-field">
            <label class="date-picker-label">{{ t('dates.startDate') }}</label>
            <input
              type="date"
              v-model="localStartDate"
              :max="localEndDate || tomorrow()"
              class="date-picker-input"
              @change="onDateChange"
            />
          </div>
          <div class="date-picker-separator">
            <Icon name="arrowRight" size="sm" class="text-gray-400" />
          </div>
          <div class="date-picker-field">
            <label class="date-picker-label">{{ t('dates.endDate') }}</label>
            <input
              type="date"
              v-model="localEndDate"
              :min="localStartDate"
              :max="tomorrow()"
              class="date-picker-input"
              @change="onDateChange"
            />
          </div>
        </div>

        <!-- Apply button -->
        <div class="date-picker-actions">
          <button @click="apply" class="date-picker-apply">
            {{ t('dates.apply') }}
          </button>
        </div>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

interface DatePreset {
  labelKey: string
  value: string
  getRange: () => { start: string; end: string }
}

interface Props {
  startDate: string
  endDate: string
  // Date-only reports omit this prop and use calendar-day presets.
  appliedPeriod?: 'dates' | '24h'
}

interface Emits {
  (e: 'update:startDate', value: string): void
  (e: 'update:endDate', value: string): void
  (e: 'change', range: { startDate: string; endDate: string; preset: string | null }): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const { t, locale } = useI18n()

const isOpen = ref(false)
const containerRef = ref<HTMLElement | null>(null)
const localStartDate = ref(props.startDate)
const localEndDate = ref(props.endDate)
const activePreset = ref<string | null>(null)

const today = () => formatDateToString(new Date())

// Tomorrow's date - used for max date to handle timezone differences
// When user is in a timezone behind the server, "today" on server might be "tomorrow" locally
const tomorrow = () => {
  const d = new Date()
  d.setDate(d.getDate() + 1)
  return formatDateToString(d)
}

// Helper function to format date to YYYY-MM-DD using local timezone
const formatDateToString = (date: Date): string => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const presets = computed<DatePreset[]>(() => [
  {
    labelKey: 'dates.today',
    value: 'today',
    getRange: () => {
      const t = today()
      return { start: t, end: t }
    }
  },
  {
    labelKey: 'dates.yesterday',
    value: 'yesterday',
    getRange: () => {
      const d = new Date()
      d.setDate(d.getDate() - 1)
      const yesterday = formatDateToString(d)
      return { start: yesterday, end: yesterday }
    }
  },
  {
    labelKey: props.appliedPeriod === undefined ? 'dates.yesterdayAndToday' : 'dates.last24Hours',
    value: props.appliedPeriod === undefined ? 'yesterdayAndToday' : 'last24Hours',
    getRange: () => {
      const end = new Date()
      const start = new Date(end)
      if (props.appliedPeriod === undefined) start.setDate(start.getDate() - 1)
      else start.setTime(end.getTime() - 24 * 60 * 60 * 1000)
      return {
        start: formatDateToString(start),
        end: formatDateToString(end)
      }
    }
  },
  {
    labelKey: 'dates.last7Days',
    value: '7days',
    getRange: () => {
      const end = today()
      const d = new Date()
      d.setDate(d.getDate() - 6)
      const start = formatDateToString(d)
      return { start, end }
    }
  },
  {
    labelKey: 'dates.last14Days',
    value: '14days',
    getRange: () => {
      const end = today()
      const d = new Date()
      d.setDate(d.getDate() - 13)
      const start = formatDateToString(d)
      return { start, end }
    }
  },
  {
    labelKey: 'dates.last30Days',
    value: '30days',
    getRange: () => {
      const end = today()
      const d = new Date()
      d.setDate(d.getDate() - 29)
      const start = formatDateToString(d)
      return { start, end }
    }
  },
  {
    labelKey: 'dates.thisMonth',
    value: 'thisMonth',
    getRange: () => {
      const now = new Date()
      const start = formatDateToString(new Date(now.getFullYear(), now.getMonth(), 1))
      return { start, end: today() }
    }
  },
  {
    labelKey: 'dates.lastMonth',
    value: 'lastMonth',
    getRange: () => {
      const now = new Date()
      const start = formatDateToString(new Date(now.getFullYear(), now.getMonth() - 1, 1))
      const end = formatDateToString(new Date(now.getFullYear(), now.getMonth(), 0))
      return { start, end }
    }
  }
])

const displayValue = computed(() => {
  if (activePreset.value) {
    const preset = presets.value.find((p) => p.value === activePreset.value)
    if (preset) return t(preset.labelKey)
  }

  if (localStartDate.value && localEndDate.value) {
    if (localStartDate.value === localEndDate.value) {
      return formatDate(localStartDate.value)
    }
    return `${formatDate(localStartDate.value)} - ${formatDate(localEndDate.value)}`
  }

  return t('dates.selectDateRange')
})

const formatDate = (dateStr: string): string => {
  const date = new Date(dateStr + 'T00:00:00')
  const dateLocale = locale.value === 'zh' ? 'zh-CN' : 'en-US'
  return date.toLocaleDateString(dateLocale, { month: 'short', day: 'numeric' })
}

const isPresetActive = (preset: DatePreset): boolean => {
  return activePreset.value === preset.value
}

const selectPreset = (preset: DatePreset) => {
  const range = preset.getRange()
  localStartDate.value = range.start
  localEndDate.value = range.end
  activePreset.value = preset.value
}

const onDateChange = () => {
  // Check if current dates match any preset
  activePreset.value = null
  for (const preset of presets.value) {
    if (preset.value === 'last24Hours') continue
    const range = preset.getRange()
    if (range.start === localStartDate.value && range.end === localEndDate.value) {
      activePreset.value = preset.value
      break
    }
  }
}

const toggle = () => {
  if (!isOpen.value) syncAppliedRange()
  isOpen.value = !isOpen.value
}

const apply = () => {
  emit('update:startDate', localStartDate.value)
  emit('update:endDate', localEndDate.value)
  emit('change', {
    startDate: localStartDate.value,
    endDate: localEndDate.value,
    preset: activePreset.value
  })
  isOpen.value = false
}

const handleClickOutside = (event: MouseEvent) => {
  if (containerRef.value && !containerRef.value.contains(event.target as Node)) {
    isOpen.value = false
  }
}

const handleEscape = (event: KeyboardEvent) => {
  if (event.key === 'Escape' && isOpen.value) {
    isOpen.value = false
  }
}

function syncAppliedRange() {
  if (props.appliedPeriod === '24h') {
    const rolling = presets.value.find(preset => preset.value === 'last24Hours')
    if (rolling) {
      selectPreset(rolling)
      return
    }
  }
  localStartDate.value = props.startDate
  localEndDate.value = props.endDate
  onDateChange()
}

watch(isOpen, (open) => {
  if (!open) syncAppliedRange()
}, { flush: 'post' })
watch(() => [props.startDate, props.endDate, props.appliedPeriod], syncAppliedRange, { immediate: true })

onMounted(() => {
  document.addEventListener('click', handleClickOutside)
  document.addEventListener('keydown', handleEscape)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside)
  document.removeEventListener('keydown', handleEscape)
})
</script>

<style scoped>
.date-picker-trigger {
  @apply flex items-center gap-2;
  @apply rounded-[14px] px-3 py-2 text-sm;
  background-color: var(--craft-surface, #fffdf8);
  border: 1px solid var(--craft-border-strong, #d5c09e);
  color: var(--craft-ink, #27221d);
  @apply transition-all duration-200;
  @apply cursor-pointer;
}

.date-picker-trigger:hover {
  background-color: var(--craft-surface-hover, #f7edda);
  border-color: var(--craft-sky-deep, #2a8bd4);
}

.date-picker-trigger:focus {
  outline: none;
  border-color: var(--craft-sky-deep, #2a8bd4);
  box-shadow: 0 0 0 3px var(--craft-sky-soft, #e6f5fe);
}

.date-picker-trigger-open {
  border-color: var(--craft-sky-deep, #2a8bd4);
  box-shadow: 0 0 0 3px var(--craft-sky-soft, #e6f5fe);
}

.date-picker-icon {
  color: var(--craft-muted, #756758);
}

.date-picker-value {
  @apply font-medium;
}

.date-picker-chevron {
  color: var(--craft-muted, #756758);
}

.date-picker-dropdown {
  @apply absolute left-0 z-[100] mt-2;
  background-color: var(--craft-surface, #fffdf8);
  border: 1px solid var(--craft-border-strong, #d5c09e);
  @apply rounded-2xl;
  box-shadow: 0 12px 30px rgba(72, 52, 26, 0.14);
  @apply overflow-hidden;
  @apply min-w-[320px];
}

.dark .date-picker-dropdown {
  box-shadow: 0 14px 34px rgba(0, 0, 0, 0.48);
}

.date-picker-presets {
  @apply grid grid-cols-2 gap-1 p-2;
}

.date-picker-preset {
  @apply rounded-xl px-3 py-1.5 text-xs font-medium;
  color: var(--craft-ink-soft, #4c4237);
  @apply transition-colors duration-150;
}

.date-picker-preset:hover {
  background-color: var(--craft-surface-hover, #f7edda);
  color: var(--craft-ink, #27221d);
}

.date-picker-preset-active {
  background-color: var(--craft-sky-soft, #e6f5fe);
  color: var(--craft-sky-deep, #2a8bd4);
  font-weight: 600;
}

.date-picker-divider {
  border-top: 1px solid var(--craft-border, #e6d7be);
}

.date-picker-custom {
  @apply flex items-end gap-2 p-3;
}

.date-picker-field {
  @apply flex-1;
}

.date-picker-label {
  @apply mb-1 block text-xs font-medium;
  color: var(--craft-muted, #756758);
}

.date-picker-input {
  @apply w-full rounded-xl px-2.5 py-1.5 text-sm;
  background-color: var(--craft-surface-muted, #faf2e2);
  border: 1px solid var(--craft-border-strong, #d5c09e);
  color: var(--craft-ink, #27221d);
}

.date-picker-input:focus {
  outline: none;
  border-color: var(--craft-sky-deep, #2a8bd4);
  box-shadow: 0 0 0 3px var(--craft-sky-soft, #e6f5fe);
}

.date-picker-input::-webkit-calendar-picker-indicator {
  @apply cursor-pointer opacity-60 hover:opacity-100;
  filter: invert(0.5);
}

.dark .date-picker-input::-webkit-calendar-picker-indicator {
  filter: none;
}

.date-picker-separator {
  @apply flex items-center justify-center pb-1;
}

.date-picker-actions {
  @apply flex justify-end p-2 pt-0;
}

.date-picker-apply {
  @apply rounded-xl px-4 py-1.5 text-sm font-semibold text-white;
  background-image: linear-gradient(180deg, #40a7ef 0%, #2682c8 100%);
  border: 1px solid rgba(25, 102, 165, 0.35);
  box-shadow: 0 2px 0 rgba(21, 90, 146, 0.25);
  @apply transition-all duration-150;
}

.date-picker-apply:hover {
  background-image: linear-gradient(180deg, #52b2f5 0%, #2d8fd8 100%);
}

/* Dropdown animation */
.date-picker-dropdown-enter-active,
.date-picker-dropdown-leave-active {
  transition: all 0.2s ease;
}

.date-picker-dropdown-enter-from,
.date-picker-dropdown-leave-to {
  opacity: 0;
  transform: translateY(-8px);
}
</style>
