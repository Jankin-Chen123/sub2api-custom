<template>
  <span
    class="jbs-hit"
    :class="[sizeClass, `jbs-size--${size}`, `jbs-icon--${resolvedName}`]"
    aria-hidden="true"
  >
    <svg
      class="jbs-svg h-full w-full overflow-visible"
      viewBox="0 0 96 96"
      fill="none"
    >
      <defs>
        <filter
          :id="filterId"
          filterUnits="userSpaceOnUse"
          x="-16"
          y="-16"
          width="128"
          height="128"
          color-interpolation-filters="sRGB"
        >
          <feMorphology in="SourceAlpha" operator="dilate" radius="4.2" result="dilated" />
          <feGaussianBlur in="dilated" stdDeviation="0.55" result="smooth" />
          <feComponentTransfer in="smooth" result="solid">
            <feFuncA type="table" tableValues="0 0 0.45 1 1" />
          </feComponentTransfer>
          <feFlood flood-color="#ffffff" result="white" />
          <feComposite in="white" in2="solid" operator="in" result="stickerEdge" />
          <feDropShadow
            in="stickerEdge"
            dx="0"
            dy="4"
            stdDeviation="3.2"
            flood-color="#20405c"
            flood-opacity="0.24"
            result="shadowed"
          />
          <feMerge>
            <feMergeNode in="shadowed" />
            <feMergeNode in="SourceGraphic" />
          </feMerge>
        </filter>

        <linearGradient :id="grad.sky" x1="18" y1="16" x2="78" y2="80" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stop-color="#aee2ff" />
          <stop offset="100%" stop-color="#7bc7fb" />
        </linearGradient>
        <linearGradient :id="grad.mint" x1="20" y1="16" x2="76" y2="78" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stop-color="#c2f5e4" />
          <stop offset="100%" stop-color="#8be3c6" />
        </linearGradient>
        <linearGradient :id="grad.gold" x1="18" y1="16" x2="78" y2="80" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stop-color="#ffea96" />
          <stop offset="100%" stop-color="#ffc74d" />
        </linearGradient>
        <linearGradient :id="grad.peach" x1="18" y1="16" x2="78" y2="80" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stop-color="#ffc7a8" />
          <stop offset="100%" stop-color="#ff956b" />
        </linearGradient>
        <linearGradient :id="grad.cream" x1="18" y1="16" x2="78" y2="80" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stop-color="#fffdf6" />
          <stop offset="100%" stop-color="#f6e8cf" />
        </linearGradient>
        <linearGradient :id="grad.lavender" x1="18" y1="16" x2="78" y2="80" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stop-color="#e1d2ff" />
          <stop offset="100%" stop-color="#bfa5ff" />
        </linearGradient>
        <linearGradient :id="grad.coral" x1="18" y1="16" x2="78" y2="80" gradientUnits="userSpaceOnUse">
          <stop offset="0%" stop-color="#ff9c87" />
          <stop offset="100%" stop-color="#f06852" />
        </linearGradient>
      </defs>

      <g :filter="`url(#${filterId})`" class="jbs-art">
        <!-- 1. Wallet (仪表盘) -->
        <template v-if="resolvedName === 'wallet'">
          <g class="jbs-p jbs-w-card">
            <rect x="24" y="23" width="44" height="26" rx="6" :fill="`url(#${grad.mint})`" stroke="#1c1713" stroke-width="4" stroke-linejoin="round" />
            <path d="M31 31H46" stroke="#1c1713" stroke-width="3" stroke-linecap="round" />
          </g>
          <g class="jbs-p jbs-w-coin">
            <circle cx="58" cy="36" r="10.5" :fill="`url(#${grad.gold})`" stroke="#1c1713" stroke-width="3.6" />
            <path d="M58 31.5V40.5M55.2 34.5C55.2 33 56.4 32.3 58 32.3C59.5 32.3 60.5 33 60.5 34.1C60.5 36.2 55.2 35.6 55.2 37.8C55.2 39 56.3 39.7 58 39.7C59.6 39.7 60.6 39 60.6 37.8" stroke="#1c1713" stroke-width="2.2" stroke-linecap="round" />
          </g>
          <g class="jbs-p jbs-w-body">
            <rect x="18" y="34" width="58" height="42" rx="12" :fill="`url(#${grad.sky})`" stroke="#1c1713" stroke-width="4.2" stroke-linejoin="round" />
            <path d="M25 43C29 40 37 40 43 40" stroke="#ffffff" stroke-opacity="0.78" stroke-width="2.8" stroke-linecap="round" />
            <path d="M25 68H63" stroke="#1c1713" stroke-opacity="0.16" stroke-width="2.5" stroke-dasharray="3 4.5" stroke-linecap="round" />
          </g>
          <g class="jbs-p jbs-w-clasp">
            <rect x="55" y="47" width="24" height="17" rx="8.5" :fill="`url(#${grad.gold})`" stroke="#1c1713" stroke-width="4" stroke-linejoin="round" />
            <circle cx="64.5" cy="55.5" r="3" fill="#1c1713" />
          </g>
          <g class="jbs-p jbs-w-sparks">
            <path d="M74 20L76 24.5L80.5 26.5L76 28.5L74 33L72 28.5L67.5 26.5L72 24.5Z" :fill="`url(#${grad.gold})`" stroke="#1c1713" stroke-width="2.4" stroke-linejoin="round" />
          </g>
        </template>

        <!-- 2. Key (API 密钥) -->
        <template v-else-if="resolvedName === 'key'">
          <g class="jbs-p jbs-k-tag">
            <path d="M33 56L22 67C20 69 20 72.5 22 74.5L26.5 79C28.5 81 32 81 34 79L45 68C46.5 66.5 46.5 64 45 62.5L39.5 56C38 54.5 34.5 54.5 33 56Z" :fill="`url(#${grad.mint})`" stroke="#1c1713" stroke-width="3.8" stroke-linejoin="round" />
            <circle cx="33" cy="67" r="2.6" fill="#1c1713" />
          </g>
          <g class="jbs-p jbs-k-main">
            <path d="M46 43H77C80.2 43 82.5 45.2 82.5 48.2C82.5 51.2 80.2 53.5 77 53.5H74V60.5C74 62.2 72.6 63.5 71 63.5H67.5C65.8 63.5 64.5 62.2 64.5 60.5V53.5H59V58.5C59 60.2 57.6 61.5 56 61.5H53.5C51.8 61.5 50.5 60.2 50.5 58.5V53.5H46" :fill="`url(#${grad.gold})`" stroke="#1c1713" stroke-width="4.2" stroke-linejoin="round" />
            <circle cx="34" cy="48" r="15.5" :fill="`url(#${grad.gold})`" stroke="#1c1713" stroke-width="4.2" />
            <circle cx="34" cy="48" r="6.5" :fill="`url(#${grad.sky})`" stroke="#1c1713" stroke-width="3.6" />
            <path d="M25 38.5C27.5 36.2 31 35 34.5 35" stroke="#ffffff" stroke-opacity="0.78" stroke-width="2.6" stroke-linecap="round" />
          </g>
          <g class="jbs-p jbs-k-sparks">
            <path d="M75 26L77 30.5L81.5 32.5L77 34.5L75 39L73 34.5L68.5 32.5L73 30.5Z" :fill="`url(#${grad.mint})`" stroke="#1c1713" stroke-width="2.5" stroke-linejoin="round" />
          </g>
        </template>

        <!-- 3. Palette (生图工作台) -->
        <template v-else-if="resolvedName === 'palette'">
          <g class="jbs-p jbs-p-board">
            <path d="M47 22C30.5 22 17 34.5 17 50.5C17 65 28.5 76 44 76C48.5 76 51.8 73 51.8 69.2C51.8 67.2 51 65.5 50.2 64.1C49.5 62.9 50.4 61.2 52 61.2H57.5C68.5 61.2 76.5 54 76.5 44.5C76.5 31.8 63.5 22 47 22ZM31 61C27.5 61 25 58.3 25 55C25 51.7 27.5 49 31 49C34.5 49 37 51.7 37 55C37 58.3 34.5 61 31 61Z" :fill="`url(#${grad.cream})`" stroke="#1c1713" stroke-width="4.2" stroke-linejoin="round" />
            <path d="M26 36C29 31.5 35 28.5 41 27.5" stroke="#ffffff" stroke-opacity="0.82" stroke-width="2.6" stroke-linecap="round" />
          </g>
          <g class="jbs-p jbs-p-dabs">
            <circle cx="34" cy="36" r="5" :fill="`url(#${grad.coral})`" stroke="#1c1713" stroke-width="3" />
            <circle cx="50" cy="32" r="5" :fill="`url(#${grad.gold})`" stroke="#1c1713" stroke-width="3" />
            <circle cx="63" cy="43" r="5" :fill="`url(#${grad.sky})`" stroke="#1c1713" stroke-width="3" />
            <circle cx="49" cy="49" r="4.6" :fill="`url(#${grad.mint})`" stroke="#1c1713" stroke-width="3" />
          </g>
          <g class="jbs-p jbs-p-brush">
            <path d="M51 54L74 23C75.8 20.8 79 20.6 80.8 22.4C82.6 24.2 82.4 27.4 80.2 29.2L56 59Z" :fill="`url(#${grad.sky})`" stroke="#1c1713" stroke-width="3.8" stroke-linejoin="round" />
            <path d="M43 66C41.2 61.5 43.2 55.8 48.2 53.6C51.6 52.1 55.8 53.8 57.2 57.5C58.8 61.8 55 67 48.5 67.5C45.8 67.7 43.8 67.2 43 66Z" :fill="`url(#${grad.peach})`" stroke="#1c1713" stroke-width="3.8" stroke-linejoin="round" />
          </g>
          <g class="jbs-p jbs-p-sparks">
            <path d="M24 22L26 26L30 28L26 30L24 34L22 30L18 28L22 26Z" :fill="`url(#${grad.gold})`" stroke="#1c1713" stroke-width="2.4" stroke-linejoin="round" />
          </g>
        </template>

        <!-- 4. Batch (批量生图) -->
        <template v-else-if="resolvedName === 'batch'">
          <g class="jbs-p jbs-b-back">
            <rect x="24" y="21" width="44" height="52" rx="8" :fill="`url(#${grad.coral})`" stroke="#1c1713" stroke-width="3.8" />
          </g>
          <g class="jbs-p jbs-b-mid">
            <rect x="28" y="23" width="44" height="52" rx="8" :fill="`url(#${grad.mint})`" stroke="#1c1713" stroke-width="3.8" />
          </g>
          <g class="jbs-p jbs-b-front">
            <rect x="26" y="24" width="44" height="52" rx="8" :fill="`url(#${grad.cream})`" stroke="#1c1713" stroke-width="4.2" />
            <rect x="32" y="30" width="32" height="28" rx="4.5" :fill="`url(#${grad.sky})`" stroke="#1c1713" stroke-width="3.2" />
            <circle cx="42" cy="39" r="3.6" :fill="`url(#${grad.gold})`" stroke="#1c1713" stroke-width="2" />
            <path d="M33 56L42 46L50 53L56 47L63 56H33Z" :fill="`url(#${grad.mint})`" stroke="#1c1713" stroke-width="2.6" stroke-linejoin="round" />
            <path d="M36 66H56" stroke="#1c1713" stroke-width="3" stroke-linecap="round" />
          </g>
          <g class="jbs-p jbs-b-spark">
            <path d="M75 18L77 22.5L81.5 24.5L77 26.5L75 31L73 26.5L68.5 24.5L73 22.5Z" :fill="`url(#${grad.gold})`" stroke="#1c1713" stroke-width="2.4" stroke-linejoin="round" />
          </g>
        </template>

        <!-- 5. Bars (使用记录) -->
        <template v-else-if="resolvedName === 'bars'">
          <g class="jbs-p jbs-ba-card">
            <rect x="19" y="22" width="58" height="54" rx="13" :fill="`url(#${grad.cream})`" stroke="#1c1713" stroke-width="4.2" />
            <path d="M27 30H45" stroke="#ffffff" stroke-opacity="0.85" stroke-width="2.6" stroke-linecap="round" />
            <path d="M26 67H70" stroke="#1c1713" stroke-width="3.4" stroke-linecap="round" />
          </g>
          <g class="jbs-p jbs-ba-b1">
            <rect x="29" y="45" width="10" height="22" rx="4.5" :fill="`url(#${grad.sky})`" stroke="#1c1713" stroke-width="3.4" />
          </g>
          <g class="jbs-p jbs-ba-b2">
            <rect x="43" y="34" width="10" height="33" rx="4.5" :fill="`url(#${grad.peach})`" stroke="#1c1713" stroke-width="3.4" />
          </g>
          <g class="jbs-p jbs-ba-b3">
            <rect x="57" y="40" width="10" height="27" rx="4.5" :fill="`url(#${grad.gold})`" stroke="#1c1713" stroke-width="3.4" />
          </g>
          <g class="jbs-p jbs-ba-star">
            <path d="M68 18L70 22.5L74.5 24.5L70 26.5L68 31L66 26.5L61.5 24.5L66 22.5Z" :fill="`url(#${grad.gold})`" stroke="#1c1713" stroke-width="2.4" stroke-linejoin="round" />
          </g>
        </template>

        <!-- 6. Channels (可用渠道) -->
        <template v-else-if="resolvedName === 'channels'">
          <g class="jbs-p jbs-ch-post">
            <rect x="43" y="22" width="10" height="56" rx="5" :fill="`url(#${grad.cream})`" stroke="#1c1713" stroke-width="4" />
            <circle cx="48" cy="22" r="6" :fill="`url(#${grad.gold})`" stroke="#1c1713" stroke-width="3.6" />
          </g>
          <g class="jbs-p jbs-ch-left">
            <path d="M56 31H30L20 39L30 47H56C58.2 47 60 45.2 60 43V35C60 32.8 58.2 31 56 31Z" :fill="`url(#${grad.sky})`" stroke="#1c1713" stroke-width="4" stroke-linejoin="round" />
            <path d="M35 37H51" stroke="#1c1713" stroke-width="2.8" stroke-linecap="round" />
          </g>
          <g class="jbs-p jbs-ch-right">
            <path d="M40 50H66L76 58L66 66H40C37.8 66 36 64.2 36 62V54C36 51.8 37.8 50 40 50Z" :fill="`url(#${grad.peach})`" stroke="#1c1713" stroke-width="4" stroke-linejoin="round" />
            <path d="M45 56H61" stroke="#1c1713" stroke-width="2.8" stroke-linecap="round" />
          </g>
          <g class="jbs-p jbs-ch-star">
            <path d="M68 18L70 22.2L74.2 24.2L70 26.2L68 30.4L66 26.2L61.8 24.2L66 22.2Z" :fill="`url(#${grad.mint})`" stroke="#1c1713" stroke-width="2.4" stroke-linejoin="round" />
          </g>
        </template>

        <!-- 7. Radio (渠道状态 V1 & V2) -->
        <template v-else-if="resolvedName === 'radio'">
          <g class="jbs-p jbs-ra-outer">
            <path d="M22 21C14.5 29 14.5 43 22 51" stroke="#7bc7fb" stroke-width="4.5" stroke-linecap="round" />
            <path d="M74 21C81.5 29 81.5 43 74 51" stroke="#7bc7fb" stroke-width="4.5" stroke-linecap="round" />
            <path d="M22 21C14.5 29 14.5 43 22 51" stroke="#1c1713" stroke-width="2" stroke-linecap="round" opacity="0.55" />
            <path d="M74 21C81.5 29 81.5 43 74 51" stroke="#1c1713" stroke-width="2" stroke-linecap="round" opacity="0.55" />
          </g>
          <g class="jbs-p jbs-ra-inner">
            <path d="M30 27C25.5 32.5 25.5 40.5 30 46" stroke="#8be3c6" stroke-width="4.2" stroke-linecap="round" />
            <path d="M66 27C70.5 32.5 70.5 40.5 66 46" stroke="#8be3c6" stroke-width="4.2" stroke-linecap="round" />
          </g>
          <g class="jbs-p jbs-ra-tower">
            <path d="M34 75L44 43H52L62 75H34Z" :fill="`url(#${grad.cream})`" stroke="#1c1713" stroke-width="4.2" stroke-linejoin="round" />
            <path d="M39 62H57" stroke="#1c1713" stroke-width="3.4" stroke-linecap="round" />
            <rect x="29" y="71" width="38" height="8" rx="4" :fill="`url(#${grad.sky})`" stroke="#1c1713" stroke-width="3.8" />
          </g>
          <g class="jbs-p jbs-ra-bulb">
            <circle cx="48" cy="36" r="10.5" :fill="`url(#${grad.gold})`" stroke="#1c1713" stroke-width="4.2" />
            <path d="M43 32C44.5 30.2 46.8 29.5 49 29.8" stroke="#ffffff" stroke-opacity="0.82" stroke-width="2.4" stroke-linecap="round" />
          </g>
        </template>

        <!-- 8. Crown (我的订阅) -->
        <template v-else-if="resolvedName === 'crown'">
          <g class="jbs-p jbs-cr-body">
            <path d="M21 63L17 34L35 45L48 25L61 45L79 34L75 63C74.5 67.5 71 70.5 66.5 70.5H29.5C25 70.5 21.5 67.5 21 63Z" :fill="`url(#${grad.gold})`" stroke="#1c1713" stroke-width="4.2" stroke-linejoin="round" />
            <rect x="21" y="59" width="54" height="12" rx="6" :fill="`url(#${grad.peach})`" stroke="#1c1713" stroke-width="4" />
            <circle cx="17" cy="31" r="4.5" :fill="`url(#${grad.sky})`" stroke="#1c1713" stroke-width="3.2" />
            <circle cx="48" cy="23" r="5" :fill="`url(#${grad.coral})`" stroke="#1c1713" stroke-width="3.2" />
            <circle cx="79" cy="31" r="4.5" :fill="`url(#${grad.mint})`" stroke="#1c1713" stroke-width="3.2" />
          </g>
          <g class="jbs-p jbs-cr-gem">
            <circle cx="48" cy="65" r="4.5" :fill="`url(#${grad.sky})`" stroke="#1c1713" stroke-width="2.8" />
          </g>
          <g class="jbs-p jbs-cr-sparks">
            <path d="M31 19L32.5 22.5L36 24L32.5 25.5L31 29L29.5 25.5L26 24L29.5 22.5Z" :fill="`url(#${grad.gold})`" stroke="#1c1713" stroke-width="2.2" stroke-linejoin="round" />
            <path d="M67 19L68.5 22.5L72 24L68.5 25.5L67 29L65.5 25.5L62 24L65.5 22.5Z" :fill="`url(#${grad.gold})`" stroke="#1c1713" stroke-width="2.2" stroke-linejoin="round" />
          </g>
        </template>

        <!-- 9. Recharge (在线充值) -->
        <template v-else-if="resolvedName === 'recharge'">
          <g class="jbs-p jbs-rc-coin">
            <circle cx="48" cy="34" r="14" :fill="`url(#${grad.gold})`" stroke="#1c1713" stroke-width="4.2" />
            <path d="M48 27.5V40.5M44 31.8C44 29.8 45.8 28.8 48 28.8C50.2 28.8 51.6 29.8 51.6 31.3C51.6 34 44 33.4 44 36.4C44 38 45.6 39 48 39C50.2 39 51.8 38 51.8 36.4" stroke="#1c1713" stroke-width="2.8" stroke-linecap="round" />
          </g>
          <g class="jbs-p jbs-rc-card">
            <rect x="18" y="39" width="60" height="38" rx="11" :fill="`url(#${grad.sky})`" stroke="#1c1713" stroke-width="4.2" />
            <rect x="18" y="48" width="60" height="9" :fill="`url(#${grad.mint})`" stroke="#1c1713" stroke-width="3.6" />
            <rect x="26" y="64" width="16" height="6" rx="3" :fill="`url(#${grad.cream})`" stroke="#1c1713" stroke-width="2.4" />
            <path d="M25 43.5H39" stroke="#ffffff" stroke-opacity="0.75" stroke-width="2.6" stroke-linecap="round" />
          </g>
          <g class="jbs-p jbs-rc-badge">
            <circle cx="72" cy="27" r="9.5" :fill="`url(#${grad.peach})`" stroke="#1c1713" stroke-width="3.2" />
            <path d="M72 22.8V31.2M67.8 27H76.2" stroke="#1c1713" stroke-width="2.8" stroke-linecap="round" />
          </g>
        </template>

        <!-- 10. Receipt (我的订单) -->
        <template v-else-if="resolvedName === 'receipt'">
          <g class="jbs-p jbs-re-paper">
            <path d="M24 22C24 19.2 26.2 17 29 17H65C67.8 17 70 19.2 70 22V73L64.2 68.5L58.5 73L52.8 68.5L47 73L41.2 68.5L35.5 73L29.8 68.5L24 73V22Z" :fill="`url(#${grad.cream})`" stroke="#1c1713" stroke-width="4.2" stroke-linejoin="round" />
            <rect x="31" y="25" width="32" height="9" rx="4.5" :fill="`url(#${grad.sky})`" stroke="#1c1713" stroke-width="3" />
            <path d="M32 43H60M32 52H52" stroke="#1c1713" stroke-width="3.6" stroke-linecap="round" />
            <path d="M32 60H44" stroke="#e5894b" stroke-width="3.6" stroke-linecap="round" />
          </g>
          <g class="jbs-p jbs-re-stamp">
            <circle cx="65" cy="61" r="13.5" :fill="`url(#${grad.coral})`" stroke="#1c1713" stroke-width="3.8" />
            <circle cx="65" cy="61" r="9" fill="none" stroke="#fff7f0" stroke-width="1.8" stroke-dasharray="2.5 2.5" />
            <path d="M59.5 61L63.5 65L71 57.5" stroke="#fffdf8" stroke-width="3.4" stroke-linecap="round" stroke-linejoin="round" />
          </g>
        </template>

        <!-- 11. Gift (兑换/签到) -->
        <template v-else-if="resolvedName === 'gift'">
          <g class="jbs-p jbs-gi-pop">
            <path d="M48 16L50.8 22.8L57.5 25.5L50.8 28.2L48 35L45.2 28.2L38.5 25.5L45.2 22.8Z" :fill="`url(#${grad.gold})`" stroke="#1c1713" stroke-width="2.8" stroke-linejoin="round" />
            <circle cx="32" cy="24" r="3.6" :fill="`url(#${grad.mint})`" stroke="#1c1713" stroke-width="2.4" />
            <circle cx="65" cy="24" r="3.6" :fill="`url(#${grad.coral})`" stroke="#1c1713" stroke-width="2.4" />
          </g>
          <g class="jbs-p jbs-gi-box">
            <rect x="23" y="45" width="50" height="33" rx="9" :fill="`url(#${grad.peach})`" stroke="#1c1713" stroke-width="4.2" />
            <rect x="42" y="45" width="12" height="33" :fill="`url(#${grad.sky})`" stroke="#1c1713" stroke-width="3.6" />
          </g>
          <g class="jbs-p jbs-gi-lid">
            <path d="M48 35C44 26 32 25 32 32C32 35.5 39 36.5 48 35Z" :fill="`url(#${grad.sky})`" stroke="#1c1713" stroke-width="3.6" stroke-linejoin="round" />
            <path d="M48 35C52 26 64 25 64 32C64 35.5 57 36.5 48 35Z" :fill="`url(#${grad.sky})`" stroke="#1c1713" stroke-width="3.6" stroke-linejoin="round" />
            <rect x="19" y="36" width="58" height="12" rx="6" :fill="`url(#${grad.cream})`" stroke="#1c1713" stroke-width="4.2" />
            <rect x="42" y="36" width="12" height="12" :fill="`url(#${grad.sky})`" stroke="#1c1713" stroke-width="3.6" />
          </g>
        </template>

        <!-- 12. Share (邀请返利) -->
        <template v-else-if="resolvedName === 'share'">
          <g class="jbs-p jbs-sh-env">
            <rect x="18" y="38" width="60" height="40" rx="10" fill="#6ebcf5" stroke="#1c1713" stroke-width="4.2" />
          </g>
          <g class="jbs-p jbs-sh-flap">
            <path d="M19 40L48 60L77 40Z" :fill="`url(#${grad.sky})`" stroke="#1c1713" stroke-width="4" stroke-linejoin="round" />
            <path d="M48 62.5C48 62.5 40 57 40 52.5C40 49.8 42.2 48 44.5 48C46.2 48 47.4 49 48 50.2C48.6 49 49.8 48 51.5 48C53.8 48 56 49.8 56 52.5C56 57 48 62.5 48 62.5Z" :fill="`url(#${grad.coral})`" stroke="#1c1713" stroke-width="3" stroke-linejoin="round" />
          </g>
          <g class="jbs-p jbs-sh-letter">
            <rect x="27" y="34" width="42" height="32" rx="7" :fill="`url(#${grad.cream})`" stroke="#1c1713" stroke-width="3.8" />
            <circle cx="48" cy="44" r="5.5" :fill="`url(#${grad.gold})`" stroke="#1c1713" stroke-width="2.6" />
            <path d="M36 55H60" stroke="#1c1713" stroke-width="2.8" stroke-linecap="round" />
          </g>
          <g class="jbs-p jbs-sh-env">
            <path d="M18 40L48 59L78 40V68C78 73.5 73.5 78 68 78H28C22.5 78 18 73.5 18 68V40Z" :fill="`url(#${grad.sky})`" stroke="#1c1713" stroke-width="4.2" stroke-linejoin="round" />
            <path d="M21 74L41 56M75 74L55 56" stroke="#1c1713" stroke-width="3" stroke-linecap="round" />
            <path d="M22 45L36 54" stroke="#ffffff" stroke-opacity="0.72" stroke-width="2.4" stroke-linecap="round" />
          </g>
        </template>

        <!-- 13. Plug (MCP 接入) -->
        <template v-else-if="resolvedName === 'plug'">
          <g class="jbs-p jbs-pl-socket">
            <rect x="52" y="31" width="27" height="34" rx="10" :fill="`url(#${grad.cream})`" stroke="#1c1713" stroke-width="4.2" />
            <rect x="58" y="38" width="14" height="20" rx="6" :fill="`url(#${grad.peach})`" stroke="#1c1713" stroke-width="3" />
            <path d="M62 44H68M62 52H68" stroke="#1c1713" stroke-width="3" stroke-linecap="round" />
          </g>
          <g class="jbs-p jbs-pl-head">
            <path d="M14 55C18 55 20 48 25 48" stroke="#1c1713" stroke-width="4.5" stroke-linecap="round" />
            <path d="M43 43H51M43 53H51" stroke="#ffea96" stroke-width="5.5" stroke-linecap="round" />
            <path d="M43 43H51M43 53H51" stroke="#1c1713" stroke-width="3.6" stroke-linecap="round" />
            <path d="M24 35H36C41.5 35 45 39 45 48C45 57 41.5 61 36 61H24C21.8 61 20 59.2 20 57V39C20 36.8 21.8 35 24 35Z" :fill="`url(#${grad.sky})`" stroke="#1c1713" stroke-width="4.2" stroke-linejoin="round" />
            <path d="M26 41H34" stroke="#ffffff" stroke-opacity="0.78" stroke-width="2.4" stroke-linecap="round" />
          </g>
          <g class="jbs-p jbs-pl-spark">
            <path d="M48 16L50 20.5L54.5 22.5L50 24.5L48 29L46 24.5L41.5 22.5L46 20.5Z" :fill="`url(#${grad.gold})`" stroke="#1c1713" stroke-width="2.4" stroke-linejoin="round" />
          </g>
        </template>

        <!-- 14. Admin (管理控制台) -->
        <template v-else-if="resolvedName === 'admin'">
          <g class="jbs-p jbs-ad-gear">
            <circle cx="70" cy="27" r="11" :fill="`url(#${grad.gold})`" stroke="#1c1713" stroke-width="3.6" stroke-dasharray="5 3.5" />
            <circle cx="70" cy="27" r="7" :fill="`url(#${grad.gold})`" stroke="#1c1713" stroke-width="3.2" />
            <circle cx="70" cy="27" r="2.6" fill="#1c1713" />
          </g>
          <g class="jbs-p jbs-ad-dial">
            <circle cx="46" cy="54" r="25" :fill="`url(#${grad.cream})`" stroke="#1c1713" stroke-width="4.2" />
            <path d="M28 56A18 18 0 0 1 40 37" stroke="#8be3c6" stroke-width="4.8" stroke-linecap="round" />
            <path d="M43 36.5A18 18 0 0 1 56 39" stroke="#ffc74d" stroke-width="4.8" stroke-linecap="round" />
            <path d="M59 41.5A18 18 0 0 1 64 56" stroke="#ff9c87" stroke-width="4.8" stroke-linecap="round" />
          </g>
          <g class="jbs-p jbs-ad-needle">
            <path d="M46 56L46 37" stroke="#f06852" stroke-width="4.5" stroke-linecap="round" />
            <circle cx="46" cy="56" r="5" :fill="`url(#${grad.sky})`" stroke="#1c1713" stroke-width="3.2" />
          </g>
          <g class="jbs-p jbs-ad-spark">
            <path d="M22 22L23.8 25.8L27.5 27.5L23.8 29.2L22 33L20.2 29.2L16.5 27.5L20.2 25.8Z" :fill="`url(#${grad.sky})`" stroke="#1c1713" stroke-width="2.2" stroke-linejoin="round" />
          </g>
        </template>

        <!-- 15. Contact (联系我们) -->
        <template v-else-if="resolvedName === 'contact'">
          <g class="jbs-p jbs-co-back">
            <path d="M40 27C40 22.6 43.6 19 48 19H72C76.4 19 80 22.6 80 27V44C80 48.4 76.4 52 72 52H67L72 60L59 52H48C43.6 52 40 48.4 40 44V27Z" :fill="`url(#${grad.peach})`" stroke="#1c1713" stroke-width="4" stroke-linejoin="round" />
            <circle cx="52" cy="35.5" r="2.6" fill="#1c1713" />
            <circle cx="60" cy="35.5" r="2.6" fill="#1c1713" />
            <circle cx="68" cy="35.5" r="2.6" fill="#1c1713" />
          </g>
          <g class="jbs-p jbs-co-front">
            <path d="M16 40C16 35 20 31 25 31H54C59 31 63 35 63 40V58C63 63 59 67 54 67H35L24 76L27 67H25C20 67 16 63 16 58V40Z" :fill="`url(#${grad.sky})`" stroke="#1c1713" stroke-width="4.2" stroke-linejoin="round" />
            <path d="M23 38.5H37" stroke="#ffffff" stroke-opacity="0.78" stroke-width="2.6" stroke-linecap="round" />
            <path d="M39.5 57.5C39.5 57.5 31 51.6 31 46.8C31 44 33.2 42 35.7 42C37.5 42 38.8 43.1 39.5 44.5C40.2 43.1 41.5 42 43.3 42C45.8 42 48 44 48 46.8C48 51.6 39.5 57.5 39.5 57.5Z" :fill="`url(#${grad.coral})`" stroke="#1c1713" stroke-width="3" stroke-linejoin="round" />
          </g>
          <g class="jbs-p jbs-co-spark">
            <path d="M22 17L24 21.5L28.5 23.5L24 25.5L22 30L20 25.5L15.5 23.5L20 21.5Z" :fill="`url(#${grad.gold})`" stroke="#1c1713" stroke-width="2.4" stroke-linejoin="round" />
          </g>
        </template>

        <!-- 16. Sparkles (模型广场 / 默认兜底) -->
        <template v-else>
          <g class="jbs-p jbs-sp-main">
            <path d="M48 20L54 39L73 45L54 51L48 70L42 51L23 45L42 39Z" :fill="`url(#${grad.gold})`" stroke="#1c1713" stroke-width="4.2" stroke-linejoin="round" />
            <path d="M48 29L50 35" stroke="#ffffff" stroke-opacity="0.82" stroke-width="2.4" stroke-linecap="round" />
          </g>
          <g class="jbs-p jbs-sp-a">
            <path d="M26 20L28.5 25.5L34 28L28.5 30.5L26 36L23.5 30.5L18 28L23.5 25.5Z" :fill="`url(#${grad.sky})`" stroke="#1c1713" stroke-width="3" stroke-linejoin="round" />
          </g>
          <g class="jbs-p jbs-sp-b">
            <path d="M69 57L71.5 62.5L77 65L71.5 67.5L69 73L66.5 67.5L61 65L66.5 62.5Z" :fill="`url(#${grad.coral})`" stroke="#1c1713" stroke-width="3" stroke-linejoin="round" />
          </g>
        </template>
      </g>
    </svg>
  </span>
