package assembly

// Blank import: registers the production archetype + mixture catalog so
// the assembly tests have something to dispatch through. Lives in an
// _test.go file so only the test binary picks it up — assembly itself
// remains content-agnostic and can be linked into binaries that wire
// their own registry.
import _ "github.com/gkgkgkgk/ThereExists/server/internal/factory/content"
