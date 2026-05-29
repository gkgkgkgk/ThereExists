package factory_test

// Blank import: registers the production mixture/resource catalog
// before the factory package's external tests run. Lives in an
// _test.go file so the factory package itself stays content-agnostic.
//
// Note: the import is in package factory_test (external) rather than
// factory because content imports factory — an internal blank import
// would close that loop. The factory test binary still runs the
// init() of the imported content package, so the registries are
// populated by the time any test runs.
import _ "github.com/gkgkgkgk/ThereExists/server/internal/factory/content"
