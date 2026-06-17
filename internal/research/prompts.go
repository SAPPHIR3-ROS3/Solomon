package research

const researchPlanPrompt = `You are a research strategist. Before searching, analyze this question and create a research plan.

**Question:** %s

Break this question down:
1. What are the key sub-topics that need to be covered for a comprehensive answer?
2. What specific data points, facts, or perspectives should we look for?
3. What would a complete, high-quality answer include?

Return a JSON object with:
- "sub_questions": Array of 3-6 specific sub-questions to investigate
- "key_topics": Array of key topics/angles to cover
- "success_criteria": One sentence describing what a complete answer looks like`

const queryGenPrompt = `You are a research assistant planning web searches.

**Original question:** %s

**Research plan:**
%s

**What we know so far:**
%s

**Round:** %d

Generate %d focused search queries that will help answer the question.
%s

Return ONLY a JSON array of query strings, nothing else.`

const synthesizePrompt = `You are updating an evolving research report.

**Original question:** %s

**Current report:**
%s

**New findings from this round:**
%s

Integrate the new findings into the existing report. Produce an updated, well-organized report that answers the original question as completely as possible given all evidence so far. Remove redundancy, resolve contradictions, and maintain logical flow. Keep source URLs as inline citations where relevant.

Write only the updated report — no preamble or meta-commentary.`

const stopPrompt = `You are deciding whether a research report is comprehensive enough.

**Original question:** %s

**Current report:**
%s

**Rounds completed:** %d

Based on the report so far, do we have enough information to answer the question comprehensively? Consider:
- Are the key aspects of the question addressed?
- Are there obvious gaps or unanswered sub-questions?
- Is the evidence sufficient and from multiple sources?

Reply with ONLY "YES" or "NO" followed by a brief one-sentence reason.`

const finalReportPrompt = `Write a **long, detailed, comprehensive** research report answering this question:

**Question:** %s

**All collected evidence and analysis:**
%s

Requirements:
- Write at MINIMUM 1500 words — this should be a thorough, magazine-quality article
- Use clear ## headings and ### subheadings to organize into logical sections
- Each section should have multiple detailed paragraphs, not just bullet points
- Synthesize and analyze the information — explain WHY things matter, draw comparisons, provide context
- Include specific data points, numbers, and statistics from the evidence
- Include source URLs as inline citations [like this](url)
- Note where sources agree and where they disagree
- Add a brief executive summary at the top
- End with a clear conclusion that directly answers the question
- Write in an engaging, informative style — not dry or robotic`

const tldrPrompt = `Write a concise executive TL;DR synthesis for this research report.

**Original question:** %s

**Full report:**
%s

Requirements:
- 150-300 words
- Directly answer the question in plain language
- Highlight the 3-5 most important takeaways
- Mention key caveats or disagreements among sources if relevant
- Do not use headings — write as flowing paragraphs
- This will appear under a "## TL;DR" section at the end of the report`

const extractorPrompt = `You are extracting research evidence from a webpage.

**Research goal:** %s

**Webpage content:**
%s

Return a JSON object with:
- "summary": 2-4 sentence summary of information relevant to the goal (or empty if irrelevant)
- "evidence": Key facts, quotes, or data points (max 2000 chars)
- "rational": Brief note on why this source matters

If the page has no useful content for the goal, set summary to "insufficient to answer" and keep evidence empty.`

const classifyCategoryPrompt = `Classify this research question into exactly ONE category.
Categories: product, comparison, howto, factcheck
If none fit well, respond with: general

Question: %s

Respond with ONLY the category name, nothing else.`

var categoryOverrides = map[string]string{
	"product": `IMPORTANT FORMAT OVERRIDE — this is a PRODUCT research report:
- Structure as a RANKED LIST of products/options (best first)
- For EACH product include: name as ### heading, approximate price, 2-3 sentence summary, **Pros:** bullet list, **Cons:** bullet list
- Start with a quick-compare markdown table of top picks
- End with a ## Verdict section picking Best Overall and Best Value`,
	"comparison": `IMPORTANT FORMAT OVERRIDE — this is a COMPARISON report:
- Create a ## Comparison Table as a markdown table comparing ALL options across key criteria
- Write a ## section per option with strengths, weaknesses, and ideal use case
- End with ## Best For verdicts`,
	"howto": `IMPORTANT FORMAT OVERRIDE — this is a HOW-TO guide:
- Start with ## Quick Guide — concise numbered list
- Then ## Prerequisites
- Then detailed ## Step N sections
- End with ## Common Mistakes`,
	"factcheck": `IMPORTANT FORMAT OVERRIDE — this is a FACT-CHECK report:
- Start with ## The Claim
- Create ## Evidence For and ## Evidence Against sections
- Include a ## Verdict section: **Supported**, **Mixed Evidence**, or **Unsupported**`,
}