</template>

<script lang="ts">
let uidSeed = 0

const STICKER_NAMES = new Set([
  'wallet',
  'key',
  'palette',
  'batch',
  'bars',
  'channels',
  'radio',
  'crown',
  'recharge',
  'receipt',
  'gift',
  'share',
  'plug',
  'admin',
  'contact',
  'sparkles',
])
</script>

<script setup lang="ts">
import { computed } from 'vue'

type StickerSize = 'sm' | 'md' | 'lg'

const props = withDefaults(
  defineProps<{
    name: string
    size?: StickerSize
  }>(),
  {
    size: 'md',
  }
)

const instanceUid = `jbs-${++uidSeed}`
const filterId = `${instanceUid}-diecut`
const grad = {
  sky: `${instanceUid}-sky`,
  mint: `${instanceUid}-mint`,
  gold: `${instanceUid}-gold`,
  peach: `${instanceUid}-peach`,
  cream: `${instanceUid}-cream`,
  lavender: `${instanceUid}-lavender`,
  coral: `${instanceUid}-coral`,
}

const resolvedName = computed(() =>
  STICKER_NAMES.has(props.name) ? props.name : 'sparkles'
)

const sizeClass = computed(() => {
  if (props.size === 'sm') return 'w-[52px] h-[52px] sm:w-[58px] sm:h-[58px]'
  if (props.size === 'lg') return 'w-[68px] h-[68px] sm:w-[74px] sm:h-[74px]'
  return 'w-[60px] h-[60px] sm:w-[66px] sm:h-[66px]'
})
</script>

