# Phase 4.1 Closeout
*2026-04-21*

Schema landed: `Mixture` owns the recipe (precursors, power, time, catalyst); `Refinery` modulates (efficiency, throughput, heat, catalyst wear) and gates via `SupportedMixtureIDs`. `MixtureProduction` deleted. `FactoryVersion = "phase4_1-v1"`.

Next: **content pass.** Author `Precursors` / `PowerCostPerKg` / `RefiningTimePerKg` / `RequiredCatalyst` / `IgnitionNeed` on the five existing mixtures, then author refinery archetypes. No schema changes expected — if the content pass wants to touch the schema, that's a new plan, not a drift of this one.
