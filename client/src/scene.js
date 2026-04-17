import * as THREE from 'three'
import { OrbitControls } from 'three/addons/controls/OrbitControls.js'

// ── Seeded PRNG (Mulberry32) ──────────────────────────────────────────────────
function makePRNG(seed) {
  let s = seed | 0
  return () => {
    s = (s + 0x6d2b79f5) | 0
    let t = Math.imul(s ^ (s >>> 15), 1 | s)
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296
  }
}

// ── Planet shader ─────────────────────────────────────────────────────────────
const planetVertexShader = /* glsl */ `
  varying vec3 vNormal;
  varying vec3 vPosition;
  varying vec3 vWorldNormal;

  void main() {
    vNormal = normalize(normalMatrix * normal);
    vPosition = position;
    vWorldNormal = normalize((modelMatrix * vec4(normal, 0.0)).xyz);
    gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
  }
`

const planetFragmentShader = /* glsl */ `
  uniform vec3 uOcean;
  uniform vec3 uLand;
  uniform vec3 uIce;
  uniform float uSeed;
  uniform vec3 uSunDir;

  varying vec3 vNormal;
  varying vec3 vPosition;
  varying vec3 vWorldNormal;

  float hash(vec3 p) {
    p = fract(p * 0.3183099 + 0.1);
    p *= 17.0;
    return fract(p.x * p.y * p.z * (p.x + p.y + p.z));
  }

  float noise(vec3 p) {
    vec3 i = floor(p);
    vec3 f = fract(p);
    f = f * f * (3.0 - 2.0 * f);
    return mix(
      mix(mix(hash(i), hash(i + vec3(1,0,0)), f.x),
          mix(hash(i + vec3(0,1,0)), hash(i + vec3(1,1,0)), f.x), f.y),
      mix(mix(hash(i + vec3(0,0,1)), hash(i + vec3(1,0,1)), f.x),
          mix(hash(i + vec3(0,1,1)), hash(i + vec3(1,1,1)), f.x), f.y), f.z
    );
  }

  float fbm(vec3 p) {
    float v = 0.0;
    v += 1.000 * noise(p * 1.0);
    v += 0.500 * noise(p * 2.0);
    v += 0.250 * noise(p * 4.0);
    v += 0.125 * noise(p * 8.0);
    return v / 1.875;
  }

  void main() {
    float seedOffset = uSeed * 0.0001;
    float n = fbm(vPosition * 1.4 + seedOffset);

    // Latitude-based ice caps
    float latitude = abs(normalize(vPosition).y);
    float iceMix = smoothstep(0.72, 0.88, latitude);

    vec3 color;
    if (n < 0.44) {
      // Ocean — slight depth variation
      float depth = smoothstep(0.0, 0.44, n);
      color = mix(uOcean * 0.6, uOcean, depth);
    } else {
      // Land — varies by elevation
      float elevation = (n - 0.44) / 0.56;
      color = mix(uLand * 0.8, uLand * 1.1, elevation);
    }

    color = mix(color, uIce, iceMix);

    // Diffuse lighting
    float diff = max(dot(vWorldNormal, uSunDir), 0.0);
    float ambient = 0.08;
    color *= (ambient + (1.0 - ambient) * diff);

    // Specular on ocean
    if (n < 0.44 && iceMix < 0.1) {
      vec3 viewDir = normalize(cameraPosition - vPosition);
      vec3 halfDir = normalize(uSunDir + viewDir);
      float spec = pow(max(dot(vWorldNormal, halfDir), 0.0), 64.0);
      color += vec3(0.4) * spec * diff;
    }

    gl_FragColor = vec4(color, 1.0);
  }
`

// ── Atmosphere shader ─────────────────────────────────────────────────────────
const atmosphereVertexShader = /* glsl */ `
  varying vec3 vNormal;
  void main() {
    vNormal = normalize(normalMatrix * normal);
    gl_Position = projectionMatrix * modelViewMatrix * vec4(position, 1.0);
  }
`

const atmosphereFragmentShader = /* glsl */ `
  uniform vec3 uAtmColor;
  varying vec3 vNormal;

  void main() {
    float intensity = pow(0.65 - dot(vNormal, vec3(0.0, 0.0, 1.0)), 3.0);
    intensity = clamp(intensity, 0.0, 1.0);
    gl_FragColor = vec4(uAtmColor, intensity * 0.7);
  }
`

