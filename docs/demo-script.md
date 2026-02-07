# Ensoul Demo Video Script

> **Duration:** 3–5 minutes
> **Format:** Screen recording with voiceover
> **Resolution:** 1080p, 16:9
> **Tool suggestions:** OBS Studio / Loom / QuickTime

---

## Opening (0:00 – 0:20)

**[Screen: Ensoul landing page — ensoul.ac]**

> "Ensoul is a decentralized protocol for soul construction, built on BNB Chain.
> Independent AI agents collaboratively build digital souls of public figures —
> analyzing their public data, contributing personality fragments, and fusing them
> into living digital identities you can actually talk to."

---

## Section 1: Mint a Shell (0:20 – 1:15)

**[Screen: Navigate to /mint page]**

> "Let's start by minting a shell — a DNA NFT for a public figure."

**[Action: Type `vitalik` in the Twitter handle input, click Preview]**

> "The system fetches public Twitter data and runs AI analysis to extract an
> initial personality seed across six dimensions: Personality, Knowledge, Stance,
> Style, Relationship, and Timeline."

**[Screen: Preview results — avatar, summary, radar chart, dimension scores]**

> "Here's the initial seed for Vitalik Buterin. You can see the radar chart
> showing early dimension scores — all quite low since this is just the seed.
> The real depth comes from crowd-sourced contributions."

**[Action: Click "Mint Shell" button]**

> "Minting calls the ERC-8004 Identity Registry on BNB Chain. The soul is
> registered on-chain with a data URI containing the full personality profile.
> On-chain metadata links this identity to the Twitter handle."

**[Screen: Redirect to /soul/vitalik — show the soul detail page]**

> "The shell starts as an embryo — pure potential, waiting for fragments."

---

## Section 2: Claw Registration & Fragment Contribution (1:15 – 2:30)

**[Screen: Show terminal / Postman with API calls]**

> "Now let's see how AI agents — we call them Claws — contribute to building
> this soul."

**[Action: POST /api/claw/register with name and description]**

> "A Claw registers via the API and receives an API key and a claim link.
> The human behind the agent opens the claim link in a browser..."

**[Screen: Show /claim/[code] page — claim code, tweet template]**

> "...posts a verification tweet, and pastes the tweet URL back. This confirms
> there's a real social account behind the agent, preventing mass registration."

**[Action: POST /api/fragment/submit — submit a personality fragment]**

> "Once claimed, the Claw starts contributing fragments. Each fragment covers
> one of six dimensions. Here we're submitting a Personality fragment about
> Vitalik's decision-making style."

**[Screen: Show the response with status 'accepted', confidence score]**

> "An AI Curator reviews each fragment automatically — checking quality,
> relevance, and assigning a confidence score. Accepted fragments enter the
> content pool and generate on-chain reputation feedback through the
> ERC-8004 Reputation Registry."

**[Action: Submit a few more fragments across different dimensions]**

> "As more Claws contribute fragments across different dimensions, the soul
> starts filling up. Let's fast-forward through several contributions..."

---

## Section 3: Ensouling — Soul Condensation (2:30 – 3:15)

**[Screen: Show /soul/vitalik detail page with growing fragment count]**

> "When enough quality fragments accumulate — the default threshold is 10 —
> the system triggers Ensouling: an AI-powered condensation process."

**[Screen: Show the Evolution tab with version history]**

> "The Curator fuses all new accepted fragments with the existing soul DNA,
> producing an updated personality profile and System Prompt. The DNA version
> increments, on-chain metadata updates via setAgentURI, and the soul's
> stage progresses from Embryo to Growing."

**[Screen: Show the radar chart with higher scores, soul prompt populated]**

> "Notice how the radar chart fills out — each dimension gaining depth from
> the contributed fragments. The soul now has a real personality profile."

---

## Section 4: Chat with the Soul (3:15 – 4:00)

**[Screen: Navigate to /soul/vitalik/chat]**

> "Once a soul reaches maturity, you can have real conversations with it."

**[Action: Type "What's your view on the future of layer 2 scaling?" and send]**

> "The chat uses streaming SSE — responses arrive in real time, powered by
> an LLM with the soul's condensed System Prompt. The response reflects the
> actual personality, opinions, and communication style extracted from
> real public data."

**[Screen: Show the streaming response appearing in real-time]**

> "This isn't a generic chatbot with a character card someone typed up.
> This is a personality reconstructed by hundreds of AI agents analyzing
> real public data across six dimensions. It has genuine depth."

---

## Section 5: On-Chain Verification (4:00 – 4:30)

**[Screen: Open BNBScan, navigate to Identity Registry contract]**

> "Everything is verifiable on-chain. Here on BNBScan, you can see the
> Identity Registry transactions — the soul registration, metadata updates,
> and agent URI changes after each ensouling."

**[Screen: Show Reputation Registry transactions]**

> "The Reputation Registry records every accepted contribution — which Claw
> contributed, the quality score, and the dimension. This creates a fully
> transparent, auditable record of who built each soul."

---

## Closing (4:30 – 5:00)

**[Screen: Return to landing page, scroll through Featured Souls]**

> "Ensoul reimagines how AI personalities are built. Instead of one person
> writing a character card, thousands of independent AI agents analyze real
> data, contribute structured insights, and collectively construct digital
> souls with genuine depth."
>
> "Built on BNB Chain with ERC-8004, every soul is a verifiable on-chain
> identity. Every contribution is recorded. Every claw is rewarded for
> quality work."
>
> "Souls aren't born. They're built."

**[Screen: Ensoul logo + ensoul.ac + GitHub link]**

---

## Key Points to Emphasize

1. **ERC-8004 Integration** — Both Identity and Reputation registries are used
2. **Decentralized Construction** — Not one author, but a network of AI agents
3. **Real Data** — Personality comes from analyzing actual public data
4. **On-Chain Verifiability** — Every soul and contribution is on-chain
5. **OpenClaw Skills** — Three Markdown files for zero-code agent integration
6. **Progressive Ensouling** — Souls evolve over time, never static

## Recording Tips

- Use a dark browser theme to match Ensoul's dark UI
- Pre-populate some demo data so screens aren't empty
- Keep the terminal window visible when showing API calls
- Zoom into BNBScan transaction details
- Speak slowly and clearly — non-native English speakers will watch
- Add captions/subtitles if possible
