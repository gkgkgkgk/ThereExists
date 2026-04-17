import { createScene } from './scene.js'
import './style.css'

async function init() {
  const apiUrl = import.meta.env.VITE_API_URL || 'http://localhost:8080'

  let seed = Math.floor(Math.random() * 2147483647)
  let playerId = null

  try {
    let url = `${apiUrl}/api/player`
    const storedId = localStorage.getItem('player_id')
    if (storedId) url += `?id=${storedId}`

    const response = await fetch(url)
    if (!response.ok) throw new Error(`API responded ${response.status}`)

    const player = await response.json()
    seed = player.seed
    playerId = player.id
    localStorage.setItem('player_id', player.id)
  } catch (err) {
    console.warn('Could not reach API, using local seed:', err.message)
  }

  // Populate HUD
  const shortId = playerId ? playerId.slice(0, 8).toUpperCase() : 'OFFLINE'
  document.getElementById('hud-player-id').textContent = shortId
  document.getElementById('hud-system').textContent = `SEED-${(seed >>> 0).toString(16).toUpperCase().padStart(8, '0')}`

  // Start scene
  const canvas = document.getElementById('canvas')
  createScene(canvas, seed, {
    onOrbitUpdate: (radius, period) => {
      document.getElementById('hud-orbit').textContent =
        `r=${radius.toFixed(2)} AU  T=${(period / 1000).toFixed(0)}s`
    },
  })

  // Fade out loading screen
  const loading = document.getElementById('loading')
  loading.classList.add('hidden')
  setTimeout(() => loading.remove(), 900)
}

init()
