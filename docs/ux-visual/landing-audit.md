# Landing page audit (`apps/portal/src/routes/+page.svelte`)

Measured on staging at 1440px and 390px, 2025-02. This file records what was
changed and what is still open, so work can continue without re-measuring.

## Changed

| Item | Before | After |
| --- | --- | --- |
| Header | floating cream pill (`--ah-surface`, radius 999px, soft shadow) | flat bar, `--ah-cream` + 1px `--ah-line` bottom border, no shadow, 69px tall |
| Header height (mobile) | 88px | 88px (mobile menu keeps its 44px target) |
| Footer | dark `--ah-ink` block, 2 link groups, credit link to a template site | shared `<SiteFooter>` (cream, hairline, 5 links) |
| Page height | 5650px | 5301px |
| Workflow H2 | 52px (`3.6vw`) | 66px (`4.6vw`) so it matches every other section H2 |
| Closing decoration | 12rem yellow circle (`--ah-yellow`) behind the CTA | removed |

Section order and copy are unchanged.

## Still open

1. **Two saturated full-bleed bands** remain: `workflow` on `--ah-ink` and
   `closing` on `--ah-coral`. The login and jadwal pages are cream/surface only,
   so these read as the oldest part of the page.
2. **Heading jump**: hero H1 is 82px, every section H2 is 66px. A 56px H2 would
   give a calmer step, matching `--ah-type-h2` (`clamp(2.1rem, 4.6vw, 4.5rem)`).
3. **Section padding is uneven**: hero `129.6/72`, services/info/faq `86.4`,
   workflow `72`, closing `129.6/80`. `--ah-space-section`
   (`clamp(3rem, 6vw, 7rem)`) is not used here at all.
4. **SSL/hero offset**: hero starts at page top and relies on
   `padding-top: clamp(6.5rem, 9vw, 8.5rem)` to clear the fixed header. If the
   header height changes again, this must be updated with it.
5. Mobile header (88px) is taller than desktop (69px) because the mobile menu
   summary keeps a 44px target; hero padding covers it, but the difference is
   worth flattening.

## Verified after the change

```
header bg        rgb(253, 251, 247)   border-bottom 1px   shadow none
nav shell        radius 0 · border 0 · bg transparent
footer           .site-footer, 6 links (brand + 5)
tap targets      0 below 40px · text under 12px: 0
horizontal scroll none at 390 / 1440
```