// ── Main scene factory ────────────────────────────────────────────────────────
export function createScene(canvas, seed, { onOrbitUpdate } = {}) {
  const rng = makePRNG(seed)

  // Renderer
  const renderer = new THREE.WebGLRenderer({ canvas, antialias: true })
  renderer.setSize(window.innerWidth, window.innerHeight)
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2))
  renderer.toneMapping = THREE.ACESFilmicToneMapping
  renderer.toneMappingExposure = 1.1

  // Scene + camera
  const scene = new THREE.Scene()
  scene.background = new THREE.Color(0x020209)

  const camera = new THREE.PerspectiveCamera(55, window.innerWidth / window.innerHeight, 0.01, 1000)
  camera.position.set(9, 4, 9)

  const controls = new OrbitControls(camera, renderer.domElement)
  controls.enableDamping = true
  controls.dampingFactor = 0.06
  controls.minDistance = 4
  controls.maxDistance = 40

  // Lighting
  scene.add(new THREE.AmbientLight(0x0a0a1a, 1.0))
  const sun = new THREE.DirectionalLight(0xfff6e0, 3.5)
  sun.position.set(20, 12, 18)
  scene.add(sun)
  const sunDir = sun.position.clone().normalize()

  // ── Stars ──
  const starRng = makePRNG(seed + 9999)
  const starCount = 10000
  const starPos = new Float32Array(starCount * 3)
  const starColors = new Float32Array(starCount * 3)
  for (let i = 0; i < starCount; i++) {
    const theta = starRng() * Math.PI * 2
    const phi = Math.acos(2 * starRng() - 1)
    const r = 250 + starRng() * 80
    starPos[i * 3 + 0] = r * Math.sin(phi) * Math.cos(theta)
    starPos[i * 3 + 1] = r * Math.sin(phi) * Math.sin(theta)
    starPos[i * 3 + 2] = r * Math.cos(phi)
    // Slight colour variation: white, blue-white, warm
    const t = starRng()
    if (t < 0.15) { starColors[i*3]=0.7; starColors[i*3+1]=0.8; starColors[i*3+2]=1.0 }       // blue
    else if (t < 0.25) { starColors[i*3]=1.0; starColors[i*3+1]=0.9; starColors[i*3+2]=0.75 } // warm
    else { starColors[i*3]=1; starColors[i*3+1]=1; starColors[i*3+2]=1 }
  }
  const starGeo = new THREE.BufferGeometry()
  starGeo.setAttribute('position', new THREE.BufferAttribute(starPos, 3))
  starGeo.setAttribute('color', new THREE.BufferAttribute(starColors, 3))
  const starMat = new THREE.PointsMaterial({ size: 0.28, sizeAttenuation: true, vertexColors: true })
  scene.add(new THREE.Points(starGeo, starMat))

  // ── Planet ──
  const hue = rng() * 360
  const saturation = 40 + rng() * 40
  const oceanColor = new THREE.Color().setHSL(hue / 360, saturation / 100, 0.32)
  const landHue = (hue + 60 + rng() * 60) % 360
  const landColor = new THREE.Color().setHSL(landHue / 360, 0.35, 0.38)
  const iceColor = new THREE.Color(0xd8eef5)
  const atmosphereColor = new THREE.Color().setHSL(hue / 360, 0.7, 0.65)

  const planetGeo = new THREE.SphereGeometry(2, 128, 128)
  const planetMat = new THREE.ShaderMaterial({
    vertexShader: planetVertexShader,
    fragmentShader: planetFragmentShader,
    uniforms: {
      uOcean: { value: oceanColor },
      uLand: { value: landColor },
      uIce: { value: iceColor },
      uSeed: { value: seed },
      uSunDir: { value: sunDir },
    },
  })
  const planet = new THREE.Mesh(planetGeo, planetMat)
  scene.add(planet)

  // Atmosphere glow (backface, additive)
  const atmGeo = new THREE.SphereGeometry(2.18, 64, 64)
  const atmMat = new THREE.ShaderMaterial({
    vertexShader: atmosphereVertexShader,
    fragmentShader: atmosphereFragmentShader,
    uniforms: { uAtmColor: { value: atmosphereColor } },
    side: THREE.BackSide,
    transparent: true,
    blending: THREE.AdditiveBlending,
    depthWrite: false,
  })
  scene.add(new THREE.Mesh(atmGeo, atmMat))

  // ── Spacecraft ──
  const ship = buildSpacecraft(rng)
  scene.add(ship)

  // ── Orbit parameters (deterministic from seed) ──
  const orbitRadius   = 3.5 + rng() * 2.0          // 3.5 – 5.5 units
  const inclination   = (rng() - 0.5) * 0.9        // tilt
  const phaseOffset   = rng() * Math.PI * 2         // starting angle
  const orbitPeriod   = 40000 + rng() * 40000       // 40 – 80 seconds

  onOrbitUpdate?.(orbitRadius, orbitPeriod)

  // ── Animation loop ──
  const _next = new THREE.Vector3()

  function animate() {
    requestAnimationFrame(animate)

    const t = Date.now()
    const angle = ((t % orbitPeriod) / orbitPeriod) * Math.PI * 2 + phaseOffset

    ship.position.set(
      orbitRadius * Math.cos(angle),
      orbitRadius * Math.sin(inclination) * Math.sin(angle),
      orbitRadius * Math.sin(angle) * Math.cos(inclination),
    )

    // Face direction of travel
    const next = angle + 0.01
    _next.set(
      orbitRadius * Math.cos(next),
      orbitRadius * Math.sin(inclination) * Math.sin(next),
      orbitRadius * Math.sin(next) * Math.cos(inclination),
    )
    ship.lookAt(_next)

    planet.rotation.y += 0.0003

    controls.update()
    renderer.render(scene, camera)
  }

  animate()

  // Resize
  window.addEventListener('resize', () => {
    camera.aspect = window.innerWidth / window.innerHeight
    camera.updateProjectionMatrix()
    renderer.setSize(window.innerWidth, window.innerHeight)
  })
}

