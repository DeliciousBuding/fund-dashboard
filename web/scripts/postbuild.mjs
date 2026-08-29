// Vite emptyOutDir wipes the build output dir, including the .gitkeep that
// keeps `go:embed all:dist` compilable from a clean checkout. Restore it.
import { writeFileSync } from "node:fs";

writeFileSync(new URL("../../internal/webui/dist/.gitkeep", import.meta.url), "");
