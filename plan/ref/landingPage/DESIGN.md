---
version: alpha
name: "Veterinary template by 8B"
description: "A warm cream page with coral arches, flat colour shapes and doorway-cut photos for a vet clinic or animal hospital. Free HTML template by 8B, MIT."
colors:
  primary: "#1f0b08"
  secondary: "#6a5c53"
  tertiary: "#f26b45"
  neutral: "#f7efe2"
  surface: "#ebe2d3"
  on-surface: "#1f0b08"
  dog-blue: "#63a4bc"
  cat-mustard: "#dcb155"
  small-rust: "#96503f"
  exotic-green: "#3dac91"
  accent-pink: "#e2a8d3"
  accent-khaki: "#8e8565"
typography:
  headline-display:
    fontFamily: Google Sans
    fontSize: 84px
    fontWeight: 500
    lineHeight: 1.02
    letterSpacing: -0.035em
  headline-lg:
    fontFamily: Google Sans
    fontSize: 72px
    fontWeight: 500
    lineHeight: 1.02
    letterSpacing: -0.035em
  title-md:
    fontFamily: Google Sans
    fontSize: 26px
    fontWeight: 500
    lineHeight: 1.16
    letterSpacing: -0.02em
  body-md:
    fontFamily: Google Sans
    fontSize: 17px
    fontWeight: 400
    lineHeight: 1.55
    letterSpacing: 0em
  body-sm:
    fontFamily: Google Sans
    fontSize: 15px
    fontWeight: 400
    lineHeight: 1.45
    letterSpacing: 0em
  label-md:
    fontFamily: Google Sans
    fontSize: 14px
    fontWeight: 500
    lineHeight: 1
    letterSpacing: 0em
  button:
    fontFamily: Google Sans
    fontSize: 16px
    fontWeight: 500
    lineHeight: 1
    letterSpacing: -0.005em
  price:
    fontFamily: Google Sans
    fontSize: 22px
    fontWeight: 500
    lineHeight: 1
    letterSpacing: -0.02em
rounded:
  xs: 10px
  sm: 14px
  md: 26px
  lg: 32px
  xl: 220px
  full: 999px
spacing:
  section-padding: 168px
  gutter: 44px
  content-max: 1216px
  grid-gap: 48px
  rule-weight: 1.5px
  button-height: 52px
  field-height: 50px
components:
  button-primary:
    backgroundColor: "{colors.tertiary}"
    textColor: "{colors.primary}"
    typography: "{typography.button}"
    rounded: "{rounded.md}"
    padding: "0 26px"
    height: "{spacing.button-height}"
  button-primary-hover:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.neutral}"
  button-secondary:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.primary}"
    typography: "{typography.button}"
    rounded: "{rounded.md}"
    padding: "0 26px"
    height: "{spacing.button-height}"
  button-dark:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.neutral}"
    typography: "{typography.button}"
    rounded: "{rounded.md}"
    padding: "0 28px"
    height: "{spacing.button-height}"
  nav-pill:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.primary}"
    typography: "{typography.body-sm}"
    rounded: "{rounded.full}"
    padding: 6px
  badge-step:
    backgroundColor: "{colors.neutral}"
    textColor: "{colors.primary}"
    typography: "{typography.label-md}"
    rounded: "{rounded.full}"
    padding: "0 14px"
    height: 30px
  card-surface:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.on-surface}"
    rounded: "{rounded.lg}"
  input-field:
    backgroundColor: "{colors.neutral}"
    textColor: "{colors.primary}"
    typography: "{typography.body-md}"
    rounded: "{rounded.sm}"
    padding: "12px 16px"
    height: "{spacing.field-height}"
---

## Overview

Burrow is a one-page site for a neighbourhood vet clinic or animal hospital that wants to feel warm and playful rather than clinical. The one idea is the doorway: coral arches frame the visit steps and the contact panel, photos are cut into arch, circle and lopsided frames, and a few flat shapes (arch, circle, square, plus, diamond) act as bullets, markers and mascots. It is written for worried owners in a hurry, so the page stays friendly, rounded and bright on a cream ground. All clinic names, vets and prices in the demo are invented.

## Colors

A cream page, one dark brown, one hot coral, and a small set of animal colours.

