# $\exists$ (There Exists)
Vision Statement: A persistent, real-time engineering simulation set in a mathematically vast, sparse galaxy.

## Brainstorming
The project originated from a desire to create a "passion project" that avoids the traditional hurdles of game development (art, writing, and complex game engines) by leaning into the beauty of emergent systems and engineering-driven gameplay.

## Core Pillars
### The Sublime Void
Emphasizing the "Awe and Insignificance" of space. The universe is indifferent, and finding a "trace" of another person should feel like a statistically improbable miracle.

### Asynchronous Stewardship
Moving away from high-reflex gameplay toward a "check-in" loop. The simulation runs in real-time; space is big, and the "waiting" is a core mechanic that adds weight to every decision.

### Math-as-Art
Leveraging procedural generation and sensor-driven data (Point Clouds, Spectroscopy) to create a "Vintage NASA" or "Star Wars" aesthetic without the need for traditional hand-drawn assets.

### Together but Alone
A shared universe defined by its emptiness. Players populate a unified database, but the likelihood of real-time interaction is near zero. The "multiplayer" element is found in the legacy left behind: beacons, wreckage, and naming rights.

## Systems Architecture
The system is designed to be a high-performance, accessible web-based platform, utilizing a full-stack orchestration capable of handling a persistent global state.

### Backend and Persistence
#### The Global Ledger
A centralized, immutable database that serves as the single source of truth for the universe. It stores all celestial bodies, player data, and historical events.

#### Deterministic Simulation
Every celestial body is generated from a seed, ensuring that the universe is consistent across all clients. This allows for a persistent, shared universe without the need for a centralized game server.

#### Signal Propagation
Signals decay, propogate, reflect, and refract through the universe. Signals follow the inverse-square law.

### Tech Stack

* Frontend: Three.js
* Backend: Golang
* Database: PostgreSQL with spatial indexing
* AI Layer: lightweight local LLMs like Ollama or Gemma
* Infrastructure: Docker, Kubernetes, etc.

## Gameplay Loop
The interface is a hybrid of a retro-futuristic terminal and tactile controls, designed to make the player feel like an operator, not a pilot.

### Ship Systems
#### Scanners
Scanners come in many forms; some are wide-field and shallow, others are narrow-field and deep. Some detect mass, others detect energy, others detect composition, others detect life. Some can detect radio waves, cosmic rays, and other forms of electromagnetic radiation, gravitational waves, and other phenomena.

#### Flight
Flight is controlled by a thruster system that allows for movement in any direction. The user can plot a course or fly manually. Complex maneuvers are possible, but require skill and practice. An autopilot system is available for long-distance travel and complex orbital mechanics.

#### Probes
Probes are unmanned vehicles that can be sent to explore distant star systems. They are equipped with a variety of sensors and can be used to collect data about the environment. Probes can be launched from a ship or from a planet.

#### Spacecraft OS
Every spacecraft has its own operating system with its own intricacies, as everyone individual has come from a different civilization with different aesthetics, engineering principles, and design philosophies. The OS is a gateway into the ship's systems, and can be used to manipulate the three primary systems: Flight, Scanners, and Probes. One constant across all spacecraft is an AI co-pilot, which can assist you with your journey.

### The Core Loop
The player starts in orbit of a random moon in a random star system. They begin with a random set of resources and a random ship. The player can interact with the three primary systems: Flight, Scanners, and Probes. 

#### Flight
The player can manually control the ship using a thruster system that allows for movement in any direction. This is useful for close encounters, navigating asteroid fields, or if the autopilot is unavailable. The player can also use the autopilot system to plot a course and execute it automatically. This involves a few different techniques, including warp drives, wormholes, orbital mechanics, and more. 

#### Scanners
The player can use the scanner system to scan for celestial bodies, resources, and other phenomena. This can affect the ship's power consumption, and can be used to detect other ships, planets, moons, asteroids, and more. Detection is important - new phenomena are discovered all the time, and the player can then document them and name them. Additionally, detection can be used to plot courses, detect dangers, and more. 

