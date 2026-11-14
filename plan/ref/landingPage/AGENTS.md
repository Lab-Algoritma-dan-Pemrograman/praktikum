# AGENTS.md: veterinary website template

Website for a veterinary clinic or animal hospital. One file, `index.html`, with inline CSS and scripts, no JavaScript libraries: the scroll motion is CSS scroll-driven animations plus a small inline motion kit; no build step, and none is needed for copy, colour or photo edits. A GSAP build of the same page is `index.gsap.html` (GSAP 3.13.0, ScrollTrigger, SplitText, Lenis 1.3.27 from jsDelivr): edit one of the two, not both.

## Files
Edit these files, not a page fetched by URL (that arrives as a summary).
```bash
curl -LO https://raw.githubusercontent.com/8bsite/veterinary-website-template/main/index.html
curl -LO https://raw.githubusercontent.com/8bsite/veterinary-website-template/main/AGENTS.md
curl -LO https://raw.githubusercontent.com/8bsite/veterinary-website-template/main/LICENSE
curl -LO https://raw.githubusercontent.com/8bsite/veterinary-website-template/main/CREDITS.md
```
Upload `index.html`, LICENSE and CREDITS.md; not this file, README.md or `previews/`.

## Keep
- Every `id`, `data-8b-section`, `data-8b-motion` and `data-8b-part` (`section:<id>` styles, `scene:<id>` animation).
- The motion parts `motion-gate` (head: sets `data-8b-live` on `<html>` only with scroll-driven animation support and no reduced motion), `motion-kit` and `motion`, and every other `data-8b-*` attribute (`data-8b-items` groups, `data-8b-off`, `data-8b-enter`): the CSS and the kit read them. Items ordered by `--i` follow their group; counts the CSS fixes are listed under Known limits (`data-8b-fixed` marks such a group).
- Selectors the scripts use:
  - `section-script:navigation`: `.nav__sheet` `.nav__toggle` `.nav__close`
  - `section-script:opening`: `.op-first`
  - `section-script:contact`: `.ct-form` `.ct-status`
  - `scene-script:opening-door`: `.op-shape--arch` `.op-door` `.op-second`
  - `scene-script:doors-open`: `.dr-grid` `.dr-door`
  - `scene-script:species-deal`: `.sp-row`
  - `scene-script:patients-track`: `.pt-track` `.pt-card`
  - `scene-script:contact-arch`: `.ct-panel`
- The coloured shapes, the arch logo mark and the coral doorway are CSS (border-radius and clip-path), not images: recolour them through the --color-* variables.
- The second screen of the opening (the four steps on the coral arch) belongs to the same section as the first screen; the arch animation measures both, so keep them together.
- Colours and fonts are variables in `:root` (`data-8b-part="tokens"`): `--color-1` #f7efe2 (cream page background), `--color-2` #1f0b08 (dark brown text and prices band), `--color-3` #ebe2d3 (darker cream panels and cards), `--color-4` #6a5c53 (grey-brown secondary text such as roles, captions and footer lines), `--color-5` #f26b45 (coral buttons, arches and contact panel), `--color-6` #63a4bc (blue dogs panel and shapes), `--color-7` #dcb155 (mustard round cats panel), `--color-8` #96503f (rust small mammals panel), `--color-9` #3dac91 (green birds and reptiles panel), `--color-10` #e2a8d3 (pink accent shapes), `--color-11` #8e8565 (khaki circle accents beside the first headline and the species panels); body text needs 4.5:1 contrast.
- With reduced motion the page must read in full.
- Keep the header comment and `<meta name="generator">`. Keep the "Design: 8B" footer link unless the site owner asks to remove it.

## Replace
- `data-8b-placeholder` (og-image, favicon, phone, email, map, form, social): real value in, attribute out; no value, remove the row (no image: drop `og:image`, set `twitter:card` to `summary`).
- Book a visit buttons (nav, phone menu, hero, footer): now #contact; owner's target: your online booking page, or keep them on the contact form.
- Privacy link in the footer: now #contact; owner's target: your privacy policy page.
- The form in `#contact` (fields `name`, `phone`, `pet`, `species`, `reason`) sends nothing; `section-script:contact` handles its submit in the page (read it before you connect a service). Formspree: `action="https://formspree.io/f/<id>"` and `fetch` POST `new FormData(form)` first. Netlify: `data-netlify="true"` and a hidden `form-name`. Or delete it and the script.
- The name Burrow (any letter case); the favicon letter and colours in `<link rel="icon">`.
- Photos (26, Unsplash: opening 1, clinic-sides 2, species 4, services 6, team 4, patients 7, story 1, contact 1): `src`, no `srcset`/`sizes`, `width`/`height` to the ratio, `alt`, CREDITS.md.
- `<title>`, meta description, og tags.