<style scoped>
.jbs-hit {
  position: relative;
  display: inline-flex;
  flex-shrink: 0;
  align-items: center;
  justify-content: center;
  user-select: none;
  cursor: default;
}

.jbs-svg {
  overflow: visible;
  transform-origin: center;
}

.jbs-size--sm .jbs-svg {
  transform: scale(1.24);
}

.jbs-size--md .jbs-svg {
  transform: scale(1.28);
}

.jbs-size--lg .jbs-svg {
  transform: scale(1.34);
}

.jbs-p {
  transform-box: view-box;
  transition-property: transform, opacity;
  transition-duration: 220ms, 180ms;
  transition-timing-function: cubic-bezier(0.23, 1, 0.32, 1), cubic-bezier(0.23, 1, 0.32, 1);
  transition-delay: 0ms, 0ms;
}

/* ==================== Resting Poses ==================== */
/* 1. Wallet */
.jbs-w-body {
  transform-origin: 48px 56px;
  transform: rotate(-5deg);
}
.jbs-w-clasp {
  transform-origin: 67px 55px;
  transform: rotate(-5deg);
}
.jbs-w-card {
  transform-origin: 46px 36px;
  transform: rotate(-5deg) translateY(5px);
}
.jbs-w-coin {
  transform-origin: 58px 36px;
  opacity: 0;
  transform: translate(0, 8px) scale(0.75);
}
.jbs-w-sparks {
  transform-origin: 74px 26px;
  opacity: 0;
  transform: scale(0.75);
}

