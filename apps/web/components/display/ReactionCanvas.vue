<script setup lang="ts">
interface Particle { emoji: string; x: number; y: number; vx: number; vy: number; size: number; life: number; ttl: number; spin: number; rotation: number }
const canvas = ref<HTMLCanvasElement | null>(null)
let context: CanvasRenderingContext2D | null = null
let frame = 0
let particles: Particle[] = []
let width = 0
let height = 0
const MAX_PARTICLES = 100
function resize() { if (!canvas.value) return; const ratio = Math.min(devicePixelRatio, 2); width = innerWidth; height = innerHeight; canvas.value.width = width * ratio; canvas.value.height = height * ratio; canvas.value.style.width = `${width}px`; canvas.value.style.height = `${height}px`; context = canvas.value.getContext('2d'); context?.setTransform(ratio, 0, 0, ratio, 0, 0) }
function addBatch(batch: Record<string, number>) {
  for (const [emoji, count] of Object.entries(batch)) {
    const representatives = Math.min(60, Math.max(1, Math.round(Math.sqrt(count) * 2.4)))
    for (let index = 0; index < representatives && particles.length < MAX_PARTICLES; index += 1) {
      const energy = Math.min(2.2, 1 + count / 300)
      particles.push({ emoji, x: Math.random() * width, y: height + 80, vx: (Math.random() - .5) * 1.8 * energy, vy: -(2.2 + Math.random() * 3.2) * energy, size: 34 + Math.random() * 46 + Math.min(32, count / 8), life: 0, ttl: 170 + Math.random() * 100, spin: (Math.random() - .5) * .025, rotation: 0 })
    }
  }
}
function clear() { particles = []; context?.clearRect(0, 0, width, height) }
function animate() {
  context?.clearRect(0, 0, width, height)
  particles = particles.filter((particle) => {
    particle.life += 1; particle.x += particle.vx; particle.y += particle.vy; particle.vy += .012; particle.rotation += particle.spin
    const fadeIn = Math.min(1, particle.life / 12); const fadeOut = Math.min(1, (particle.ttl - particle.life) / 35)
    if (context) { context.save(); context.globalAlpha = Math.max(0, Math.min(fadeIn, fadeOut)); context.translate(particle.x, particle.y); context.rotate(particle.rotation); context.font = `${particle.size}px "Apple Color Emoji","Segoe UI Emoji",sans-serif`; context.textAlign = 'center'; context.fillText(particle.emoji, 0, 0); context.restore() }
    return particle.life < particle.ttl && particle.y > -120
  })
  frame = requestAnimationFrame(animate)
}
onMounted(() => { resize(); addEventListener('resize', resize); frame = requestAnimationFrame(animate) })
onBeforeUnmount(() => { cancelAnimationFrame(frame); removeEventListener('resize', resize) })
defineExpose({ addBatch, clear })
</script>
<template><canvas ref="canvas" class="absolute inset-0 h-full w-full" aria-label="Live emoji reactions" /></template>
