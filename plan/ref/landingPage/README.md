# Free HTML veterinary website template with scroll animations and no libraries

Veterinary website template: a free one-page site for a vet clinic or animal hospital, with a four-step visit guide, services, vets, recent patients, a price list and a visit request form. The look is cream with coral arches and flat shapes. Its scroll motion runs on CSS with no JavaScript libraries, in one file of about 30 KB gzipped with an AGENTS.md, so Claude Code, Cursor or Codex can adapt it to your practice.

[![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE) ![one HTML file: no build](https://img.shields.io/badge/one%20HTML%20file-no%20build-orange) ![dependencies: none](https://img.shields.io/badge/dependencies-none-brightgreen)

**[Live demo](https://8bsite.github.io/veterinary-website-template/)** · **[Use this template](https://github.com/new?template_name=veterinary-website-template&template_owner=8bsite)** · **[Download ZIP](https://github.com/8bsite/veterinary-website-template/archive/refs/heads/main.zip)** · **[Template page on 8b.com](https://8b.com/templates/burrow/)**

<a href="https://8bsite.github.io/veterinary-website-template/"><img src="previews/desktop.png" alt="Veterinary website template: first screen on a desktop" width="76.1%"></a> <a href="https://8bsite.github.io/veterinary-website-template/"><img src="previews/phone.png" alt="Veterinary website template: first screen on a phone" width="21.9%"></a>

<p align="center"><a href="https://8bsite.github.io/veterinary-website-template/"><img src="previews/scroll.gif" alt="Veterinary website template: scrolling through the first sections" width="40%"></a><br><sub>Scroll animation</sub></p>

## Use it with Claude Code, Cursor or Codex

Download the four files into your project folder (`curl.exe` in Windows PowerShell):

```bash
curl -LO https://raw.githubusercontent.com/8bsite/veterinary-website-template/main/index.html
curl -LO https://raw.githubusercontent.com/8bsite/veterinary-website-template/main/AGENTS.md
curl -LO https://raw.githubusercontent.com/8bsite/veterinary-website-template/main/LICENSE
curl -LO https://raw.githubusercontent.com/8bsite/veterinary-website-template/main/CREDITS.md
```

Then fill in the angle brackets and give your agent this prompt:

```text
Read AGENTS.md, then edit index.html. Turn it into the site for <my practice: clinic name, town and address, the animals we see, vets and nurses with roles, services with prices, opening hours, night and Sunday emergency cover, insurance and payment terms, how clients book>. Motion is CSS scroll-driven animation plus the inline motion kit: keep the gate script, the section ids, the data-8b attributes, the classes AGENTS.md lists and the opening in one section; add no animation library and no fifth small shape. Ask me for every missing fact. Never invent prices, hours, addresses, licences, reviews, vet names or contacts.
```

If you vibe code this veterinary website template for Claude Code, Cursor or Codex, tell the agent to begin with facts only the clinic has: name, town and address, each vet and role, the price of every service, hours, night and Sunday cover, insurance and payment terms. It should ask for each one and never make up a price, a licence or a review. The demo contradicts itself on staff, with four vets in the species text and three in the team intro, so make them agree. Leave the section ids, the data-8b attributes and the classes AGENTS.md lists alone. The opening's CSS gives a fling path to four named shapes besides the arch, so do not add a fifth.

No agent? Click **[Use this template](https://github.com/new?template_name=veterinary-website-template&template_owner=8bsite)**, edit `index.html` on GitHub and turn on **Settings > Pages**.

## Sections and motion

- **Navigation**: top bar: arch logo and wordmark, section links, Book a visit button; phone menu overlay.
- **Calm, thorough, kind. Vet care for every animal.**: first screen: headline, two buttons, wide photo and floating shapes; second screen: four numbered steps of a visit on a coral arch. Smooth wheel scrolling throughout; seven section headings rise line by line. On scroll, the small coral arch near the headline grows into the doorway behind the visit steps as the other shapes fly outward and the photo shrinks into a rounded frame.
- **Two doors. Cats never wait with dogs.**: two arch-cut photos either side of the copy (demo: separate cat and dog sides of the clinic). Tied to scroll, the cat and dog photos slide out from the centre to either side as their narrow arch masks widen.
- **Small, scaly, feathered or furry.**: four overlapping colour panels, one per group (demo: animals the clinic treats). The four colour panels start stacked and tilted in the middle and are dealt out into an overlapping row as you scroll; small shapes drift past.
- **What we do here.**: list of six services, each row with a photo, text and a short note. Tied to scroll, each row's photo opens from a small pill to a full frame near the middle of the screen and folds back as the row leaves, while its rule grows and takes the row's colour.
- **Who you’ll meet in the room.**: four people with portraits, role and one line (demo: vets and head nurse). As the team scrolls into view, each portrait opens from a circle, arch, square or diamond into its full frame.
- **This month at Burrow.**: pinned horizontal row of seven cards (demo: patients of the month, name, breed, age, why they came). The section holds in place while seven patient cards slide sideways; cards away from the centre drop and tilt, so the row curves like a wheel.
- **Prices you see before we start.**: dark band: price list (service, what is included, price) and a note on estimates.
- **On Lombard Street since 2014.**: sticky building photo beside three dated paragraphs (demo: the clinic's history). The sticky building photo starts close on the ground floor, moves to the upper floor and pulls back to the whole corner as you read.
- **Before you come in.**: questions and answers (native &lt;details&gt;).
- **Come in.**: coral arch panel: address, phone, email, hours, photo and a visit request form. As the section scrolls in, the coral panel grows and rises while its top corners round into a tall arch.
- **Footer**: rounded panel: brand line, address, three link columns, copyright and the Design: 8B credit.

## Design

![#f7efe2](https://img.shields.io/badge/-%23f7efe2-f7efe2?style=flat-square) cream page background  
![#1f0b08](https://img.shields.io/badge/-%231f0b08-1f0b08?style=flat-square) dark brown text and prices band  
![#ebe2d3](https://img.shields.io/badge/-%23ebe2d3-ebe2d3?style=flat-square) darker cream panels and cards  
![#6a5c53](https://img.shields.io/badge/-%236a5c53-6a5c53?style=flat-square) grey-brown secondary text such as roles, captions and footer lines  
![#f26b45](https://img.shields.io/badge/-%23f26b45-f26b45?style=flat-square) coral buttons, arches and contact panel  
![#63a4bc](https://img.shields.io/badge/-%2363a4bc-63a4bc?style=flat-square) blue dogs panel and shapes  
![#dcb155](https://img.shields.io/badge/-%23dcb155-dcb155?style=flat-square) mustard round cats panel  
![#96503f](https://img.shields.io/badge/-%2396503f-96503f?style=flat-square) rust small mammals panel  
![#3dac91](https://img.shields.io/badge/-%233dac91-3dac91?style=flat-square) green birds and reptiles panel  
![#e2a8d3](https://img.shields.io/badge/-%23e2a8d3-e2a8d3?style=flat-square) pink accent shapes  
![#8e8565](https://img.shields.io/badge/-%238e8565-8e8565?style=flat-square) khaki circle accents beside the first headline and the species panels

**Google Sans**: all text: large medium-weight headings with tight letter spacing, regular body copy (Google Fonts). A cream page with centred headings in several sections and flat shapes (arch, circle, square, quarter-circle, diamond, plus) used as bullets and step markers. Photos sit in cut frames: arch-topped doors, circles, one huge rounded corner. Coral arches frame the visit steps and the contact panel, and most buttons have rounded tops.

## Questions

**Where do visit requests go when someone submits the form?**  
Nowhere, as shipped. The form asks for a name and phone, both required, plus the animal's name, a species and what is going on. The script checks the required fields, shows a thank-you line and clears the form, but nothing leaves the page. Connect Formspree or Netlify Forms, using the steps in AGENTS.md, or remove the form and keep the email link.

**What do great vet websites have in common?**  
Clear services, visible prices, named vets, the species treated, opening hours and an easy way to ask for a visit. This template has a section for services, prices, vets and species, puts the hours beside the request form, and adds the clinic's story and a six-question FAQ.

**Can I use it for an exotic-pet practice, or a clinic that sees only cats and dogs?**  
Yes, both. The species section has four colour panels, and the form's species menu has five options: rewrite them to match the animals you treat. The section on separate cat and dog sides describes two entrances in the demo, so if your building has one, rewrite its copy and photos or delete the section as AGENTS.md describes.

## Is it the right base?

This vet template suits a clinic with a team, services and published prices that wants a warm, playful page. This is the light build: the same look and scroll behaviour, with no animation library and no script from a CDN. Measured here, it downloads about 30 KB of code gzipped in one file, where the GSAP version needs about 77 KB and four extra requests. For overnight stays, the [cat boarding](https://8b.com/templates/sill/) template starts from rooms and nightly prices. The weak spot is booking: there is no appointment calendar or online payment, only a request form that sends nothing until you connect a service.

## Before you publish

- Replace what is marked `data-8b-placeholder` (social preview image, favicon, phone, email, map, form, social) and the invented demo facts; [AGENTS.md](AGENTS.md) lists them.
- The form sends nothing yet: connect it to Formspree or Netlify Forms, or remove it.

## Tech

No JavaScript libraries: CSS scroll-driven animations and a small inline motion script with its own smooth wheel scrolling, 120 KB, no build step. Chrome and Edge 108+, Safari 15.4+, Firefox 101+ (it uses cascade layers, small/dynamic viewport units). Without scroll-driven animations (Firefox today) the scroll motion and the smooth wheel are off and each section shows its finished layout, while any load-in or looping animation still plays; with "reduce motion" on, the page is complete and nothing moves on its own (hover effects stay).

A GSAP build of the same page is [`index.gsap.html`](index.gsap.html): GSAP 3.13.0, ScrollTrigger, SplitText, Lenis 1.3.27 from jsDelivr, 106 KB; its scroll motion also plays in Firefox. Use one of the two files.

## License

MIT, see [LICENSE](LICENSE). Photos, fonts and libraries keep their own licences: [CREDITS.md](CREDITS.md). More free templates: [8b.com/templates](https://8b.com/templates/) · [github.com/8bsite](https://github.com/8bsite).
