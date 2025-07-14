<template>
  <div class="pipeline-spinner" role="status" aria-label="Pipeline running">
    <div class="chase-dot chase-dot-1"></div>
    <div class="chase-dot chase-dot-2"></div>
    <div class="chase-dot chase-dot-3"></div>
    <div class="chase-dot chase-dot-4"></div>
    <div class="chase-dot chase-dot-5"></div>
    <div class="chase-dot chase-dot-6"></div>
  </div>
</template>

<script lang="ts" setup>
// Pipeline progress spinner component
</script>

<style scoped>
.pipeline-spinner {
  position: relative;
  display: inline-block;
  width: 100%;
  height: 100%;
  animation: pipeline-chase 2.5s infinite linear both;
}

.chase-dot {
  width: 100%;
  height: 100%;
  position: absolute;
  left: 0;
  top: 0;
  animation: pipeline-chase-dot 2s infinite ease-in-out both;
}

.chase-dot:before {
  content: '';
  display: block;
  width: 25%;
  height: 25%;
  background-color: #ff9b00;
  border-radius: 100%;
  animation: pipeline-chase-dot-before 2s infinite ease-in-out both;
}

/* Staggered animation delays for chase effect */
.chase-dot-1 {
  animation-delay: -1.1s;
}
.chase-dot-2 {
  animation-delay: -1s;
}
.chase-dot-3 {
  animation-delay: -0.9s;
}
.chase-dot-4 {
  animation-delay: -0.8s;
}
.chase-dot-5 {
  animation-delay: -0.7s;
}
.chase-dot-6 {
  animation-delay: -0.6s;
}

.chase-dot-1:before {
  animation-delay: -1.1s;
}
.chase-dot-2:before {
  animation-delay: -1s;
}
.chase-dot-3:before {
  animation-delay: -0.9s;
}
.chase-dot-4:before {
  animation-delay: -0.8s;
}
.chase-dot-5:before {
  animation-delay: -0.7s;
}
.chase-dot-6:before {
  animation-delay: -0.6s;
}

@keyframes pipeline-chase {
  100% {
    transform: rotate(360deg);
  }
}

@keyframes pipeline-chase-dot {
  80%,
  100% {
    transform: rotate(360deg);
  }
}

@keyframes pipeline-chase-dot-before {
  50% {
    transform: scale(0.4);
  }
  100%,
  0% {
    transform: scale(1);
  }
}
</style>