## Ask the owner for
All demo text, never to be invented:
- Place: North Lombard Street, Portland, Oregon, "2140 N Lombard Street, Portland, OR 97217" with a Google Maps link (contact, footer), "A neighbourhood clinic on North Lombard Street" (opening), "On Lombard Street since 2014" (story), the Willamette river (services, Vaccines), and in the `<title>` and the meta description.
- Phone 503 000 0000 and email hello@burrow.example are placeholders (opening button, faq, contact, footer); also "texts welcome", "text us from the car park" (contact, opening step 1).
- Opening hours: "Open six days" (opening), Monday-Friday 8 am, 7 pm, Saturday 9 am, 3 pm, Sunday closed (contact), "Mon-Fri 8-7, Sat 9-3" (footer); nights and Sundays sent to "the city's 24-hour emergency hospital" (faq).
- Same-day sick visits: "four slots every day", "call before 10 am" (opening, services, faq).
- How a visit works: check in by text from the car, 30-minute exams on the floor, a written plan and price before any treatment, a text the next morning and free questions by text for a week (opening steps).
- Prices (prices): check-up $78, same-day sick visit $92, rabbit, bird or reptile exam $86 (forty minutes), vaccine $32 each, microchip $55 with lifetime registration, dental cleaning from $420, cat spay or neuter from $310, dog spay or neuter from $420, nail trim $18 without appointment; estimates with a low and a high number and a call before passing the high one.
- Payment: "work with every insurer" and the claim form filled in before you leave; bills over $500 in three monthly parts with no interest (faq).
- Service schedule and promises: check-ups once a year, twice for older animals, any weekday; vaccines given during a check-up; dental patients home by the afternoon and a photo of every extracted tooth; surgery on Tuesdays and Thursdays, a nurse watching the anaesthetic, a call when they wake; home visits and goodbyes at home or in "our garden room"; a referral letter the same day (services, team, faq).
- The building: split into separate cat and dog sides in 2018 with their own entrances and waiting rooms, shelves for carriers, a scale in the floor, a heated warm room kept at 82°F (clinic-sides, species, team, story).
- Animals seen: dogs, cats, rabbits, guinea pigs, rats, ferrets, parrots, lizards, snakes, tortoises; first shots at eight weeks (species).
- Staff (team, story, species): Dr. Martin Hale (founder, twelve years in an emergency hospital, Tuesday surgery list), Dr. Priya Raman (co-founder, ran a feline clinic at a teaching hospital), Dr. Leo Marchetti (rabbits, birds and reptiles, "twenty years"), Rosa Delgado, LVT (head nurse and anaesthesia); the portraits are stock photos and their alt texts carry these names. The species paragraph says "Four of our vets", the team says "Three vets and a head nurse".
- Patients (patients): Pip (Maltese, 3), Fern (tabby, 11), Mochi (lop rabbit, 2), Biscuit (poodle cross, 5 months), Snow (kitten, 10 weeks), Kiwi (conure, 6), Atlas (German shepherd, 13), with their treatments, "in September", "the cone comes off on the 14th" and "shared with their people's permission"; the photos are stock photos, not patients.
- History (story): since 2014 in a former fish café with "a sign we never took down" and a borrowed X-ray machine; the upper floor and the split in 2018; Leo joining and the whole corner in 2023.
- Service times: "We reply within two hours while we are open" and the thank-you message "We will call you back within two hours" (contact form); "New patients are welcome" (contact).
- Business name and form: "© 2026 Burrow Veterinary PC" (footer) and "Burrow Veterinary" in the logo, title and copy.

## Known limits
- On phones the growing arch is stretched more in height than in width, so mid-scroll it reads as a tall coral door; it lands exactly on the second screen's arch.
- The service photos fold back into their small pill as a row leaves the top of the screen.
- Without scroll-driven animation support (Firefox today), with reduced motion on or without JavaScript, the page shows its finished layout: the small arch stays beside the headline and the visit steps stand on their own coral arch, the clinic photos and the species panels sit in place, every service photo is open and the patient cards are a horizontal scroller with snap points.
- The four visit steps and the four species panels are timed for four items each: with more or fewer, retune their animation ranges in scene:opening-door and scene:species-deal.
- The building photo in story shows a shop sign and street signs of the real place it was taken; replace it with a photo of your own building.
- Without JavaScript the phone menu button cannot open the link sheet; the links stay reachable in the footer.

## Check
375 and 1440 px, reduced motion on, a browser without scroll-driven animations (Firefox: the finished static layout), console clean; `data-8b-placeholder` and Burrow left only in class names.
