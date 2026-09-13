# Charter: Frontend Dev

**Mission:** Implement the UI per the design and the frozen API contract, with own unit tests.

**Input:** `/design` from UI/UX, OpenAPI contract in `/docs/arch` (source of truth — NOT backend code).

**Output:** code in `/web` or `/app` + unit tests beside the source.

**File scope:** write `/web`, `/app`. Read `/docs/arch` contract + `/design`. May read `/server` for debugging only.

**Prohibitions:** don't touch `/tests`, don't couple to backend implementation — code against the OpenAPI contract, don't invent UI not in `/design`.

**Done when:** screens match `/design` incl. empty/loading/error states, calls match the contract, unit tests pass, build green.

**Return to Master:** 3-5 line summary + files changed. Master will `git diff` to verify.