- **Primary (#1f0b08):** a brown so dark it reads as black, for text, rules, dark buttons, focus outlines and the prices band.
- **Secondary (#6a5c53):** grey-brown for roles, captions and footer lines.
- **Tertiary (#f26b45):** coral for the main button, the doorway arch, the contact panel, the phone menu sheet, the FAQ plus sign and the highlight that rises behind hovered links.
- **Neutral (#f7efe2):** the cream page background and form panels.
- **Surface (#ebe2d3):** darker cream for cards, the nav link pill, the price ledger and the secondary button.
- **On-surface (#1f0b08):** text on cream.
- **Dog blue (#63a4bc), cat mustard (#dcb155), small-animal rust (#96503f), exotic green (#3dac91):** the four species panels, also reused for small bullet shapes and rule highlights.
- **Accent pink (#e2a8d3) and accent khaki (#8e8565):** extra shape colours only.

## Typography

One family, **Google Sans**, at weights 400 and 500. It is round, open and friendly, which is what this audience needs.

- Headlines are weight 500, tightly tracked (-0.035em), with a line height near 1 and balanced wrapping. The hero headline reaches 82px; section headings run from about 34px to 72px and are often centred.
- Card and row titles are weight 500 at 19px to 36px with -0.02em.
- Body is 17px at weight 400 with a 1.55 line height; captions are 15px in the secondary brown.
- Buttons are 16px weight 500. Prices are 22px weight 500 with small 14px units.
- Nothing is uppercase, and there are no tracked small caps.

## Layout

Content sits in a centred wrapper up to 1216px wide with a gutter of up to 44px, and each section has vertical padding of up to 168px. Many section headings are centred. Others are two-column: prices on the ink band split roughly 0.9 to 1.1, the story pairs text with a sticky photo, the FAQ puts its heading beside the list. The three-column clinic row has a centred text column between two photos. Four colour panels overlap in one row. The care list is rows of photo, text and note, the vets are four columns with every second one dropped, and patient cards run in a horizontal track.

At 900px and below the nav collapses to a Menu pill that opens a full-screen coral sheet with 40px links; the grids become one or two columns and the story photo sticks at the top. At 720px and below the species panels stack with a small overlap and the patient cards are 72vw wide.

## Elevation & Depth

There are no shadows and no filters. Depth comes from overlap and flat colour: species panels overlap by up to 44px and the round mustard panel sits on top, the coral doorway sits behind the four visit steps, and the near-black prices band interrupts the cream page. Sticky layers add movement: the nav floats above everything and hides on a downward scroll, and the story photo stays in view while the text scrolls.

## Shapes

Shape is the signature. Radii are 10px, 14px, 26px, 32px, 220px and 999px. Photo frames use mixed corners: arch tops (999px above, 14px below), circles, one huge 220px corner, and plain rounded frames. Buttons have asymmetric corners, 26px on top and 10px below, flipped on the secondary button. The contact panel is a coral door with a tall arched top. The flat shapes are CSS: an arch, a circle, a square, a quarter circle, a diamond and a plus. They carry species colours and replace icons.

## Components

- **button-primary:** coral, 52px, arched top. On hover a dark block rises from below and its curved top flattens while the label turns cream.
- **button-secondary / button-dark:** the darker-cream and the dark versions; the secondary hover matches the main button, and the dark one fills with cream instead.
- **nav-pill:** the logo pill and the link pill float on the page, with the coral Book a visit button beside them.
- **badge-step:** a small cream pill for Step 1 to Step 4 and for labels on photos.
- **card-surface:** a 32px rounded darker-cream card, used for the price ledger.
- **input-field:** 50px, cream fill, 1.5px dark border, 14px corners.
- **Rules:** 1.5px dark rules separate steps, rows, FAQ items and facts.

## Motion

The page loads no animation library and no script from a CDN. Scroll effects are CSS scroll-driven animations; a small inline motion kit at the end of the body measures the page, reveals elements as they arrive, splits headings into lines and smooths the mouse wheel. A gate script in the head turns all of it on only in browsers with scroll-driven animations and without a reduced-motion preference. The one motion idea is shapes that change: a small coral arch grows into a door, frames open from a circle, arch, square or diamond, and colour panels are dealt like cards.

- **Opening:** on load the intro rises, the photo opens from a rounded inset and pans slowly, and the shapes loop (a circle orbits, a plus turns, a diamond swings, a square tilts). On scroll the small arch moves and scales into the doorway behind the visit steps, the photo shrinks into a rounded shape, the other shapes fly outward and the step markers spin in.
- **Clinic:** the cat and dog photos slide out from the middle as their narrow arch masks widen.
- **Species:** the four panels start stacked and tilted in the middle and are dealt into an overlapping row.
- **Care and team:** each care photo opens from a small pill and closes again as the row leaves, with its rule growing and taking the row's colour; team portraits and the contact photo open from a circle, arch, square or diamond.
- **Patients:** the section holds in place while the cards slide sideways, and cards away from the centre drop and tilt so the row curves like a wheel.
- **Story:** the sticky photo starts close on the ground floor, moves to the upper floor and pulls back to the whole photo.
- **Contact:** the coral panel grows and rises as its top corners round into a tall arch.

Seven section headings reveal line by line from a mask (about 0.95 seconds, an ease-out curve), and small shapes drift and turn as you scroll past the species panels, the prices and the footer. The markup carries the motion: `data-8b-motion` (rise, reshape, drift), `data-8b-items`, `data-from` and `data-8b-enter`. In browsers without scroll-driven animations (Firefox today) and with reduced motion, the scroll effects, heading reveals and smooth wheel are off, every section shows its finished layout and the patient cards become a sideways scrolling row with snap points; Firefox still plays the opening's load-in and loops, and reduced motion stops those too. Hover effects stay in every case.

## Do's and Don'ts

- Do keep the doorway language: arches, circles and flat shapes, never plain rectangles with sharp corners.
- Do keep one family, Google Sans, at weights 400 and 500.
- Do use the species colours for shapes and panels, and coral for actions and the door.
- Do keep to four small shapes besides the arch near the headline; the opening CSS has a fling path for each.
- Do keep the gate script, the inline motion kit and the data-8b attributes; don't add an animation library.
- Do keep the cream ground and the near-black prices band.
- Don't add shadows, blurs, gradients or stock icons.
- Don't use uppercase labels or a second display face.
- Don't turn the shape bullets into photos or emoji.
