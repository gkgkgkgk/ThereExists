# Design Doc Design Doc

## Purpose
This document is a meta-document that describes the design document writing process for the ThereExists project.

## Design Doc Types
**Game Design Documents (starts with `TE_`)** - These documents describe the game design of the project. They are intended to capture the philosophy, vision, and high-level design of the game. 
** Phase Design Documents (starts with `Phase` and ends with `_Plan.md`)** - These documents describe a phase. This is essentially a technical overview of a phase of development. These live in folders named after the phase.
** Implentation Design Documents (starts with `Phase` and ends with `_Implementation.md`)** - These documents describe the implementation details of a phase. These live in the same folders as the phase design documents.
** Summary Design Documents (starts with `Phase` and ends with `_Summary.md`)** - These documents describe a phase. This will include a short writeup of what was accomplished, discussed, future work, and any other relevant information about the phase. It shouldn't be too long, and should be a good overview of the phase for someone who wants to understand what happened during the phase so they know what to do next.

### Phase Design Document Creation Process
This is usually the result of a long conversation discussing the next phase of development. This will be somewhat informal and technical, and will usually include (but not limited to):
- Overarching goals of the phase
- Potential technical approaches to the problems
- Thoughts, concerns, and ideas that came up during the conversation
- Future work

### Implementation Design Document Creation Process
This follows the creation of a Phase Design Document. It will outline the implementation details of the phase, and will be used to guide the development of the phase. It will contain a list of commits, and a detailed description of what each commit does as well as any other relevant information about the implementation of the phase. The implementation design document should be so good that it can be used to generate the entire phase by itself, by a human or by an AI that has never seen the project before. Don't write the code in the doc - (unless it's a one-liner or something) just solve the problems and describe the solution in the doc.

### Log Design Documents
These are created as the phase is being implemented. After each commit, update this document to reflect. This is so that if you need to pick up where you left off, you can do so easily. It should list which step was completed, when it was completed, and any other relevant information about the implementation of the phase. It shouldn't be too long, and should be a good overview of the phase so far so anyone can pick up where you left off and continue implementing the phase.