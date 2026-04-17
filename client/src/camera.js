import * as THREE from 'three'
import { OrbitControls } from 'three/addons/controls/OrbitControls.js'
import { FBXLoader } from 'three/addons/loaders/FBXLoader.js'

// Views in order — matches keys 1/2/3 and V-cycle
export const VIEW = {
  ORBIT: 0,
  COCKPIT: 1,
  PANEL: 2,
}

const VIEW_NAMES = ['ORBIT', 'COCKPIT', 'PANEL']

const COCKPIT_URL = '/cockpit.fbx'
const LOOK_SENSITIVITY = 0.0025
const PITCH_LIMIT = Math.PI / 2 - 0.01
// Target max bounding-box dimension for the cockpit, in world units.
// Ship hull is ~0.3 units long; a cockpit the player sits inside should
// be a bit bigger than that. Tweak freely.
const COCKPIT_TARGET_SIZE = 0.5

export function createCameraSystem(renderer, ship, scene) {
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

  // ── Cockpit look-around state ─────────────────────────────────────────────
  let cockpitModel = null
  let cockpitLoading = false
  const look = { yaw: 0, pitch: 0, dragging: false, lastX: 0, lastY: 0 }
  const _lookEuler = new THREE.Euler(0, 0, 0, 'YXZ')

  // Snapshot the external ship parts so we can hide/show only those
  // (cockpit will be added as a sibling child of the ship group).
  const externalShipParts = [...ship.children]

  // Start in orbit view, positioned relative to ship
  let currentView = VIEW.ORBIT
  const _prevShipPos = new THREE.Vector3()
  const _shipDelta = new THREE.Vector3()
  _enterOrbit(camera, orbitControls, ship, _prevShipPos)

  // ── HUD label ─────────────────────────────────────────────────────────────
  const hudView = document.getElementById('hud-view')
  _updateHUD(hudView, currentView)

  // ── Pointer look (active only in cockpit) ─────────────────────────────────
  const dom = renderer.domElement
  dom.addEventListener('pointerdown', (e) => {
    if (currentView !== VIEW.COCKPIT) return
    look.dragging = true
    look.lastX = e.clientX
    look.lastY = e.clientY
    dom.setPointerCapture?.(e.pointerId)
  })
  dom.addEventListener('pointermove', (e) => {
    if (!look.dragging || currentView !== VIEW.COCKPIT) return
    const dx = e.clientX - look.lastX
    const dy = e.clientY - look.lastY
    look.lastX = e.clientX
    look.lastY = e.clientY
    look.yaw   -= dx * LOOK_SENSITIVITY
    look.pitch -= dy * LOOK_SENSITIVITY
    look.pitch = Math.max(-PITCH_LIMIT, Math.min(PITCH_LIMIT, look.pitch))
  })
  const endDrag = (e) => {
    look.dragging = false
    if (e.pointerId !== undefined) dom.releasePointerCapture?.(e.pointerId)
  }
  dom.addEventListener('pointerup', endDrag)
  dom.addEventListener('pointercancel', endDrag)

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
    _switchTo(next)
    currentView = next
  })

  function _switchTo(view) {
    _updateHUD(hudView, view)

    if (view === VIEW.ORBIT) {
      orbitControls.enabled = true
      _showExternalShip(true)
      if (cockpitModel) cockpitModel.visible = false
      scene.add(camera) // reparent camera back to world
      _enterOrbit(camera, orbitControls, ship, _prevShipPos)
      return
    }

    orbitControls.enabled = false

    if (view === VIEW.COCKPIT) {
      _showExternalShip(false)
      _ensureCockpitLoaded()
      if (cockpitModel) cockpitModel.visible = true
      // Parent the camera to the ship so it inherits orbit + heading;
      // local (0,0,0) = ship's origin = cockpit's origin.
      ship.add(camera)
      camera.position.set(0, 0, 0)
      look.yaw = 0
      look.pitch = 0
      _applyLookRotation()
    } else if (view === VIEW.PANEL) {
      _showExternalShip(true)
      if (cockpitModel) cockpitModel.visible = false
      scene.add(camera)
      console.log('[camera] Control panel view — coming soon')
    }
  }

  function _showExternalShip(visible) {
    for (const part of externalShipParts) part.visible = visible
  }

  function _ensureCockpitLoaded() {
    if (cockpitModel || cockpitLoading) return
    cockpitLoading = true
    new FBXLoader().load(
      COCKPIT_URL,
      (obj) => {
        cockpitModel = obj
        // Auto-fit to ship scale (FBX often exports at 100× — Blender cm).
        const bbox = new THREE.Box3().setFromObject(cockpitModel)
        const size = new THREE.Vector3()
        bbox.getSize(size)
        const maxDim = Math.max(size.x, size.y, size.z) || 1
        const s = COCKPIT_TARGET_SIZE / maxDim
        cockpitModel.scale.setScalar(s)
        // Re-centre so the model's geometric centre sits at ship origin
        const centre = new THREE.Vector3()
        bbox.getCenter(centre)
        cockpitModel.position.copy(centre.multiplyScalar(-s))
        cockpitModel.visible = currentView === VIEW.COCKPIT
        // Child of ship → inherits orbit position + heading
        ship.add(cockpitModel)
        cockpitLoading = false
      },
      undefined,
      (err) => {
        console.error('[camera] Failed to load cockpit.fbx', err)
        cockpitLoading = false
      },
    )
  }

  function _applyLookRotation() {
    _lookEuler.set(look.pitch, look.yaw, 0, 'YXZ')
    camera.quaternion.setFromEuler(_lookEuler)
  }

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
    } else if (currentView === VIEW.COCKPIT) {
      _applyLookRotation()
    }
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

function _updateHUD(el, view) {
  if (!el) return
  el.textContent = `VIEW: ${VIEW_NAMES[view]}`
}
