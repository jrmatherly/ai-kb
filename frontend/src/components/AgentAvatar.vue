<template>
  <div 
    class="agent-avatar" 
    :style="avatarStyle"
    :class="{ 'agent-avatar-small': size === 'small', 'agent-avatar-large': size === 'large' }"
  >
    <!-- Star decoration, blended into the background -->
    <svg class="agent-sparkles" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">
      <!-- Small star at the top right -->
      <path d="M24 5L24.4 6.6C24.45 6.85 24.65 7.05 24.9 7.1L26.5 7.5L24.9 7.9C24.65 7.95 24.45 8.15 24.4 8.4L24 10L23.6 8.4C23.55 8.15 23.35 7.95 23.1 7.9L21.5 7.5L23.1 7.1C23.35 7.05 23.55 6.85 23.6 6.6L24 5Z" fill="rgba(255,255,255,0.6)"/>
      <!-- Small star at the bottom left -->
      <path d="M7 22L7.4 23.6C7.45 23.85 7.65 24.05 7.9 24.1L9.5 24.5L7.9 24.9C7.65 24.95 7.45 25.15 7.4 25.4L7 27L6.6 25.4C6.55 25.15 6.35 24.95 6.1 24.9L4.5 24.5L6.1 24.1C6.35 24.05 6.55 23.85 6.6 23.6L7 22Z" fill="rgba(255,255,255,0.5)"/>
    </svg>
    <span class="agent-avatar-letter" :style="letterStyle">{{ letter }}</span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';

const props = withDefaults(defineProps<{
  name: string;
  size?: 'small' | 'medium' | 'large';
}>(), {
  size: 'medium'
});

// Predefined gradient schemes: modern, soft, professional
const gradients = [
  { from: '#667eea', to: '#764ba2' },  // purple-blue gradient
  { from: '#4facfe', to: '#00f2fe' },  // blue-cyan gradient
  { from: '#43e97b', to: '#38f9d7' },  // green-cyan gradient
  { from: '#11998e', to: '#38ef7d' },  // deep green gradient
  { from: '#5ee7df', to: '#b490ca' },  // cyan-purple gradient
  { from: '#48c6ef', to: '#6f86d6' },  // blue-purple gradient
  { from: '#a8edea', to: '#fed6e3' },  // cyan-pink gradient (soft)
  { from: '#667db6', to: '#0082c8' },  // blue gradient
  { from: '#36d1dc', to: '#5b86e5' },  // cyan-blue gradient
  { from: '#56ab2f', to: '#a8e063' },  // grass green gradient
  { from: '#614385', to: '#516395' },  // deep purple-blue gradient
  { from: '#02aab0', to: '#00cdac' },  // teal gradient
  { from: '#6a82fb', to: '#fc5c7d' },  // blue-pink gradient (soft)
  { from: '#834d9b', to: '#d04ed6' },  // purple gradient
  { from: '#4776e6', to: '#8e54e9' },  // blue-purple gradient
  { from: '#00b09b', to: '#96c93d' },  // teal-green gradient
];

// Stable hash of the name
const hashCode = (str: string): number => {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    const char = str.charCodeAt(i);
    hash = ((hash << 5) - hash) + char;
    hash = hash & hash;
  }
  return Math.abs(hash);
};

// First letter (CJK supported)
const letter = computed(() => {
  const name = props.name?.trim() || '';
  if (!name) return '?';
  
  // First character
  const firstChar = name.charAt(0);
  
  // Uppercase English letters
  if (/[a-zA-Z]/.test(firstChar)) {
    return firstChar.toUpperCase();
  }
  
  // CJK and other characters are returned as is
  return firstChar;
});

// Pick a gradient from the name
const gradient = computed(() => {
  const hash = hashCode(props.name || '');
  return gradients[hash % gradients.length];
});

// Build the style
const avatarStyle = computed(() => {
  const g = gradient.value;
  return {
    background: `linear-gradient(135deg, ${g.from} 0%, ${g.to} 100%)`
  };
});

// Letter style: white with a background-color shadow for depth
const letterStyle = computed(() => {
  const g = gradient.value;
  return {
    textShadow: `0 1px 2px ${g.to}80, 0 0 8px ${g.from}30`
  };
});
</script>

<style scoped lang="less">
.agent-avatar {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: var(--app-radius-md);
  flex-shrink: 0;
  box-shadow: var(--td-shadow-2);
  overflow: hidden;
  
  &.agent-avatar-small {
    width: 22px;
    height: 22px;
    border-radius: 5px;
    box-shadow: none;
    
    .agent-avatar-letter {
      font-size: var(--app-text-xs);
    }
    
    .agent-sparkles {
      display: none;
    }
  }
  
  &.agent-avatar-large {
    width: 48px;
    height: 48px;
    border-radius: var(--app-radius-xl);
    
    .agent-avatar-letter {
      font-size: var(--app-text-3xl);
    }
    
    .agent-sparkles {
      opacity: 0.9;
    }
  }
}

.agent-sparkles {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
  opacity: 0.85;
}

.agent-avatar-letter {
  position: relative;
  z-index: 1;
  color: var(--td-text-color-anti);
  font-size: var(--app-text-base);
  font-weight: 600;
  font-family: var(--app-font-family);
}
</style>