/* 2. Key */
.jbs-k-main {
  transform-origin: 34px 48px;
  transform: rotate(-12deg);
}
.jbs-k-tag {
  transform-origin: 33px 56px;
  transform: rotate(0deg);
}
.jbs-k-sparks {
  transform-origin: 75px 32px;
  opacity: 0;
  transform: scale(0.75);
}

/* 3. Palette */
.jbs-p-board {
  transform-origin: 46px 50px;
  transform: rotate(-4deg);
}
.jbs-p-dabs {
  transform-origin: 48px 42px;
  transform: rotate(-4deg);
}
.jbs-p-brush {
  transform-origin: 62px 42px;
  transform: rotate(0deg);
}
.jbs-p-sparks {
  transform-origin: 24px 28px;
  opacity: 0;
  transform: scale(0.75);
}

/* 4. Batch */
.jbs-b-back {
  transform-origin: 48px 66px;
  transform: rotate(-8deg);
}
.jbs-b-mid {
  transform-origin: 48px 66px;
  transform: rotate(6deg);
}
.jbs-b-front {
  transform-origin: 48px 50px;
  transform: rotate(-3deg);
}
.jbs-b-spark {
  transform-origin: 75px 24px;
  opacity: 0;
  transform: scale(0.75);
}

/* 5. Bars */
.jbs-ba-card {
  transform-origin: 48px 50px;
  transform: rotate(-4deg);
}
.jbs-ba-b1 {
  transform-origin: 34px 67px;
  transform: rotate(-4deg) scaleY(1);
}
.jbs-ba-b2 {
  transform-origin: 48px 67px;
  transform: rotate(-4deg) scaleY(1);
}
.jbs-ba-b3 {
  transform-origin: 62px 67px;
  transform: rotate(-4deg) scaleY(1);
}
.jbs-ba-star {
  transform-origin: 68px 24px;
  opacity: 0;
  transform: translateY(4px) scale(0.75);
}

