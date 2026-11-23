<template>
  <svg
    class="craft-icon"
    :class="[`craft-icon--${size}`]"
    viewBox="0 0 28 28"
    fill="none"
    aria-hidden="true"
  >
    <!-- Calendar (Today Requests) -->
    <template v-if="name === 'calendar'">
      <rect
        x="4"
        y="6.5"
        width="20"
        height="17.5"
        rx="4"
        fill="var(--craft-icon-paper, #fffaf0)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
      />
      <path
        d="M4 10.5C4 8.29 5.79 6.5 8 6.5H20C22.21 6.5 24 8.29 24 10.5V12.2H4V10.5Z"
        fill="var(--craft-icon-sky, #95d5fc)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
        stroke-linejoin="round"
      />
      <path
        d="M9.5 4.2V8.2M18.5 4.2V8.2"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2.2"
        stroke-linecap="round"
      />
      <circle cx="9.5" cy="16" r="1.35" fill="var(--craft-icon-ink, #27221d)" />
      <circle cx="14" cy="16" r="1.35" fill="var(--craft-icon-ink, #27221d)" />
      <circle cx="18.5" cy="16" r="1.35" fill="var(--craft-icon-peach, #ff9b70)" />
      <circle cx="9.5" cy="20" r="1.35" fill="var(--craft-icon-ink, #27221d)" />
      <circle cx="14" cy="20" r="1.35" fill="var(--craft-icon-ink, #27221d)" />
    </template>

    <!-- Coins (Today Cost) -->
    <template v-else-if="name === 'coins'">
      <ellipse
        cx="16.8"
        cy="19.5"
        rx="6.2"
        ry="3"
        fill="var(--craft-icon-gold-deep, #f5b942)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
      />
      <path
        d="M10.6 15.2V19.5C10.6 21.16 13.38 22.5 16.8 22.5C20.22 22.5 23 21.16 23 19.5V15.2"
        fill="var(--craft-icon-gold, #ffdc6b)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
        stroke-linejoin="round"
      />
      <ellipse
        cx="16.8"
        cy="15.2"
        rx="6.2"
        ry="3"
        fill="var(--craft-icon-gold, #ffe582)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
      />
      <path
        d="M5 9.8V18.8C5 20.46 7.78 21.8 11.2 21.8C14.62 21.8 17.4 20.46 17.4 18.8V9.8"
        fill="var(--craft-icon-gold, #ffdc6b)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
        stroke-linejoin="round"
      />
      <path
        d="M5 14.3C5 15.96 7.78 17.3 11.2 17.3C14.62 17.3 17.4 15.96 17.4 14.3"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="1.8"
      />
      <ellipse
        cx="11.2"
        cy="9.8"
        rx="6.2"
        ry="3"
        fill="var(--craft-icon-gold-light, #fff0a6)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
      />
    </template>

    <!-- Key (API Keys) -->
    <template v-else-if="name === 'key'">
      <path
        d="M15.8 6.4A5.4 5.4 0 1 1 11.6 15.2L6.2 20.6C5.6 21.2 5.4 21.8 5.9 22.3L7.2 23.6C7.6 24 8.2 24 8.7 23.5L10.3 21.9L11.7 23.2L13.7 21.2L12.4 19.9L14.5 17.8A5.4 5.4 0 0 1 15.8 6.4Z"
        fill="var(--craft-icon-gold, #ffe394)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <circle
        cx="17.8"
        cy="10.2"
        r="1.9"
        fill="var(--craft-icon-paper, #fffaf0)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="1.8"
      />
    </template>

    <!-- Layers (Today Tokens) -->
    <template v-else-if="name === 'layers'">
      <path
        d="M5 18.2L14 22.5L23 18.2"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <path
        d="M5 14L14 18.3L23 14"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <path
        d="M14 5.5L23 9.8L14 14.1L5 9.8L14 5.5Z"
        fill="var(--craft-icon-gold, #ffe08a)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
        stroke-linejoin="round"
      />
    </template>

    <!-- Bars (Total Tokens / Chart) -->
    <template v-else-if="name === 'bars' || name === 'chart'">
      <rect
        x="4.5"
        y="14.5"
        width="5"
        height="8.5"
        rx="2"
        fill="var(--craft-icon-paper, #fffaf0)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
      />
      <rect
        x="11.5"
        y="9.5"
        width="5"
        height="13.5"
        rx="2"
        fill="var(--craft-icon-sky, #95d5fc)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
      />
      <rect
        x="18.5"
        y="5.5"
        width="5"
        height="17.5"
        rx="2"
        fill="var(--craft-icon-peach, #ffad87)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
      />
    </template>

    <!-- Bolt (RPM / Performance) -->
    <template v-else-if="name === 'bolt'">
      <path
        d="M15.4 3.8L6.8 15.2H13.4L11.8 24.2L21.2 12.2H14.5L15.4 3.8Z"
        fill="var(--craft-icon-gold, #ffd954)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
        stroke-linejoin="round"
      />
    </template>

    <!-- Clock (Avg Response) -->
    <template v-else-if="name === 'clock'">
      <circle
        cx="14"
        cy="14"
        r="9.5"
        fill="var(--craft-icon-gold-light, #ffe88a)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
      />
      <circle cx="14" cy="14" r="6.8" fill="var(--craft-icon-paper, #fffaf0)" />
      <path
        d="M14 9.8V14.2L17.2 16.2"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2.1"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
    </template>

    <!-- Gift (Redeem) -->
    <template v-else-if="name === 'gift'">
      <rect
        x="5.5"
        y="12"
        width="17"
        height="11.5"
        rx="2.8"
        fill="var(--craft-icon-paper, #fffaf0)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
      />
      <rect
        x="4"
        y="8.5"
        width="20"
        height="4.4"
        rx="2"
        fill="var(--craft-icon-peach, #ffad87)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
      />
      <path
        d="M14 8.5V23.5"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
      />
      <path
        d="M14 8.5C11.5 8.5 9.2 7.1 9.2 5.4C9.2 4.1 10.6 3.5 12 4.3C13.2 5 13.8 6.8 14 8.5ZM14 8.5C16.5 8.5 18.8 7.1 18.8 5.4C18.8 4.1 17.4 3.5 16 4.3C14.8 5 14.2 6.8 14 8.5Z"
        fill="var(--craft-icon-gold, #ffe08a)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="1.9"
        stroke-linejoin="round"
      />
    </template>

    <!-- Sparkles (Batch Image) -->
    <template v-else-if="name === 'sparkles'">
      <path
        d="M14 4.5L16.3 10.8L22.5 13L16.3 15.2L14 21.5L11.7 15.2L5.5 13L11.7 10.8L14 4.5Z"
        fill="var(--craft-icon-sky, #95d5fc)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
        stroke-linejoin="round"
      />
      <circle cx="21.5" cy="6.5" r="1.8" fill="var(--craft-icon-gold, #ffd954)" stroke="var(--craft-icon-ink, #27221d)" stroke-width="1.6" />
    </template>

    <!-- User (Home Setup Step 1) -->
    <template v-else-if="name === 'user'">
      <circle
        cx="14"
        cy="9.2"
        r="4"
        fill="var(--craft-icon-sky-soft, #d7effe)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2.1"
      />
      <path
        d="M6.5 22.5C6.5 18.3 9.7 15.5 14 15.5C18.3 15.5 21.5 18.3 21.5 22.5"
        fill="var(--craft-icon-peach-soft, #ffe5d6)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2.1"
        stroke-linecap="round"
      />
    </template>

    <!-- Code Brackets (Home Setup Step 3) -->
    <template v-else-if="name === 'code'">
      <path
        d="M10.2 9.2L5.2 14L10.2 18.8"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2.4"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <path
        d="M17.8 9.2L22.8 14L17.8 18.8"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2.4"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <path
        d="M15.4 7.5L12.6 20.5"
        stroke="var(--craft-icon-peach, #ff8f5c)"
        stroke-width="2.2"
        stroke-linecap="round"
      />
    </template>

    <!-- Book (Home Docs Button) -->
    <template v-else-if="name === 'book'">
      <path
        d="M4.5 7.2C4.5 6.3 5.2 5.7 6.1 5.8C8.8 6.1 11.7 6.9 14 8.4C16.3 6.9 19.2 6.1 21.9 5.8C22.8 5.7 23.5 6.3 23.5 7.2V20.3C23.5 21.1 22.9 21.7 22 21.8C19.3 22 16.4 22.6 14 24C11.6 22.6 8.7 22 6 21.8C5.1 21.7 4.5 21.1 4.5 20.3V7.2Z"
        fill="var(--craft-icon-paper, #fffaf0)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
        stroke-linejoin="round"
      />
      <path d="M14 8.4V23.5" stroke="var(--craft-icon-ink, #27221d)" stroke-width="1.9" />
      <path d="M7.5 11.2H11M7.5 14.8H11M17 11.2H20.5M17 14.8H20.5" stroke="var(--craft-icon-sky-deep, #4aa8e8)" stroke-width="1.8" stroke-linecap="round" />
    </template>

    <!-- Shield (Security / 2FA / Trust) -->
    <template v-else-if="name === 'shield'">
      <path
        d="M14 4.2L21.8 7.2V13.4C21.8 18.2 18.5 21.8 14 23.8C9.5 21.8 6.2 18.2 6.2 13.4V7.2L14 4.2Z"
        fill="var(--craft-icon-sky-soft, #e4f4ff)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
        stroke-linejoin="round"
      />
      <path
        d="M14 7L19 9V13.3C19 16.4 16.8 18.9 14 20.4C11.2 18.9 9 16.4 9 13.3V9L14 7Z"
        fill="var(--craft-icon-sky, #95d5fc)"
      />
      <path
        d="M11 13.8L13.1 15.9L17.2 11.6"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2.1"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
    </template>

    <!-- Receipt (Orders / Invoices / Billing) -->
    <template v-else-if="name === 'receipt'">
      <path
        d="M6.5 4.8H21.5V23.2L18.8 21.4L16.4 23.2L14 21.4L11.6 23.2L9.2 21.4L6.5 23.2V4.8Z"
        fill="var(--craft-icon-paper, #fffaf0)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
        stroke-linejoin="round"
      />
      <path
        d="M10 9.5H18M10 13.5H18M10 17.3H14.8"
        stroke="var(--craft-icon-sky-deep, #2f95e0)"
        stroke-width="2"
        stroke-linecap="round"
      />
      <circle cx="17.8" cy="17.3" r="1.4" fill="var(--craft-icon-peach, #ffad87)" />
    </template>

    <!-- Crown (Subscriptions / Membership Plans) -->
    <template v-else-if="name === 'crown'">
      <path
        d="M4.8 19.6L6.4 8.8L11 13.2L14 7L17 13.2L21.6 8.8L23.2 19.6H4.8Z"
        fill="var(--craft-icon-gold, #ffdc6b)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
        stroke-linejoin="round"
      />
      <rect
        x="5"
        y="19.6"
        width="18"
        height="3.4"
        rx="1.5"
        fill="var(--craft-icon-peach, #ffad87)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
      />
      <circle cx="14" cy="6.2" r="1.5" fill="var(--craft-icon-sky, #95d5fc)" stroke="var(--craft-icon-ink, #27221d)" stroke-width="1.6" />
    </template>

    <!-- Share (Affiliate / Invite) -->
    <template v-else-if="name === 'share'">
      <path
        d="M9.5 14L18.2 8.8M9.5 14L18.2 19.2"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2.2"
        stroke-linecap="round"
      />
      <circle cx="7.8" cy="14" r="3.4" fill="var(--craft-icon-sky, #95d5fc)" stroke="var(--craft-icon-ink, #27221d)" stroke-width="2" />
      <circle cx="19.5" cy="8" r="3.2" fill="var(--craft-icon-gold, #ffdc6b)" stroke="var(--craft-icon-ink, #27221d)" stroke-width="2" />
      <circle cx="19.5" cy="20" r="3.2" fill="var(--craft-icon-peach, #ffad87)" stroke="var(--craft-icon-ink, #27221d)" stroke-width="2" />
    </template>

    <!-- Radio (Available Channels / Live Monitor) -->
    <template v-else-if="name === 'radio'">
      <path
        d="M6.8 20.8A10.2 10.2 0 0 1 6.8 7.2M21.2 7.2A10.2 10.2 0 0 1 21.2 20.8"
        stroke="var(--craft-icon-sky-deep, #2f95e0)"
        stroke-width="2"
        stroke-linecap="round"
      />
      <path
        d="M9.7 18A6.1 6.1 0 0 1 9.7 10M18.3 10A6.1 6.1 0 0 1 18.3 18"
        stroke="var(--craft-icon-peach, #ffad87)"
        stroke-width="2"
        stroke-linecap="round"
      />
      <circle cx="14" cy="14" r="3" fill="var(--craft-icon-gold, #ffdc6b)" stroke="var(--craft-icon-ink, #27221d)" stroke-width="2" />
      <path d="M14 17V22.8" stroke="var(--craft-icon-ink, #27221d)" stroke-width="2" stroke-linecap="round" />
    </template>

    <!-- Plug (MCP Connections / Integrations) -->
    <template v-else-if="name === 'plug'">
      <path d="M10.5 5.2V9.5M17.5 5.2V9.5" stroke="var(--craft-icon-ink, #27221d)" stroke-width="2.2" stroke-linecap="round" />
      <path
        d="M7.5 9.5H20.5V13.8C20.5 17.4 17.6 19.8 14 19.8C10.4 19.8 7.5 17.4 7.5 13.8V9.5Z"
        fill="var(--craft-icon-sky, #95d5fc)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
        stroke-linejoin="round"
      />
      <path d="M14 19.8V23.5" stroke="var(--craft-icon-ink, #27221d)" stroke-width="2.2" stroke-linecap="round" />
      <circle cx="14" cy="14.2" r="1.7" fill="var(--craft-icon-paper, #fffaf0)" />
    </template>

    <!-- Palette (Image Workbench / Studio) -->
    <template v-else-if="name === 'palette'">
      <path
        d="M14 4.8C8.7 4.8 4.6 8.8 4.6 13.8C4.6 18.4 8.2 22.6 13.2 22.6C15.1 22.6 16.4 21.5 16.4 20C16.4 18.9 15.8 18.2 16.8 17.3C17.6 16.6 19.4 16.8 21 16.2C22.6 15.6 23.4 14 23.4 12.2C23.4 8 19.3 4.8 14 4.8Z"
        fill="var(--craft-icon-paper, #fffaf0)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
        stroke-linejoin="round"
      />
      <circle cx="9.6" cy="12" r="1.6" fill="var(--craft-icon-peach, #ffad87)" />
      <circle cx="13.8" cy="9" r="1.6" fill="var(--craft-icon-gold, #ffdc6b)" />
      <circle cx="18.2" cy="11.2" r="1.6" fill="var(--craft-icon-sky, #95d5fc)" />
      <circle cx="10.2" cy="16.4" r="1.5" fill="var(--craft-icon-sky-deep, #2f95e0)" />
    </template>

    <!-- Wallet (Online Recharge) -->
    <template v-else-if="name === 'wallet'">
      <rect
        x="4.5"
        y="7"
        width="19"
        height="14.5"
        rx="3.8"
        fill="var(--craft-icon-peach-soft, #ffe7da)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
      />
      <path d="M6.8 7L17.8 4.6C18.8 4.4 19.8 5.1 19.8 6.1V7" stroke="var(--craft-icon-ink, #27221d)" stroke-width="2" stroke-linejoin="round" />
      <rect
        x="15.5"
        y="11.6"
        width="8.5"
        height="5.4"
        rx="2"
        fill="var(--craft-icon-sky, #95d5fc)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="1.9"
      />
      <circle cx="18.5" cy="14.3" r="1.1" fill="var(--craft-icon-ink, #27221d)" />
    </template>

    <!-- Passkey (Passkey / Keycard) -->
    <template v-else-if="name === 'passkey'">
      <rect
        x="4.2"
        y="6.2"
        width="19.6"
        height="15.6"
        rx="3.4"
        fill="var(--craft-icon-paper, #fffaf0)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
      />
      <circle cx="10" cy="12" r="2.3" fill="var(--craft-icon-sky, #95d5fc)" stroke="var(--craft-icon-ink, #27221d)" stroke-width="1.8" />
      <path d="M6.8 18.2C7.2 16.3 8.4 15.3 10 15.3C11.6 15.3 12.8 16.3 13.2 18.2" stroke="var(--craft-icon-ink, #27221d)" stroke-width="1.8" stroke-linecap="round" />
      <path d="M15.4 11.5H20.2M15.4 15.2H18.8" stroke="var(--craft-icon-peach, #ffad87)" stroke-width="2" stroke-linecap="round" />
    </template>

    <!-- Inbox (Handcrafted Empty State Tray) -->
    <template v-else-if="name === 'inbox'">
      <path
        d="M5.2 14L7.4 6.8H20.6L22.8 14V21.2C22.8 22 22.1 22.6 21.3 22.6H6.7C5.9 22.6 5.2 22 5.2 21.2V14Z"
        fill="var(--craft-icon-paper, #fffaf0)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
        stroke-linejoin="round"
      />
      <path
        d="M5.2 14H10.3L11.8 16.6H16.2L17.7 14H22.8"
        fill="var(--craft-icon-sky-soft, #e4f4ff)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
        stroke-linejoin="round"
      />
      <path d="M10.5 10.2H17.5" stroke="var(--craft-icon-peach, #ffad87)" stroke-width="2" stroke-linecap="round" />
    </template>

    <!-- Server (Admin Accounts / Service Nodes) -->
    <template v-else-if="name === 'server'">
      <rect
        x="4.8"
        y="5.2"
        width="18.4"
        height="7.6"
        rx="2.6"
        fill="var(--craft-icon-sky-soft, #e4f4ff)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
      />
      <rect
        x="4.8"
        y="15.2"
        width="18.4"
        height="7.6"
        rx="2.6"
        fill="var(--craft-icon-paper, #fffaf0)"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="2"
      />
      <circle cx="8.8" cy="9" r="1.3" fill="var(--craft-icon-peach, #ffad87)" />
      <circle cx="8.8" cy="19" r="1.3" fill="var(--craft-icon-gold, #ffdc6b)" />
      <path d="M13 9H19.4M13 19H19.4" stroke="var(--craft-icon-ink, #27221d)" stroke-width="1.9" stroke-linecap="round" />
    </template>

    <!-- Users (Team / Active Users) -->
    <template v-else-if="name === 'users'">
      <circle cx="10.8" cy="9.6" r="3.3" fill="var(--craft-icon-sky-soft, #d7effe)" stroke="var(--craft-icon-ink, #27221d)" stroke-width="2" />
      <circle cx="17.8" cy="10.6" r="2.7" fill="var(--craft-icon-gold-light, #fff0a6)" stroke="var(--craft-icon-ink, #27221d)" stroke-width="1.9" />
      <path d="M4.8 21.6C5.1 17.8 7.7 15.6 10.8 15.6C13.9 15.6 16.5 17.8 16.8 21.6" fill="var(--craft-icon-peach-soft, #ffe5d6)" stroke="var(--craft-icon-ink, #27221d)" stroke-width="2" stroke-linecap="round" />
      <path d="M16.8 16.2C19.6 16.4 21.8 18.2 22.2 21.2" stroke="var(--craft-icon-ink, #27221d)" stroke-width="1.9" stroke-linecap="round" />
    </template>

    <!-- Provider: OpenAI -->
    <template v-else-if="name === 'openai'">
      <circle cx="14" cy="14" r="10.2" fill="var(--craft-icon-paper, #f6f8fa)" stroke="var(--craft-icon-ink, #27221d)" stroke-width="1.8" />
      <g stroke="var(--craft-icon-ink, #27221d)" stroke-width="1.55" stroke-linecap="round" stroke-linejoin="round">
        <path d="M14 7.5a3.6 3.6 0 0 1 3.1 1.8l2.5 1.4a3.6 3.6 0 0 1 .1 6.2l-2.6 1.5a3.6 3.6 0 0 1-6.2 0l-2.6-1.5a3.6 3.6 0 0 1 .1-6.2l2.5-1.4A3.6 3.6 0 0 1 14 7.5Z" fill="none" />
        <path d="M14 10.6v6.8M11 12.3l6 3.4M17 12.3l-6 3.4" />
      </g>
    </template>

    <!-- Provider: Anthropic / Claude -->
    <template v-else-if="name === 'anthropic'">
      <circle cx="14" cy="14" r="10.2" fill="var(--craft-icon-peach-tint, #fff0e6)" stroke="var(--craft-icon-ink, #27221d)" stroke-width="1.8" />
      <g stroke="#e06d44" stroke-width="2" stroke-linecap="round">
        <path d="M14 7.8V20.2M7.8 14H20.2M9.6 9.6L18.4 18.4M18.4 9.6L9.6 18.4" />
      </g>
    </template>

    <!-- Provider: Gemini -->
    <template v-else-if="name === 'gemini'">
      <circle cx="14" cy="14" r="10.2" fill="var(--craft-icon-sky-soft, #e5f4ff)" stroke="var(--craft-icon-ink, #27221d)" stroke-width="1.8" />
      <path
        d="M14 7.2C14.7 11.1 16.9 13.3 20.8 14C16.9 14.7 14.7 16.9 14 20.8C13.3 16.9 11.1 14.7 7.2 14C11.1 13.3 13.3 11.1 14 7.2Z"
        fill="#4499eb"
        stroke="var(--craft-icon-ink, #27221d)"
        stroke-width="1.5"
        stroke-linejoin="round"
      />
    </template>

    <!-- Provider: Grok / Antigravity / Generic -->
    <template v-else>
      <circle cx="14" cy="14" r="10.2" fill="var(--craft-icon-paper, #fffaf0)" stroke="var(--craft-icon-ink, #27221d)" stroke-width="1.8" />
      <path
        d="M9.5 14L12.5 11L15.5 14L18.5 11M9.5 17.2H18.5"
        stroke="var(--craft-icon-sky-deep, #2f95e0)"
        stroke-width="2"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
    </template>
  </svg>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    name: string
    size?: 'xs' | 'sm' | 'md' | 'lg'
  }>(),
  {
    size: 'md',
  }
)
</script>

<style scoped>
.craft-icon {
  display: inline-block;
  flex: none;
  overflow: visible;
  transform-origin: center;
}

.craft-icon--xs {
  width: 16px;
  height: 16px;
  transform: scale(1.22);
}

.craft-icon--sm {
  width: 20px;
  height: 20px;
  transform: scale(1.2);
}

.craft-icon--md {
  width: 24px;
  height: 24px;
  transform: scale(1.28);
}

.craft-icon--lg {
  width: 30px;
  height: 30px;
  transform: scale(1.26);
}
</style>
