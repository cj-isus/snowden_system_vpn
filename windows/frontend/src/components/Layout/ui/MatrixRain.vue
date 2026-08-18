<script lang="ts" setup>
import { ref, onMounted, onUnmounted } from "vue"

const canvasRef = ref<HTMLCanvasElement | null>(null)

const chars = "アイウエオカキクケコサシスセソタチツテトナニヌネノハヒフヘホマミムメモヤユヨラリルレロワヲン0123456789ABCDEF"
const charArray = chars.split("")

let animationId: number | undefined
let ctx: CanvasRenderingContext2D | null = null

onMounted(() => {
  const canvas = canvasRef.value
  if (!canvas) return

  ctx = canvas.getContext("2d")
  if (!ctx) return

  const resize = () => {
    canvas.width = canvas.offsetWidth
    canvas.height = canvas.offsetHeight
  }
  resize()
  window.addEventListener("resize", resize)

  const fontSize = 12
  const columns = Math.floor(canvas.width / fontSize)
  const drops: number[] = Array(columns).fill(0).map(() => Math.random() * -100)

  function draw() {
    if (!ctx || !canvas) return
    ctx.fillStyle = "rgba(5, 5, 5, 0.05)"
    ctx.fillRect(0, 0, canvas.width, canvas.height)

    ctx.font = `${fontSize}px "JetBrains Mono", monospace`

    for (let i = 0; i < drops.length; i++) {
      const char = charArray[Math.floor(Math.random() * charArray.length)]
      const x = i * fontSize
      const y = drops[i] * fontSize

      // Head - bright green
      ctx.fillStyle = `rgba(60, 255, 90, ${0.8 + Math.random() * 0.2})`
      ctx.fillText(char, x, y)

      // Trail - fading green
      if (drops[i] > 0) {
        for (let t = 1; t < 8 && y - t * fontSize > 0; t++) {
          const trailChar = charArray[Math.floor(Math.random() * charArray.length)]
          const alpha = Math.max(0, 0.3 - t * 0.04)
          ctx.fillStyle = `rgba(0, 200, 60, ${alpha})`
          ctx.fillText(trailChar, x, y - t * fontSize)
        }
      }

      drops[i]++
      if (y > canvas.height && Math.random() > 0.975) {
        drops[i] = 0
      }
    }

    animationId = requestAnimationFrame(draw)
  }

  draw()

  onUnmounted(() => {
    window.removeEventListener("resize", resize)
    if (animationId) cancelAnimationFrame(animationId)
  })
})
</script>

<template>
  <canvas ref="canvasRef" class="matrix-canvas" />
</template>

<style scoped>
.matrix-canvas {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
  z-index: 0;
  opacity: 0.15;
}
</style>