/* 6. Channels */
.jbs-ch-post {
  transform-origin: 48px 52px;
  transform: rotate(-4deg);
}
.jbs-ch-left {
  transform-origin: 48px 39px;
  transform: rotate(-4deg);
}
.jbs-ch-right {
  transform-origin: 48px 58px;
  transform: rotate(-4deg);
}
.jbs-ch-star {
  transform-origin: 68px 24px;
  opacity: 0;
  transform: translateY(4px) scale(0.75);
}

/* 7. Radio */
.jbs-ra-tower {
  transform-origin: 48px 58px;
  transform: rotate(-3deg);
}
.jbs-ra-bulb {
  transform-origin: 48px 36px;
  transform: rotate(-3deg) scale(1);
}
.jbs-ra-inner {
  transform-origin: 48px 36px;
  opacity: 0.72;
  transform: rotate(-3deg) scale(0.95);
}
.jbs-ra-outer {
  transform-origin: 48px 36px;
  opacity: 0.35;
  transform: rotate(-3deg) scale(0.88);
}

/* 8. Crown */
.jbs-cr-body {
  transform-origin: 48px 50px;
  transform: rotate(-5deg);
}
.jbs-cr-gem {
  transform-origin: 48px 65px;
  transform: rotate(-5deg) scale(1);
}
.jbs-cr-sparks {
  transform-origin: 48px 24px;
  opacity: 0;
  transform: translateY(4px) scale(0.75);
}