// ── Spacecraft geometry ───────────────────────────────────────────────────────
function buildSpacecraft(rng) {
  const group = new THREE.Group()

  const bodyMat = new THREE.MeshStandardMaterial({
    color: 0xbbc8d4,
    metalness: 0.85,
    roughness: 0.25,
    emissive: 0x111822,
  })
  const panelMat = new THREE.MeshStandardMaterial({
    color: 0x1a2f50,
    metalness: 0.3,
    roughness: 0.5,
    emissive: 0x061020,
  })
  const thrusterMat = new THREE.MeshStandardMaterial({
    color: 0x888899,
    metalness: 0.9,
    roughness: 0.15,
  })
  const engineGlowMat = new THREE.MeshBasicMaterial({ color: 0x6699ff })

  // Hull
  const hull = new THREE.Mesh(new THREE.BoxGeometry(0.10, 0.075, 0.30), bodyMat)
  group.add(hull)

  // Nose cone
  const nose = new THREE.Mesh(new THREE.ConeGeometry(0.05, 0.11, 8), bodyMat)
  nose.rotation.x = Math.PI / 2
  nose.position.z = 0.205
  group.add(nose)

  // Solar panels (left + right)
  for (const side of [-1, 1]) {
    const panel = new THREE.Mesh(new THREE.BoxGeometry(0.38, 0.004, 0.11), panelMat)
    panel.position.x = side * 0.24
    panel.position.z = -0.02
    group.add(panel)

    // Panel frame
    const frame = new THREE.Mesh(new THREE.BoxGeometry(0.40, 0.006, 0.02), bodyMat)
    frame.position.x = side * 0.24
    frame.position.z = -0.02
    group.add(frame)
  }

  // Thruster nozzle
  const thruster = new THREE.Mesh(new THREE.CylinderGeometry(0.025, 0.038, 0.06, 8), thrusterMat)
  thruster.rotation.x = Math.PI / 2
  thruster.position.z = -0.18
  group.add(thruster)

  // Engine plume glow
  const glow = new THREE.Mesh(new THREE.CircleGeometry(0.02, 8), engineGlowMat)
  glow.rotation.x = Math.PI / 2
  glow.position.z = -0.215
  group.add(glow)

  return group
}