#### Probes
The player can use probes to gather data and resources, broadcast signals, or explore areas that are too dangerous for the ship. Probes can be launched from a ship or from a planet. Probes can be equipped with a variety of sensors and can be used to collect data about the environment. Probes can be satellites, landers, rovers, drones, etc.

#### Engineering
Using the resources on board the ship as well as resources gathered by the player, the player can repair, build, and upgrade depending on what they have access to and what plans are available to them. 

#### Operational Management
Players must manage their ship's resources. This includes the ship's fuel, power, temperature, and more. They must also manage their life support, which can differ depending on the ship and the environment (some need oxygen to breath, others need nitrogen, etc.). 

#### Death
If the player dies, they lose their ship and all their resources. They will be given a summary of the legacy they left behind, and some statistics about their journey. They can restart the game with a new ship and new resources. To prevent players from constantly dying and restarting, new ships and resources can only be procured once per real-life day. Additionally, when you spawn, the player will be given a set of brief descriptions for three space programs they can start at, so they can choose where to begin their journey.

#### To Infinity and Beyond
Between sensors, engineering, and probes, the player can gather data and resources to help them survive in the universe. With flight and navigation, the player can travel to new star systems and explore the galaxy.

## Interface & Visuals

### Camera and Views
There are three views:
1. Ship View: The player can see the ship from a third-person perspective. This view is used for manual flight and for observing their surroundings.
2. Cockpit View: The player can see the ship from a first-person perspective. They can see out their window at the stars, and can interact with the ship's systems through a variety of controls.
3. Controls View: The player's control panel fills the screen, and they can view sensor displays, maps, flight displays, access the OS, and more.
4. Probe View: The player can see the probe from a third-person perspective. This view is used for manual flight and for observing their surroundings. This will only be availabe when the probe is close enough to communicate with the ship from a navigation console.

### Graphics
The visual goal is "used future" and "retro-futuristic" with a focus on simple, yet effective, visuals.

* Asset Pipeline: Low-poly models that can be procedurally generated and modified. 
* Natural Phenomena: Lighting, atmospheric scattering, and other natural phenomena are simulated using a variety of techniques, including ray marching, voxel rendering, and more.

## The Universe
### Star Systems
Star systems are procedurally generated and can contain a variety of celestial bodies, including stars, planets, moons, asteroids, and more. Systems can include all sorts of phenomena, such as nebulae, black holes, pulsars, and more.

### Spacecraft
Spacecraft are procedurally generated and can vary wildly in size, shape, and function. They can have different propulsion systems, scanner systems, probe systems, and more.

### Celestial Bodies
Celestial bodies are procedurally generated and can vary wildly in size, shape, and function. They can have different propulsion systems, scanner systems, probe systems, and more.

### Phenomena
Phenomena are procedurally generated and can vary wildly in size, shape, and function. They can have different propulsion systems, scanner systems, probe systems, and more.

### Aliens
In effect, the aliens are the other players. In addition to other players, there are also procedurally generated aliens that can be encountered in the universe. It is incredibly rare to encounter them in real time, but it is possible to find evidence of their existence through probes and other means.

## Mission Statement
The mission of $\exists$ is to bridge the gap between hard engineering and cosmic wonder. We aim to:

* Educate through Stewardship: To teach the mechanics of the universe—orbital dynamics, signal propagation, and resource scarcity—not through lectures, but through the necessity of survival.
* Empower the Problem-Solver: To provide a sanctuary for analytical minds to solve small, tangible problems that contribute to a massive, intangible whole.
* Cultivate Cosmic Humility: To offer a space for quiet contemplation of the beauty and vastness of the void, emphasizing that while we are all together in the same mathematical system, we are profoundly, beautifully alone.