/* 9. Recharge */
.jbs-rc-card {
  transform-origin: 48px 58px;
  transform: rotate(-5deg);
}
.jbs-rc-coin {
  transform-origin: 48px 34px;
  transform: rotate(-5deg) translateY(5px);
}
.jbs-rc-badge {
  transform-origin: 72px 27px;
  opacity: 0;
  transform: scale(0.75);
}

/* 10. Receipt */
.jbs-re-paper {
  transform-origin: 47px 46px;
  transform: rotate(-5deg);
}
.jbs-re-stamp {
  transform-origin: 65px 61px;
  opacity: 0.8;
  transform: scale(0.88) rotate(8deg);
}

/* 11. Gift */
.jbs-gi-box {
  transform-origin: 48px 60px;
  transform: rotate(-4deg);
}
.jbs-gi-lid {
  transform-origin: 26px 40px;
  transform: rotate(-4deg);
}
.jbs-gi-pop {
  transform-origin: 48px 26px;
  opacity: 0;
  transform: translateY(8px) scale(0.75);
}

/* 12. Share */
.jbs-sh-env {
  transform-origin: 48px 58px;
  transform: rotate(-4deg);
}
.jbs-sh-flap {
  transform-origin: 48px 40px;
  transform: rotate(-4deg);
}
.jbs-sh-letter {
  transform-origin: 48px 45px;
  opacity: 0;
  transform: rotate(-4deg) translateY(8px) scale(0.85);
}

