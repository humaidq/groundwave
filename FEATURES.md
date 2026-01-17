## Overview

Groundwave is a single‑user, self‑hosted personal operating system built for one person: me. It’s not a SaaS platform, not a multi‑tenant tool, and not a generalized product. It exists to unify the systems I actually live in — relationships, knowledge, tasks, inventory, health, and ham radio — into one coherent ecosystem that reflects how I think and operate day to day. Every feature is purpose‑built for this niche and required for my workflow, with no compromises for broader audiences. The application is intentionally “vibe‑coded,” favoring clarity, continuity, and personal memory over generic enterprise patterns. This document is written in maximal detail so humans, LLMs, and agents can fully understand the intent, scope, and interconnected nature of the system.

## Contacts

Contacts are the heart of Groundwave, giving you a rich, structured profile for every person or organization you care about. You can capture names, roles, organizations, call signs, and tiers, then expand each profile with emails, phones, addresses, and links so everything you need sits in one place. Profiles also support dedicated URLs and identity links (GitHub, LinkedIn, ORCID, and more), making each contact a complete hub rather than a scattered set of handles.

You can build your contact list manually or import from CardDAV and link profiles directly to your address book. Linked contacts stay in sync, updating names, organizations, emails, and phone numbers while keeping your local edits intact. CardDAV notes are also surfaced separately alongside your local notes, so external context is always visible without losing your own history.

Contacts aren’t just static records — they’re living timelines. The Activity Feed blends notes and contact logs into a single view, so every interaction stays connected. Notes are quick, timestamped snapshots, while logs capture meaningful moments like calls, meetings, emails, messages, and more. For deeper context, every contact has a dedicated chat history that tracks platform, sender, and time, giving you a clear narrative of your ongoing conversations.

When you’re logging updates across multiple people, Groundwave supports bulk contact logging so you can record a single interaction against a group without repetitive edits. That makes team meetings, group check‑ins, and shared events easy to capture once and track everywhere they belong.

Service Contacts keep utilities, vendors, and organizations separate from personal relationships, while tags let you group, filter, and rediscover people fast. Whether you’re managing a handful of key relationships or a large network, Contacts keep everything organized, searchable, and ready when you need it.

## Zettelkasten

Groundwave’s Zettelkasten turns your Org-roam knowledge base into a living memory layer for your CRM. You write and connect notes on your laptop in Org-roam, then read them beautifully formatted inside the app or on the web, so your thinking stays fluid and accessible wherever you are.

Links are first-class citizens. Backlinks and forward links are imported and surfaced directly on each note, so you can see what inspired an idea and where it leads. In the Zettelkasten Chat, you can pull in backlinks and forward links to widen the context of a question, letting the conversation follow your existing trails of thought instead of starting from scratch.

Linking stays fresh through an explicit refresh action and a background link‑cache updater, so the web view always reflects the current state of your Org‑roam graph. Recent navigation history also stays visible, helping you retrace your steps when you’re deep in a chain of ideas.

Each note can also carry lightweight comments, with an inbox view that keeps new notes and reflections easy to triage and revisit later. It’s a calm, connected system that rewards linking, revisiting, and deepening your knowledge over time.

## TODOs

The TODO page is a simple, dependable mirror of your Org-mode task list. You author tasks in Org mode on your laptop, then Groundwave renders them cleanly with their original TODO states intact (e.g., TODO, NEXT, DONE), so you can scan progress at a glance. It treats your Org file as the source of truth, preserving the structure and formatting you already use instead of forcing a new task system.

It’s intentionally lightweight: a single, readable task view that stays consistent with the file you already maintain, making it easy to keep your commitments visible without changing your workflow.

## QSL Log

The QSL Log is where your ham radio confirmations come together in one clean timeline. Import your ADIF logbook and Groundwave merges it with what you already have, keeping your data current while skipping any malformed records that shouldn’t pollute the log.

The QSL list is built for quick scanning, with search across call sign, mode, band, and country so you can jump straight to the contact you need. Each QSO opens into a dedicated confirmation view with a crisp summary table, station details, and a full history of all QSOs with that same station, so you can trace the relationship over time.

Confirmations are tracked across paper and digital paths. Paper QSLs show sent/received status with a quick request action when you need to ask for a card, while LoTW and eQSL confirmations surface clearly as sent/received indicators. When grid squares are available, a map renders the path between stations, adding a visual layer to each contact. QSOs also link back to their associated contact record, so your radio log and CRM stay connected.

## Inventory

Inventory gives you a clean, personal catalog for physical items you want to track. Each item receives an auto-generated ID, then you can capture its name, location, description, and current status so everything stays organized and easy to browse. Statuses are explicit and practical — active, stored, damaged, given, disposed, or lost — so you can always tell where something stands.

The list view keeps things fast with status filtering and quick search, while each item page surfaces the full detail in one place. Location fields include autocomplete based on your existing entries, so storage patterns stay consistent without extra effort. You can attach generic files from WebDAV (manuals, receipts, photos, or anything else) and keep a running comment thread for notes, updates, or maintenance history. It’s a lightweight system that makes it effortless to track what you own and where it lives.

## Health

Health turns Groundwave into a long‑term wellness journal for you or your family. Create health profiles with date of birth, gender, and baseline notes so each person’s labs are interpreted with the right context over time. The health dashboard tracks how many follow‑ups each person has and surfaces the most recent visit, making it easy to see who’s been monitored and what’s current.

Each profile is organized around follow‑ups — a visit or report with its own notes and a full lab panel. Results are grouped by category with a predefined test library, use standard units, and highlight not just reference ranges but optimal ranges too, so you can aim for longevity rather than settling for “normal.” Values outside reference ranges are clearly flagged, while values outside optimal range are still surfaced for gentle course correction. Calculated results are labeled as such, making it clear when a value is derived rather than manually entered.

Groundwave automatically calculates key derived metrics like absolute blood counts, TG/HDL ratio, and atherogenic coefficient, so deeper signals emerge without extra work. Trend charts track each lab test across time and overlay reference and optimal bands, making it easy to spot improvements or drift at a glance.

For a fast read, you can generate a local AI summary powered by Ollama. It streams a concise interpretation of the current follow‑up using your profile context and lab ranges, and it explicitly incorporates your baseline notes so the summary reflects your personal normal. The result is a private, on‑device explanation that highlights what changed and what matters, without sending your health data anywhere else.
