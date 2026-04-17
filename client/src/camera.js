import * as THREE from 'three'
import { OrbitControls } from 'three/addons/controls/OrbitControls.js'

// Views in order — matches keys 1/2/3 and V-cycle
export const VIEW = {
  ORBIT: 0,
  COCKPIT: 1,
  PANEL: 2,
}

const VIEW_NAMES = ['ORBIT', 'COCKPIT', 'PANEL']

export function createCameraSystem(renderer, ship) {
  // ── Shared camera ─────────────────────────────────────────────────────────
  const camera = new THREE.PerspectiveCamera(
    55,
    window.innerWidth / window.innerHeight,
    0.01,
    1000,
  )

  // ── Orbit controls (View 1) ───────────────────────────────────────────────
  const orbitControls = new OrbitControls(camera, renderer.domElement)
  orbitControls.enableDamping = true
  orbitControls.dampingFactor = 0.06
  orbitControls.minDistance = 0.5
  orbitControls.maxDistance = 40
  orbitControls.enablePan = false

  // Start in orbit view, positioned relative to ship
  let currentView = VIEW.ORBIT
  const _prevShipPos = new THREE.Vector3()
  const _shipDelta = new THREE.Vector3()
  _enterOrbit(camera, orbitControls, ship, _prevShipPos)

  // ── HUD label ─────────────────────────────────────────────────────────────
  const hudView = document.getElementById('hud-view')
  _updateHUD(hudView, currentView)

  // ── Keyboard switching ────────────────────────────────────────────────────
  window.addEventListener('keydown', (e) => {
    const key = e.key
    let next = currentView

    if (key === '1') next = VIEW.ORBIT
    else if (key === '2') next = VIEW.COCKPIT
    else if (key === '3') next = VIEW.PANEL
    else if (key === 'v' || key === 'V') next = (currentView + 1) % 3
    else return

    if (next === currentView) return
    _switchTo(next, camera, orbitControls, ship, hudView, _prevShipPos)
    currentView = next
  })

  // ── Per-frame update ──────────────────────────────────────────────────────
  function update() {
    if (currentView === VIEW.ORBIT) {
      // Translate the whole orbit rig with the ship so the user's
      // azimuth/elevation/zoom stays ship-relative (third-person follow).
      _shipDelta.subVectors(ship.position, _prevShipPos)
      camera.position.add(_shipDelta)
      orbitControls.target.add(_shipDelta)
      _prevShipPos.copy(ship.position)
      orbitControls.update()
    }
    // Cockpit / Panel: handled when those views are implemented (Phase 2)
  }

  // ── Resize ────────────────────────────────────────────────────────────────
  window.addEventListener('resize', () => {
    camera.aspect = window.innerWidth / window.innerHeight
    camera.updateProjectionMatrix()
  })

  return { camera, update, getView: () => currentView }
}

// ── Internal helpers ──────────────────────────────────────────────────────────

function _enterOrbit(camera, controls, ship, prevShipPos) {
  // Position camera relative to ship's current location so the ship is
  // roughly centred on screen from the start
  const offset = new THREE.Vector3(0.4, 0.15, 0.6)
  camera.position.copy(ship.position).add(offset)
  controls.target.copy(ship.position)
  controls.enabled = true
  controls.update()
  prevShipPos.copy(ship.position)
}

function _switchTo(view, camera, orbitControls, ship, hudEl, prevShipPos) {
  _updateHUD(hudEl, view)

  if (view === VIEW.ORBIT) {
    orbitControls.enabled = true
    _enterOrbit(camera, orbitControls, ship, prevShipPos)
    return
  }

  // Disable orbit mouse control for non-orbit views
  orbitControls.enabled = false

  if (view === VIEW.COCKPIT) {
    // Stub — camera will be parented to ship in a later step
    console.log('[camera] Cockpit view — coming soon')
  } else if (view === VIEW.PANEL) {
    // Stub — instrument panel overlay in Phase 2
    console.log('[camera] Control panel view — coming soon')
  }
}

function _updateHUD(el, view) {
  if (!el) return
  el.textContent = `VIEW: ${VIEW_NAMES[view]}`
}