/* 13. Plug */
.jbs-pl-socket {
  transform-origin: 65px 48px;
  transform: rotate(-3deg);
}
.jbs-pl-head {
  transform-origin: 33px 48px;
  transform: rotate(-3deg) translateX(0px);
}
.jbs-pl-spark {
  transform-origin: 48px 22px;
  opacity: 0;
  transform: scale(0.75);
}

/* 14. Admin */
.jbs-ad-dial {
  transform-origin: 46px 54px;
  transform: rotate(-4deg);
}
.jbs-ad-needle {
  transform-origin: 46px 56px;
  transform: rotate(-30deg);
}
.jbs-ad-gear {
  transform-origin: 70px 27px;
  transform: rotate(0deg);
}
.jbs-ad-spark {
  transform-origin: 22px 27px;
  opacity: 0;
  transform: scale(0.75);
}

/* 15. Contact */
.jbs-co-back {
  transform-origin: 60px 42px;
  transform: rotate(4deg);
}
.jbs-co-front {
  transform-origin: 40px 54px;
  transform: rotate(-5deg);
}
.jbs-co-spark {
  transform-origin: 22px 24px;
  opacity: 0;
  transform: scale(0.75);
}

/* 16. Sparkles */
.jbs-sp-main {
  transform-origin: 48px 45px;
  transform: rotate(-5deg);
}
.jbs-sp-a {
  transform-origin: 26px 28px;
  transform: scale(0.92);
}
.jbs-sp-b {
  transform-origin: 69px 65px;
  transform: scale(0.92);
}

/* ==================== Hover Choreography (One-Shot Play & Hold) ==================== */
@media (hover: hover) and (pointer: fine) {
  .jbs-hit:hover .jbs-p {
    transition-duration: 330ms, 220ms;
    transition-timing-function: cubic-bezier(0.22, 1.38, 0.36, 1), cubic-bezier(0.23, 1, 0.32, 1);
  }

  /* 1. Wallet */
  .jbs-hit:hover .jbs-w-body {
    transform: rotate(3deg) translateY(2px);
  }
  .jbs-hit:hover .jbs-w-clasp {
    transform: translate(5px, -1px) rotate(-14deg);
    transition-delay: 25ms, 25ms;
  }
  .jbs-hit:hover .jbs-w-card {
    transform: rotate(2deg) translateY(-7px);
    transition-delay: 50ms, 50ms;
  }
  .jbs-hit:hover .jbs-w-coin {
    opacity: 1;
    transform: translate(-2px, -13px) rotate(12deg) scale(1);
    transition-delay: 85ms, 85ms;
  }
  .jbs-hit:hover .jbs-w-sparks {
    opacity: 1;
    transform: scale(1) rotate(10deg);
    transition-delay: 115ms, 115ms;
  }

  /* 2. Key */
  .jbs-hit:hover .jbs-k-main {
    transform: rotate(14deg) translateY(-1px);
  }
  .jbs-hit:hover .jbs-k-tag {
    transform: rotate(-22deg);
    transition-delay: 55ms, 55ms;
  }
  .jbs-hit:hover .jbs-k-sparks {
    opacity: 1;
    transform: translate(2px, -3px) scale(1);
    transition-delay: 105ms, 105ms;
  }

  /* 3. Palette */
  .jbs-hit:hover .jbs-p-board {
    transform: translateY(-3px) rotate(3deg);
  }
  .jbs-hit:hover .jbs-p-dabs {
    transform: rotate(3deg) scale(1.08);
    transition-delay: 35ms, 35ms;
  }
  .jbs-hit:hover .jbs-p-brush {
    transform: translate(-8px, -6px) rotate(-22deg);
    transition-delay: 50ms, 50ms;
  }
  .jbs-hit:hover .jbs-p-sparks {
    opacity: 1;
    transform: translate(-2px, -3px) scale(1);
    transition-delay: 110ms, 110ms;
  }

  /* 4. Batch */
  .jbs-hit:hover .jbs-b-back {
    transform: translate(-11px, -4px) rotate(-18deg);
    transition-delay: 45ms, 45ms;
  }
  .jbs-hit:hover .jbs-b-mid {
    transform: translate(11px, -3px) rotate(16deg);
    transition-delay: 70ms, 70ms;
  }
  .jbs-hit:hover .jbs-b-front {
    transform: translateY(-4px) scale(1.04);
  }
  .jbs-hit:hover .jbs-b-spark {
    opacity: 1;
    transform: scale(1) rotate(15deg);
    transition-delay: 115ms, 115ms;
  }

  /* 5. Bars */
  .jbs-hit:hover .jbs-ba-card {
    transform: translateY(-2px) rotate(2deg);
  }
  .jbs-hit:hover .jbs-ba-b1 {
    transform: rotate(2deg) scaleY(1.22);
    transition-delay: 30ms, 30ms;
  }
  .jbs-hit:hover .jbs-ba-b2 {
    transform: rotate(2deg) scaleY(1.3);
    transition-delay: 70ms, 70ms;
  }
  .jbs-hit:hover .jbs-ba-b3 {
    transform: rotate(2deg) scaleY(1.2);
    transition-delay: 110ms, 110ms;
  }
  .jbs-hit:hover .jbs-ba-star {
    opacity: 1;
    transform: translateY(-2px) scale(1) rotate(12deg);
    transition-delay: 130ms, 130ms;
  }

  /* 6. Channels */
  .jbs-hit:hover .jbs-ch-post {
    transform: rotate(1deg) translateY(-2px);
  }
  .jbs-hit:hover .jbs-ch-left {
    transform: translate(-5px, -2px) rotate(-8deg);
    transition-delay: 35ms, 35ms;
  }
  .jbs-hit:hover .jbs-ch-right {
    transform: translate(5px, 1px) rotate(6deg);
    transition-delay: 65ms, 65ms;
  }
  .jbs-hit:hover .jbs-ch-star {
    opacity: 1;
    transform: translateY(-2px) scale(1);
    transition-delay: 115ms, 115ms;
  }

  /* 7. Radio */
  .jbs-hit:hover .jbs-ra-tower {
    transform: rotate(0deg) translateY(-2px);
  }
  .jbs-hit:hover .jbs-ra-bulb {
    transform: rotate(0deg) scale(1.18);
    transition-delay: 25ms, 25ms;
  }
  .jbs-hit:hover .jbs-ra-inner {
    opacity: 1;
    transform: rotate(0deg) scale(1.1);
    transition-delay: 55ms, 55ms;
  }
  .jbs-hit:hover .jbs-ra-outer {
    opacity: 1;
    transform: rotate(0deg) scale(1.16);
    transition-delay: 105ms, 105ms;
  }

  /* 8. Crown */
  .jbs-hit:hover .jbs-cr-body {
    transform: translateY(-5px) rotate(4deg) scale(1.05);
  }
  .jbs-hit:hover .jbs-cr-gem {
    transform: translateY(-5px) rotate(4deg) scale(1.22);
    transition-delay: 45ms, 45ms;
  }
  .jbs-hit:hover .jbs-cr-sparks {
    opacity: 1;
    transform: translateY(-4px) scale(1);
    transition-delay: 100ms, 100ms;
  }

  /* 9. Recharge */
  .jbs-hit:hover .jbs-rc-card {
    transform: rotate(3deg) translateY(2px);
  }
  .jbs-hit:hover .jbs-rc-coin {
    transform: translateY(-10px) rotate(14deg) scale(1.08);
    transition-delay: 45ms, 45ms;
  }
  .jbs-hit:hover .jbs-rc-badge {
    opacity: 1;
    transform: translate(2px, -3px) scale(1);
    transition-delay: 105ms, 105ms;
  }

  /* 10. Receipt */
  .jbs-hit:hover .jbs-re-paper {
    transform: rotate(1deg) translateY(-3px);
  }
  .jbs-hit:hover .jbs-re-stamp {
    opacity: 1;
    transform: scale(1.12) rotate(-10deg);
    transition-duration: 340ms, 200ms;
    transition-timing-function: cubic-bezier(0.22, 1.48, 0.36, 1), cubic-bezier(0.23, 1, 0.32, 1);
    transition-delay: 75ms, 75ms;
  }

  /* 11. Gift */
  .jbs-hit:hover .jbs-gi-box {
    transform: rotate(1deg) translateY(2px) scale(1.02);
  }
  .jbs-hit:hover .jbs-gi-lid {
    transform: translate(-3px, -11px) rotate(-14deg);
    transition-delay: 35ms, 35ms;
  }
  .jbs-hit:hover .jbs-gi-pop {
    opacity: 1;
    transform: translateY(-7px) scale(1);
    transition-delay: 95ms, 95ms;
  }

  /* 12. Share */
  .jbs-hit:hover .jbs-sh-env {
    transform: rotate(2deg) translateY(2px);
  }
  .jbs-hit:hover .jbs-sh-flap {
    transform: rotate(2deg) translateY(-2px) scaleY(-0.68);
    transition-delay: 25ms, 25ms;
  }
  .jbs-hit:hover .jbs-sh-letter {
    opacity: 1;
    transform: rotate(-4deg) translateY(-16px) scale(1);
    transition-delay: 65ms, 65ms;
  }

  /* 13. Plug */
  .jbs-hit:hover .jbs-pl-socket {
    transform: rotate(1deg) translateX(2px) scale(1.03);
    transition-delay: 65ms, 65ms;
  }
  .jbs-hit:hover .jbs-pl-head {
    transform: rotate(1deg) translateX(9px);
  }
  .jbs-hit:hover .jbs-pl-spark {
    opacity: 1;
    transform: translateY(-3px) scale(1);
    transition-delay: 110ms, 110ms;
  }

  /* 14. Admin */
  .jbs-hit:hover .jbs-ad-dial {
    transform: rotate(1deg) translateY(-2px);
  }
  .jbs-hit:hover .jbs-ad-needle {
    transform: rotate(24deg);
    transition-delay: 35ms, 35ms;
  }
  .jbs-hit:hover .jbs-ad-gear {
    transform: rotate(60deg);
    transition-delay: 65ms, 65ms;
  }
  .jbs-hit:hover .jbs-ad-spark {
    opacity: 1;
    transform: scale(1);
    transition-delay: 115ms, 115ms;
  }

  /* 15. Contact */
  .jbs-hit:hover .jbs-co-front {
    transform: translate(-3px, -3px) rotate(-8deg) scale(1.04);
  }
  .jbs-hit:hover .jbs-co-back {
    transform: translate(6px, -6px) rotate(11deg) scale(1.06);
    transition-delay: 45ms, 45ms;
  }
  .jbs-hit:hover .jbs-co-spark {
    opacity: 1;
    transform: translate(-2px, -3px) scale(1) rotate(12deg);
    transition-delay: 105ms, 105ms;
  }

  /* 16. Sparkles */
  .jbs-hit:hover .jbs-sp-main {
    transform: rotate(12deg) scale(1.1);
  }
  .jbs-hit:hover .jbs-sp-a {
    transform: translate(-4px, -4px) rotate(-14deg) scale(1.16);
    transition-delay: 45ms, 45ms;
  }
  .jbs-hit:hover .jbs-sp-b {
    transform: translate(4px, 4px) rotate(14deg) scale(1.16);
    transition-delay: 85ms, 85ms;
  }
}

@media (prefers-reduced-motion: reduce) {
  .jbs-p {
    transition-property: opacity;
    transition-duration: 160ms;
  }

  .jbs-hit:hover .jbs-p {
    transform: none !important;
  }
}
</style>
